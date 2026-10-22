package main

// The Shell tab's other shell: Alpine Linux in a jail (Settings, "Shell
// tab runs"), with its own tools and apk. It lives in an ext2 image on
// the Kobo's storage, .adds/hellobro/alpine.img, since the storage is FAT
// (no owners, no symlinks) and the Kobo's own system has 27 MB free.
// We mount it once, at alpineRoot, and every session is a jail of its
// own on it: alpineinit.go. The first session makes the image and puts
// Alpine's minirootfs in, fetched over plain HTTP (the Kobo has no CA
// certificates) and checked against the hash pinned here; inside, apk
// has its own and goes over HTTPS.
//
// The image is unmounted when we leave (and launch.sh makes sure), so
// that Nickel can hand the storage to a computer over USB.
//
// The system bus, if Settings says so; sound is what it's for first: the
// Kobo has no sound card, only Bluetooth through its bluealsa (3.1),
// which a client reaches over the bus. The jail gets the bus, and ALSA
// a plugin that speaks to that bluealsa:
// Alpine's own bluez-alsa is 4.x, whose API the Kobo's lacks, so it's
// the 3.1.0 one of Alpine 3.15, checked against its pinned hash too.
// The bus is the price: root on it reaches BlueZ and dhcpcd (Bluetooth
// and Wi-Fi setup), which is why it's off by default.

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"hellobro/kobo"
)

const (
	alpineVersion  = "3.24.3"
	alpineSHA256   = "dc1ddfaf03ede4be79ad7475490f6b7859dbcbbbbb85150aa1ee013b4bc2f0c1"
	alpineMBMin    = 256
	alpineMBMax    = 4095 // FAT's biggest file is 4 GB less a byte
	alpineMBDef    = 1024
	alpineDone     = "etc/hellobro-alpine"
	bluealsaSHA256 = "b515919407a3c098a1d794f88a997c54888e62806ef32bb3bbcd52087ec9bbec"
)

var (
	alpineURL  = "http://dl-cdn.alpinelinux.org/alpine/v3.24/releases/armv7/alpine-minirootfs-" + alpineVersion + "-armv7.tar.gz"
	alpineImg  = kobo.AppFile("alpine.img")
	alpineRoot = kobo.EnvOr("HELLOBRO_ALPINE_ROOT", "/tmp/hellobro-alpine")
	alpineMu   sync.Mutex // one setup at a time

	bluealsaURL = "http://dl-cdn.alpinelinux.org/alpine/v3.15/community/armv7/bluez-alsa-3.1.0-r0.apk"
	// The plugins, as kept next to hellobro: the jail's first process
	// copies them in (alpineinit.go).
	bluealsaPlugins = map[string]string{
		"usr/lib/alsa-lib/libasound_module_pcm_bluealsa.so": kobo.AppFile("alpine-bluealsa-pcm.so"),
		"usr/lib/alsa-lib/libasound_module_ctl_bluealsa.so": kobo.AppFile("alpine-bluealsa-ctl.so"),
	}
	alpineBus atomic.Bool // the setting, for the share server's goroutines too
)

// startAlpine starts a session in the jail on a new PTY: a login shell,
// or command. Setting up (the first time) and mounting happen on the
// PTY meanwhile, so the session shows how it goes.
func startAlpine(cols, rows int, command string) (*localShell, error) {
	pty, tty, err := openPTY(cols, rows)
	if err != nil {
		return nil, err
	}
	s := &localShell{pty: pty, exit: make(chan struct{})}
	go func() {
		defer tty.Close() // the jail has its own copies
		if err := alpineReady(tty); err != nil {
			log.Printf("alpine: %v", err)
			fmt.Fprintf(tty, "alpine: %v\r\n", err)
			s.fail(err)
			return
		}
		cmd := exec.Command("/proc/self/exe", "--alpine-init", command)
		cmd.Stdin, cmd.Stdout, cmd.Stderr = tty, tty, tty
		cmd.Env = []string{"TERM=xterm-256color"}
		if alpineBus.Load() {
			if err := bluealsaReady(tty); err != nil { // the bus goes in all the same
				log.Printf("alpine: sound: %v", err)
				fmt.Fprintf(tty, "alpine: no sound: %v\r\n", err)
			}
			cmd.Env = append(cmd.Env, "HELLOBRO_ALPINE_BUS=1")
		}
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setpgid:    true, // close's hangup goes to the jail's first process, and with it the jail
			Cloneflags: syscall.CLONE_NEWNS | syscall.CLONE_NEWPID | syscall.CLONE_NEWUTS | syscall.CLONE_NEWIPC,
		}
		if err := s.start(cmd); err != nil {
			log.Printf("alpine: %v", err)
			s.fail(err)
			return
		}
		pid := cmd.Process.Pid
		log.Printf("alpine: a session (pid %d)", pid)
		jailStarted(pid) // its players' bus (mpris.go)
		<-s.exit
		jailEnded(pid)
	}()
	return s, nil
}

