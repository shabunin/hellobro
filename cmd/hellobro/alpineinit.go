package main

// The jail's first process: hellobro --alpine-init, started by
// startAlpine in new mount, PID, UTS and IPC namespaces. It makes the
// jail, starts the shell in it and reaps what's left over; once the shell
// is gone, so is it, and with it every process in the jail.
//
// The jail: the image (alpine.go) as /, a /dev of a few devices (none of
// the Kobo's: not the screen, the input, ntx_io or the disks), its own
// /proc, a read-only /sys, the hosts' files mounted from the SSH tab at
// /mnt/hosts (mount.go), and the Kobo's storage at /mnt/onboard, but
// .adds and .kobo read-only: what's there runs as root outside (the
// launcher, NickelMenu's entries, KoboRoot.tgz). The shell is root in
// it, as the storage is FAT and only root writes there, but with root's
// powers over the system taken away: no mount, modules, raw I/O, mknod,
// reboot, ptrace, clock or network setup, and no way to get them back
// (no_new_privs).
//
// If the jail has D-Bus (apk add dbus), the session gets a session bus
// of its own, at /run/user/0/bus: the players' remote control (MPRIS)
// is there, which hellobro sends the media keys to (mpris.go).
//
// With system bus access on (alpine.go), the jail gets the bus's socket,
// and for sound the bluealsa 3.1 plugins in /usr/local/lib/alsa-lib and
// an /etc/asound.conf that makes them ALSA's default (if it has none).
//
// Nothing is mounted or written at a path in the jail before pivot_root:
// an absolute symlink in it would point at the Kobo's own files then.

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

// alpineCaps are the capabilities the jail keeps, as Docker does, less
// DAC_READ_SEARCH (open_by_handle_at would open .adds' files through the
// writable storage), MKNOD and SYS_ADMIN's like.
var alpineCaps = map[int]bool{
	0:  true, // CHOWN
	1:  true, // DAC_OVERRIDE
	3:  true, // FOWNER
	4:  true, // FSETID
	5:  true, // KILL: the jail's own processes, it sees no others
	6:  true, // SETGID
	7:  true, // SETUID
	10: true, // NET_BIND_SERVICE
	13: true, // NET_RAW: ping
	18: true, // SYS_CHROOT
	29: true, // AUDIT_WRITE
	31: true, // SETFCAP: apk sets file capabilities
}

const prSetNoNewPrivs, prCapbsetDrop = 38, 24

func alpineInit(command string) {
	runtime.LockOSThread() // what's dropped is this thread's, and the shell starts from it
	if os.Getpid() != 1 {
		fmt.Fprintln(os.Stderr, "hellobro --alpine-init: only as a jail's first process")
		os.Exit(2)
	}
	if err := alpineJail(); err != nil {
		fmt.Fprintf(os.Stderr, "alpine: %v\r\n", err)
		os.Exit(1)
	}
	shell := rootShell()
	cmd := exec.Command(shell, "-l")
	if command != "" {
		cmd = exec.Command(shell, "-c", command)
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Env = []string{"TERM=xterm-256color", "HOME=/root", "USER=root", "LOGNAME=root", "SHELL=" + shell,
		"LANG=C.UTF-8", "PATH=" + jailPath}
	cmd.Env = append(cmd.Env, sessionBus()...)
	cmd.Dir = "/root"
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "alpine: %v\r\n", err)
		os.Exit(127)
	}
	for { // the first process reaps the orphans
		var ws syscall.WaitStatus
		pid, err := syscall.Wait4(-1, &ws, 0, nil)
		if err == syscall.EINTR {
			continue
		}
		if err != nil {
			os.Exit(0)
		}
		if pid == cmd.Process.Pid {
			if ws.Signaled() {
				os.Exit(128 + int(ws.Signal()))
			}
			os.Exit(ws.ExitStatus())
		}
	}
}

const jailPath = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

// sessionBus starts the session's bus, if the jail has D-Bus, and is
// what the shell's environment needs to find it. The bus goes with the
// jail, as everything in it does.
func sessionBus() []string {
	const daemon, dir = "/usr/bin/dbus-daemon", "/run/user/0"
	if _, err := os.Stat(daemon); err != nil {
		return nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		fmt.Fprintf(os.Stderr, "alpine: session bus: %v\r\n", err)
		return nil
	}
	d := exec.Command(daemon, "--session", "--address=unix:path="+dir+"/bus", "--nofork", "--nopidfile")
	d.Env = []string{"PATH=" + jailPath, "HOME=/root"}
	d.Dir = "/"
	if err := d.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "alpine: session bus: %v\r\n", err)
		return nil
	}
	return []string{"XDG_RUNTIME_DIR=" + dir, "DBUS_SESSION_BUS_ADDRESS=unix:path=" + dir + "/bus"}
}