// alpineReady makes the image and puts Alpine in if that's not done yet,
// and mounts it. Its progress goes to out, the session's terminal, which
// is asked how big the image is to be.
func alpineReady(out io.ReadWriter) error {
	if !alpineMu.TryLock() {
		fmt.Fprint(out, "waiting for another session's setup…\r\n")
		alpineMu.Lock()
	}
	defer alpineMu.Unlock()
	if os.Getuid() != 0 {
		return errors.New("the jail needs root, as on the Kobo")
	}
	if _, err := os.Stat(alpineImg); err != nil {
		if err := alpineMakeImage(out); err != nil {
			return err
		}
	}
	if err := alpineMount(); err != nil {
		return err
	}
	if err := hostsReady(); err != nil { // before the jail: it's to follow the mounts there
		log.Printf("alpine: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(alpineRoot, alpineDone)); err == nil {
		return nil
	}
	return alpineInstall(out)
}

// alpineMakeImage writes the image, as big as out's answer says: zeros
// (FAT has no holes), then an ext2 file system. It's named .part till
// that's done.
func alpineMakeImage(out io.ReadWriter) error {
	most := alpineMBMax
	var st syscall.Statfs_t
	if err := syscall.Statfs(filepath.Dir(alpineImg), &st); err == nil {
		free := int(st.Bavail * uint64(st.Bsize) >> 20)
		if most = min(most, free-256); most < alpineMBMin { // 256 MB left for the books
			return fmt.Errorf("the image needs %d MB free, and there's %d MB", alpineMBMin+256, free)
		}
	}
	size, err := alpineAskSize(out, most)
	if err != nil {
		return err
	}
	part := alpineImg + ".part"
	f, err := os.OpenFile(part, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	zeros := make([]byte, 1<<20)
	for mb := 0; mb < size; mb++ {
		if mb%32 == 0 {
			fmt.Fprintf(out, "\rmaking the disk image (once): %d of %d MB", mb, size)
		}
		if _, err := f.Write(zeros); err != nil {
			f.Close()
			os.Remove(part)
			return err
		}
	}
	err = f.Sync()
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(part)
		return err
	}
	fmt.Fprintf(out, "\rmaking the disk image (once): %d MB, done\r\n", size)
	if b, err := exec.Command("mke2fs", "-F", "-m", "1", "-L", "alpine", part).CombinedOutput(); err != nil {
		os.Remove(part)
		return fmt.Errorf("mke2fs: %v: %s", err, bytes.TrimSpace(b))
	}
	return os.Rename(part, alpineImg)
}

// alpineAskSize asks on the session's terminal how big the image is to
// be, in MB. The PTY is as new, in line mode: the kernel echoes and edits
// the line, and Enter sends it.
func alpineAskSize(rw io.ReadWriter, most int) (int, error) {
	in := bufio.NewReader(rw)
	for {
		fmt.Fprintf(rw, "Alpine's disk image, made once: how big, in MB (%d to %d; Enter: %d)? ", alpineMBMin, most, min(alpineMBDef, most))
		line, err := in.ReadString('\n')
		if err != nil {
			return 0, errors.New("no size given")
		}
		mb, err := alpineSize(line, most)
		if err == nil {
			return mb, nil
		}
		fmt.Fprintf(rw, "%v\r\n", err)
	}
}

// alpineSize reads a size typed, in MB ("2048", "2048 MB"); empty is the
// default.
func alpineSize(s string, most int) (int, error) {
	s = strings.TrimSpace(strings.TrimSuffix(strings.ToLower(strings.TrimSpace(s)), "mb"))
	if s == "" {
		return min(alpineMBDef, most), nil
	}
	mb, err := strconv.Atoi(s)
	if err != nil || mb < alpineMBMin || mb > most {
		return 0, fmt.Errorf("a number of MB, %d to %d, please", alpineMBMin, most)
	}
	return mb, nil
}

// alpineMounted says whether the image is mounted at alpineRoot.
func alpineMounted() bool {
	b, _ := os.ReadFile("/proc/mounts")
	for _, line := range strings.Split(string(b), "\n") {
		if f := strings.Fields(line); len(f) > 1 && f[1] == alpineRoot {
			return true
		}
	}
	return false
}

// alpineMount mounts the image, through a loop device busybox's mount
// sets up and frees again. nodev: no device in it works, but the ones
// the jail's /dev gets (alpineinit.go).
func alpineMount() error {
	if alpineMounted() {
		return nil
	}
	if err := os.MkdirAll(alpineRoot, 0o755); err != nil {
		return err
	}
	if b, err := exec.Command("mount", "-t", "ext2", "-o", "loop,nosuid,nodev,noatime", alpineImg, alpineRoot).CombinedOutput(); err != nil {
		return fmt.Errorf("mounting the image: %v: %s", err, bytes.TrimSpace(b))
	}
	log.Printf("alpine: mounted %s at %s", alpineImg, alpineRoot)
	return nil
}

// alpineExit lets go of the image as we leave: the sessions are hung up
// on by then, and it goes once they're gone.
func alpineExit() {
	if !alpineMounted() {
		return
	}
	if err := syscall.Unmount(alpineRoot, syscall.MNT_DETACH); err != nil {
		log.Printf("alpine: unmounting: %v", err)
		return
	}
	log.Print("alpine: unmounted")
}

// alpineInstall fetches Alpine's minirootfs and unpacks it in the image.
func alpineInstall(out io.Writer) error {
	fmt.Fprintf(out, "fetching Alpine %s (3 MB)…\r\n", alpineVersion)
	b, err := fetchPinned(alpineURL, alpineSHA256)
	if err != nil {
		return err
	}
	fmt.Fprint(out, "unpacking…\r\n")
	if err := alpineExtract(bytes.NewReader(b), alpineRoot); err != nil {
		return fmt.Errorf("unpacking Alpine: %w", err)
	}
	for _, f := range []struct{ name, text string }{
		{"etc/hostname", "alpine\n"},
		{"etc/motd", "Alpine " + alpineVersion + " in a jail on the Kobo. apk add installs things.\n" +
			"The Kobo's storage is /mnt/onboard; its .adds and .kobo are read-only here.\n\n"},
		{alpineDone, alpineVersion + "\n"},
	} {
		if err := writeNew(filepath.Join(alpineRoot, f.name), f.text); err != nil {
			return err
		}
	}
	syscall.Sync()
	log.Printf("alpine: %s installed", alpineVersion)
	fmt.Fprint(out, "done.\r\n\n")
	return nil
}

const alpineRemoveWords = "yes delete it bro"

// alpineSummary is the Settings row's: what removing would delete.
func alpineSummary() string {
	if st, err := os.Stat(alpineImg); err == nil {
		return fmt.Sprintf("deletes alpine.img (%d MB), once you type it", st.Size()>>20)
	}
	return "none here"
}

// alpineRemoveAsk asks before Alpine goes, firmly: nothing in the image
// comes back.
func (a *bro) alpineRemoveAsk() {
	st, err := os.Stat(alpineImg)
	if err != nil {
		a.Toast("there's no Alpine to remove", 3*time.Second)
		return
	}
	if n := jailCount(); n > 0 {
		a.Toast(fmt.Sprintf("close the Alpine sessions first (%d open)", n), 5*time.Second)
		return
	}
	text := fmt.Sprintf("alpine.img (%d MB) goes, and everything in it: what apk put there, /root, all of it.", st.Size()>>20)
	a.askTyped("Remove Alpine?", text, alpineRemoveWords, "delete it", func() {
		go func() {
			err := alpineRemove()
			a.Do(func() {
				if err != nil {
					log.Printf("alpine: not removed: %v", err)
					a.Toast("Alpine not removed: "+err.Error(), 6*time.Second)
					return
				}
				a.Toast("Alpine removed; the next Alpine session sets it up anew", 5*time.Second)
				a.sv.shown = ""
				a.drawSettings()
			})
		}()
	})
}

// alpineRemove unmounts the image and deletes it, and the sound plugins
// fetched for it. Not lazily: whatever still holds it, it stays.
func alpineRemove() error {
	if !alpineMu.TryLock() {
		return errors.New("a session is setting it up")
	}
	defer alpineMu.Unlock()
	if n := jailCount(); n > 0 {
		return fmt.Errorf("%d Alpine sessions are open", n)
	}
	if alpineMounted() {
		if err := syscall.Unmount(alpineRoot, 0); err != nil {
			return fmt.Errorf("unmounting it: %w", err)
		}
		log.Print("alpine: unmounted")
	}
	if err := os.Remove(alpineImg); err != nil {
		return err
	}
	os.Remove(alpineImg + ".part")
	for _, keep := range bluealsaPlugins {
		os.Remove(keep)
	}
	syscall.Sync()
	log.Print("alpine: removed")
	return nil
}

// bluealsaReady fetches the bluealsa 3.1 plugins, once.
func bluealsaReady(out io.Writer) error {
	alpineMu.Lock()
	defer alpineMu.Unlock()
	missing := false
	for _, keep := range bluealsaPlugins {
		if _, err := os.Stat(keep); err != nil {
			missing = true
		}
	}
	if !missing {
		return nil
	}
	fmt.Fprint(out, "fetching the sound plugin, bluez-alsa 3.1.0 (once)…\r\n")
	b, err := fetchPinned(bluealsaURL, bluealsaSHA256)
	if err != nil {
		return err
	}
	zr, err := gzip.NewReader(bytes.NewReader(b)) // an .apk: gzipped tar, in three streams
	if err != nil {
		return err
	}
	tr := tar.NewReader(zr)
	found := 0
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		keep, ok := bluealsaPlugins[h.Name]
		if !ok || h.Typeflag != tar.TypeReg {
			continue
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			return err
		}
		if err := writeNew(keep, string(data)); err != nil {
			return err
		}
		found++
	}
	if found != len(bluealsaPlugins) {
		return fmt.Errorf("the package has %d of the %d plugins", found, len(bluealsaPlugins))
	}
	return nil
}

// fetchPinned gets url over HTTP, if what comes is what hash says.
func fetchPinned(url, hash string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching %s (is Wi-Fi on?): %w", path.Base(url), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching %s: %s", path.Base(url), resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return nil, fmt.Errorf("fetching %s: %w", path.Base(url), err)
	}
	if sum := sha256.Sum256(b); hex.EncodeToString(sum[:]) != hash {
		return nil, fmt.Errorf("%s isn't what it should be: its SHA-256 doesn't match", path.Base(url))
	}
	return b, nil
}

// writeNew writes a file afresh: if a symlink was there, it's replaced,
// not followed.
func writeNew(path, text string) error {
	os.Remove(path)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, err = f.WriteString(text)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

// alpineExtract unpacks a .tar.gz into dir, as it is: owners (when we're
// root), modes, symlinks, hard links. Nothing goes outside dir: not by
// "..", nor through a symlink in the way, whose target would be the
// Kobo's, not the jail's.
func alpineExtract(r io.Reader, dir string) error {
	zr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	tr, root := tar.NewReader(zr), os.Getuid() == 0
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		name := filepath.Clean("/" + h.Name)
		if name == "/" {
			continue
		}
		if err := noSymlinkIn(dir, filepath.Dir(name)); err != nil {
			return err
		}
		p := filepath.Join(dir, name)
		switch h.Typeflag {
		case tar.TypeDir:
			if st, err := os.Lstat(p); err != nil || !st.IsDir() {
				os.Remove(p)
				if err := os.Mkdir(p, 0o700); err != nil {
					return err
				}
			}
		case tar.TypeReg:
			os.Remove(p)
			f, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
			if err != nil {
				return err
			}
			_, err = io.Copy(f, tr)
			if cerr := f.Close(); err == nil {
				err = cerr
			}
			if err != nil {
				return err
			}
		case tar.TypeSymlink:
			os.Remove(p)
			if err := os.Symlink(h.Linkname, p); err != nil {
				return err
			}
		case tar.TypeLink:
			to := filepath.Clean("/" + h.Linkname)
			if err := noSymlinkIn(dir, filepath.Dir(to)); err != nil {
				return err
			}
			os.Remove(p)
			if err := os.Link(filepath.Join(dir, to), p); err != nil {
				return err
			}
		default: // devices, fifos: the jail makes its own /dev
			continue
		}
		if root {
			if err := os.Lchown(p, h.Uid, h.Gid); err != nil {
				return err
			}
		}
		if h.Typeflag != tar.TypeSymlink { // after chown, which clears setuid
			if err := syscall.Chmod(p, uint32(h.Mode&0o7777)); err != nil {
				return err
			}
		}
	}
}

// noSymlinkIn checks that no part of rel (a clean, absolute path, taken
// as under dir) is a symlink.
func noSymlinkIn(dir, rel string) error {
	p := dir
	for _, part := range strings.Split(strings.Trim(rel, "/"), "/") {
		if part == "" {
			continue
		}
		p = filepath.Join(p, part)
		st, err := os.Lstat(p)
		if errors.Is(err, os.ErrNotExist) {
			return nil // made later, as a directory
		}
		if err != nil {
			return err
		}
		if st.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s: a symlink in the way", strings.TrimPrefix(p, dir))
		}
	}
	return nil
}