// alsaConf is the jail's /etc/asound.conf, with the bus.
const alsaConf = `# hellobro: sound to the Kobo's Bluetooth headphones, through its
# bluealsa 3.1 (Settings, Alpine: system bus access). Alpine's own bluez-alsa is 4.x,
# which the Kobo's doesn't speak, so its plugins are 3.1.0's.
pcm_type.bluealsa { lib "/usr/local/lib/alsa-lib/libasound_module_pcm_bluealsa.so" }
ctl_type.bluealsa { lib "/usr/local/lib/alsa-lib/libasound_module_ctl_bluealsa.so" }
pcm.!default {
	type plug
	slave.pcm { type bluealsa; service "org.bluealsa"; device "00:00:00:00:00:00"; profile "a2dp"; delay 0 }
}
ctl.!default { type bluealsa; service "org.bluealsa"; battery "yes" }
`

func alpineJail() error {
	resolv, _ := os.ReadFile("/etc/resolv.conf") // the Kobo's, for the jail's
	bus := os.Getenv("HELLOBRO_ALPINE_BUS") == "1"
	plugins := map[string][]byte{} // by the name in the jail; none if they couldn't be fetched
	if bus {
		for name, keep := range bluealsaPlugins {
			if b, err := os.ReadFile(keep); err == nil {
				plugins["/usr/local/lib/alsa-lib/"+filepath.Base(name)] = b
			}
		}
	}
	// Slaves: nothing mounted here goes out. The Kobo's mounts are all
	// private, so this is private for them; only the hosts' folder is
	// shared (mount.go), and what's mounted there later comes in.
	if err := syscall.Mount("", "/", "", syscall.MS_REC|syscall.MS_SLAVE, ""); err != nil {
		return fmt.Errorf("making mounts slaves: %w", err)
	}
	old := filepath.Join(alpineRoot, ".oldroot")
	if st, err := os.Lstat(old); err == nil && !st.IsDir() {
		os.Remove(old)
	}
	os.Mkdir(old, 0o700)
	if err := syscall.PivotRoot(alpineRoot, old); err != nil {
		return fmt.Errorf("pivot_root: %w", err)
	}
	if err := os.Chdir("/"); err != nil {
		return err
	}
	// From here on, paths are the jail's; the Kobo's are under /.oldroot
	// till it goes.
	const (
		nosuid, nodev, noexec, rdonly = syscall.MS_NOSUID, syscall.MS_NODEV, syscall.MS_NOEXEC, syscall.MS_RDONLY
		bind                          = syscall.MS_BIND | syscall.MS_REC
	)
	for _, m := range []struct {
		src, dst, fs string
		flags        uintptr
		data         string
	}{
		{"tmpfs", "/dev", "tmpfs", nosuid | noexec, "mode=755,size=1m"},
		{"devpts", "/dev/pts", "devpts", nosuid | noexec, "newinstance,ptmxmode=0666,mode=0620,gid=5"},
		{"tmpfs", "/dev/shm", "tmpfs", nosuid | nodev, "mode=1777"},
		{"proc", "/proc", "proc", nosuid | nodev | noexec, ""},
		{"sysfs", "/sys", "sysfs", rdonly | nosuid | nodev | noexec, ""},
		{"tmpfs", "/tmp", "tmpfs", nosuid | nodev, "mode=1777"},
		{"tmpfs", "/run", "tmpfs", nosuid | nodev, "mode=755"},
		{"/.oldroot/mnt/onboard", "/mnt/onboard", "", bind, ""},
	} {
		if err := jailDir(m.dst); err != nil {
			return err
		}
		if err := syscall.Mount(m.src, m.dst, m.fs, m.flags, m.data); err != nil {
			return fmt.Errorf("mounting %s: %w", m.dst, err)
		}
		if m.flags&syscall.MS_BIND != 0 {
			if err := syscall.Mount("", m.dst, "", syscall.MS_BIND|syscall.MS_REMOUNT|nosuid|nodev, ""); err != nil {
				return fmt.Errorf("mounting %s: %w", m.dst, err)
			}
		}
	}
	// The hosts' folders, read-only: nothing in here can put a symlink
	// where hellobro mounts. The mounts in them are their own.
	if _, err := os.Lstat("/.oldroot" + hostsDir); err == nil {
		if err := jailDir("/mnt/hosts"); err != nil {
			return err
		}
		if err := syscall.Mount("/.oldroot"+hostsDir, "/mnt/hosts", "", bind, ""); err != nil {
			return fmt.Errorf("mounting /mnt/hosts: %w", err)
		}
		if err := syscall.Mount("", "/mnt/hosts", "", syscall.MS_BIND|syscall.MS_REMOUNT|rdonly|nosuid|nodev, ""); err != nil {
			return fmt.Errorf("mounting /mnt/hosts: %w", err)
		}
	}
	if bus {
		if err := jailDir("/run/dbus"); err != nil {
			return err
		}
		const sock = "/run/dbus/system_bus_socket"
		if err := writeNew(sock, ""); err != nil {
			return err
		}
		if err := syscall.Mount("/.oldroot/var/run/dbus/system_bus_socket", sock, "", syscall.MS_BIND, ""); err != nil {
			return fmt.Errorf("the system bus: %w", err)
		}
		if len(plugins) > 0 {
			if err := jailDir("/usr/local/lib/alsa-lib"); err != nil {
				return err
			}
		}
		for name, b := range plugins {
			if have, err := os.ReadFile(name); err == nil && string(have) == string(b) {
				continue
			}
			if err := writeNew(name, string(b)); err != nil {
				return err
			}
			os.Chmod(name, 0o755)
		}
		if _, err := os.Lstat("/etc/asound.conf"); err != nil && len(plugins) > 0 {
			writeNew("/etc/asound.conf", alsaConf)
		}
	}
	// Read-only: what runs as root outside the jail, and what of /proc
	// would change the Kobo itself.
	for _, p := range []string{"/mnt/onboard/.adds", "/mnt/onboard/.kobo", "/proc/sys", "/proc/sysrq-trigger", "/proc/irq", "/proc/bus"} {
		if _, err := os.Lstat(p); err != nil {
			continue
		}
		if err := noSymlinkIn("/", p); err != nil {
			return err
		}
		if err := syscall.Mount(p, p, "", bind, ""); err != nil {
			return fmt.Errorf("mounting %s: %w", p, err)
		}
		if err := syscall.Mount("", p, "", syscall.MS_BIND|syscall.MS_REMOUNT|rdonly|nosuid|nodev, ""); err != nil {
			return fmt.Errorf("mounting %s read-only: %w", p, err)
		}
	}
	old022 := syscall.Umask(0)
	for _, d := range []struct {
		name       string
		major, min int
	}{{"null", 1, 3}, {"zero", 1, 5}, {"full", 1, 7}, {"random", 1, 8}, {"urandom", 1, 9}, {"tty", 5, 0}} {
		if err := syscall.Mknod("/dev/"+d.name, syscall.S_IFCHR|0o666, d.major<<8|d.min); err != nil {
			return fmt.Errorf("/dev/%s: %w", d.name, err)
		}
	}
	syscall.Umask(old022)
	for name, to := range map[string]string{"ptmx": "pts/ptmx", "fd": "/proc/self/fd",
		"stdin": "/proc/self/fd/0", "stdout": "/proc/self/fd/1", "stderr": "/proc/self/fd/2"} {
		os.Symlink(to, "/dev/"+name)
	}
	if len(resolv) > 0 {
		writeNew("/etc/resolv.conf", string(resolv))
	}
	if err := syscall.Unmount("/.oldroot", syscall.MNT_DETACH); err != nil {
		return fmt.Errorf("leaving the Kobo's /: %w", err)
	}
	os.Remove("/.oldroot")
	host := "alpine"
	if b, err := os.ReadFile("/etc/hostname"); err == nil && len(strings.TrimSpace(string(b))) > 0 {
		host = strings.TrimSpace(string(b))
	}
	syscall.Sethostname([]byte(host))
	// Root's powers: gone from the bounding set, which is what root gets
	// on exec, and never to come back.
	for c := 0; c < 64; c++ {
		if alpineCaps[c] {
			continue
		}
		if _, _, e := syscall.RawSyscall6(syscall.SYS_PRCTL, prCapbsetDrop, uintptr(c), 0, 0, 0, 0); e == syscall.EINVAL {
			break // past the last one this kernel has
		} else if e != 0 {
			return fmt.Errorf("dropping capability %d: %w", c, e)
		}
	}
	if _, _, e := syscall.RawSyscall6(syscall.SYS_PRCTL, prSetNoNewPrivs, 1, 0, 0, 0, 0); e != 0 {
		return fmt.Errorf("no_new_privs: %w", e)
	}
	return nil
}

// jailDir makes sure dir is a directory, with no symlink on the way to it.
func jailDir(dir string) error {
	if err := noSymlinkIn("/", dir); err != nil {
		return err
	}
	if st, err := os.Lstat(dir); err == nil {
		if !st.IsDir() {
			return fmt.Errorf("%s isn't a directory", dir)
		}
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

// rootShell is root's shell in the jail's /etc/passwd (apk add bash, and
// chsh), or /bin/sh.
func rootShell() string {
	f, err := os.Open("/etc/passwd")
	if err != nil {
		return "/bin/sh"
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if f := strings.Split(sc.Text(), ":"); len(f) >= 7 && f[0] == "root" {
			if st, err := os.Stat(f[6]); err == nil && st.Mode()&0o111 != 0 {
				return f[6]
			}
		}
	}
	return "/bin/sh"
}
