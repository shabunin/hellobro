package main

// Mounting a saved host's files, from its menu on the SSH tab: we log in
// as the SSH tab does (our key, or a password asked on the session's
// page), open SFTP, and hand it to a process of our own that serves it
// as a folder (sftpfs.go), read-only or read-write as the user said.
//
// The folders are under hostsDir, a tmpfs of ours made shared, so a
// mount there shows in the Alpine sessions too, at /mnt/hosts
// (alpineinit.go binds it in read-only, and its mounts follow it in, not
// out). They go before sleeping, as Wi-Fi does, and come back on waking;
// when we leave, they go for good. We never look in them ourselves: a
// host gone quiet would hang whoever does.

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"golang.org/x/crypto/ssh"

	"hellobro/kobo"
)

var hostsDir = kobo.EnvOr("HELLOBRO_HOSTS", "/tmp/hellobro-hosts")

// hostMount is a host's files, mounted.
type hostMount struct {
	host   sshHost
	rw     bool
	dir    string
	client *ssh.Client
	cmd    *exec.Cmd
	done   chan struct{} // closed by unmount
	lost   atomic.Bool   // the keepalives went unanswered
	asked  bool          // unmounted by the user, or for sleep: no news
}

// mountState is the mounts, by host name, and those sleep took down.
type mountState struct {
	on    map[string]*hostMount
	again []sshHostRW // to mount again on waking
}

type sshHostRW struct {
	host sshHost
	rw   bool
}

func mountMode(rw bool) string { return map[bool]string{false: "read-only", true: "read-write"}[rw] }

// hostDirName is the folder name for a host: its name, made safe.
func hostDirName(name string) string {
	name = strings.Map(func(r rune) rune {
		if r == '/' || r < ' ' {
			return '_'
		}
		return r
	}, name)
	if name == "" || strings.HasPrefix(name, ".") {
		name = "_" + name
	}
	return name
}

// hostsReady mounts hostsDir, if it isn't, as a shared tmpfs: before the
// first jail, so the jails get it as slaves.
func hostsReady() error {
	if isMountPoint(hostsDir) {
		return nil
	}
	if err := os.MkdirAll(hostsDir, 0o755); err != nil {
		return err
	}
	if err := syscall.Mount("hellobro-hosts", hostsDir, "tmpfs", syscall.MS_NOSUID|syscall.MS_NODEV|syscall.MS_NOEXEC, "mode=755,size=64k"); err != nil {
		return fmt.Errorf("mounting %s: %w", hostsDir, err)
	}
	if err := syscall.Mount("", hostsDir, "", syscall.MS_SHARED, ""); err != nil {
		syscall.Unmount(hostsDir, syscall.MNT_DETACH)
		return fmt.Errorf("sharing %s: %w", hostsDir, err)
	}
	log.Printf("mount: %s ready", hostsDir)
	return nil
}

func isMountPoint(dir string) bool {
	b, _ := os.ReadFile("/proc/mounts")
	for _, line := range strings.Split(string(b), "\n") {
		if f := strings.Fields(line); len(f) > 1 && f[1] == dir {
			return true
		}
	}
	return false
}

func (a *bro) mounted(h sshHost) *hostMount { return a.mnt.on[h.name] }

// mountHost mounts h, rw or not; the questions logging in asks go to t's
// page, as connecting's do. quiet: no questions (waking), a password
// needed fails.
func (a *bro) mountHost(t *termSession, h sshHost, rw, quiet bool) {
	if a.mnt.on == nil {
		a.mnt.on = map[string]*hostMount{}
	}
	if a.mnt.on[h.name] != nil {
		return
	}
	var o dialOpts
	gen := -1
	if t != nil && !quiet {
		if t.state != termIdle {
			return
		}
		t.notes, t.state, t.target = nil, termConnecting, h
		t.gen++
		gen = t.gen
		o = a.dialPrompts(t, gen, h)
		o.password = t.password
		a.termNote(t, fmt.Sprintf("mounting %s (%s)…", h, mountMode(rw)))
	} else {
		o = dialOpts{
			ask:  func(string, bool) (string, error) { return "", errors.New("it asks for a password") },
			note: func(s string) { log.Printf("mount: %s", s) },
		}
	}
	m := &hostMount{host: h, rw: rw, dir: filepath.Join(hostsDir, hostDirName(h.name)), done: make(chan struct{})}
	a.mnt.on[h.name] = m // taken, while it's on its way
	go func() {
		err := m.start(o)
		a.Do(func() {
			if t != nil && gen >= 0 {
				if t.gen != gen { // canceled meanwhile
					if err == nil {
						m.asked = true
						m.stop()
					}
					delete(a.mnt.on, h.name)
					return
				}
				t.prompt, t.state = nil, termIdle
			}
			if err != nil {
				delete(a.mnt.on, h.name)
				log.Printf("mount %s: %v", h.name, err)
				if t != nil && gen >= 0 {
					a.termNote(t, "couldn't mount: "+err.Error())
				} else {
					a.Toast(fmt.Sprintf("%s: not mounted again: %v", h.name, err), 6*time.Second)
				}
				return
			}
			log.Printf("mount: %s at %s, %s", h, m.dir, mountMode(rw))
			if t != nil && gen >= 0 {
				a.termNote(t, fmt.Sprintf("%s mounted, %s: /mnt/hosts/%s in Alpine (%s on the Kobo)", h.name, mountMode(rw), filepath.Base(m.dir), m.dir))
			}
			a.Toast(fmt.Sprintf("%s mounted at /mnt/hosts/%s", h.name, filepath.Base(m.dir)), 4*time.Second)
			a.redrawHosts()
			go func() {
				m.cmd.Wait()
				a.Do(func() { a.mountGone(m) })
			}()
		})
	}()
}

// start logs in, opens SFTP and starts the process that serves it.
func (m *hostMount) start(o dialOpts) error {
	if err := hostsReady(); err != nil {
		return err
	}
	if st, err := os.Lstat(m.dir); err == nil && !st.IsDir() {
		return fmt.Errorf("%s is in the way", m.dir)
	}
	if err := os.Mkdir(m.dir, 0o755); err != nil && !errors.Is(err, os.ErrExist) {
		return err
	}
	client, _, err := dialClient(m.host, o)
	if err != nil {
		os.Remove(m.dir)
		return err
	}
	fail := func(err error) error {
		client.Close()
		os.Remove(m.dir)
		return err
	}
	sess, err := client.NewSession()
	if err != nil {
		return fail(err)
	}
	toHost, err := sess.StdinPipe()
	if err != nil {
		return fail(err)
	}
	fromHost, err := sess.StdoutPipe()
	if err != nil {
		return fail(err)
	}
	if err := sess.RequestSubsystem("sftp"); err != nil {
		return fail(fmt.Errorf("the host has no SFTP: %w", err))
	}
	// Pipes of our own, so the process's end is a file, and copies both ways.
	inR, inW, err := os.Pipe()
	if err != nil {
		return fail(err)
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		inR.Close()
		inW.Close()
		return fail(err)
	}
	stR, stW, err := os.Pipe()
	if err != nil {
		inR.Close()
		inW.Close()
		outR.Close()
		outW.Close()
		return fail(err)
	}
	mode := "ro"
	if m.rw {
		mode = "rw"
	}
	cmd := exec.Command("/proc/self/exe", "--mount", m.dir, mode)
	cmd.Stdin, cmd.Stdout = inR, outW
	errW := logPipe("mount " + m.host.name)
	if errW != nil {
		cmd.Stderr = errW
	}
	cmd.ExtraFiles = []*os.File{stW}
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGTERM} // we're gone: so is it (a dead mount says so)
	err = cmd.Start()
	inR.Close()
	outW.Close()
	stW.Close()
	if errW != nil {
		errW.Close()
	}
	if err != nil {
		inW.Close()
		outR.Close()
		stR.Close()
		return fail(err)
	}
	go func() {
		io.Copy(inW, fromHost)
		inW.Close()
	}()
	go func() {
		io.Copy(toHost, outR)
		outR.Close()
		client.Close()
	}()
	st, _ := io.ReadAll(io.LimitReader(stR, 4096))
	stR.Close()
	if s := strings.TrimSpace(string(st)); s != "ok" {
		client.Close()
		cmd.Wait()
		os.Remove(m.dir)
		if s == "" {
			s = "it quit"
		}
		return errors.New(s)
	}
	m.client, m.cmd = client, cmd
	go keepalive(client, m.done, &m.lost)
	return nil
}

// stop unmounts: lazily, so whoever's in it isn't held up, and what's
// left of it fails instead of hanging. The process goes once the kernel
// lets go.
func (m *hostMount) stop() {
	select {
	case <-m.done:
		return
	default:
		close(m.done)
	}
	if err := syscall.Unmount(m.dir, syscall.MNT_DETACH); err != nil {
		log.Printf("mount: unmounting %s: %v", m.dir, err)
	}
	if m.client != nil {
		m.client.Close()
	}
}

// mountGone is a mount's process ended: unmounted by us, or the host
// went.
func (a *bro) mountGone(m *hostMount) {
	if a.mnt.on[m.host.name] == m {
		delete(a.mnt.on, m.host.name)
	}
	m.stop()
	os.Remove(m.dir)
	if !m.asked {
		why := "the connection's gone"
		if m.lost.Load() {
			why = "the host stopped answering"
		}
		log.Printf("mount: %s unmounted: %s", m.host.name, why)
		a.Toast(fmt.Sprintf("%s unmounted: %s", m.host.name, why), 6*time.Second)
	}
	a.redrawHosts()
}

func (a *bro) unmountHost(h sshHost) {
	m := a.mnt.on[h.name]
	if m == nil || m.cmd == nil {
		return
	}
	m.asked = true
	m.stop()
	log.Printf("mount: %s unmounted", h.name)
	a.Toast(h.name+" unmounted", 3*time.Second)
}

// unmountAll takes them all down: before sleep (again: to mount them
// again on waking) and when we leave.
func (a *bro) unmountAll(again bool) {
	for _, m := range a.mnt.on {
		if m.cmd == nil { // still on its way: it finds itself canceled
			continue
		}
		if again {
			a.mnt.again = append(a.mnt.again, sshHostRW{m.host, m.rw})
		}
		m.asked = true
		m.stop()
	}
}

// mountAgain brings back what sleep took down, once the network's up.
func (a *bro) mountAgain() {
	for _, m := range a.mnt.again {
		a.mountHost(nil, m.host, m.rw, true)
	}
	a.mnt.again = nil
}

// mountsExit lets go of hostsDir too, as we leave.
func mountsExit() {
	if isMountPoint(hostsDir) {
		syscall.Unmount(hostsDir, syscall.MNT_DETACH)
	}
}

// logPipe is a pipe whose lines go to the log, till its writers close
// it; nil if there's none to be had.
func logPipe(prefix string) *os.File {
	r, w, err := os.Pipe()
	if err != nil {
		return nil
	}
	go func() {
		defer r.Close()
		sc := bufio.NewScanner(r)
		for sc.Scan() {
			log.Printf("%s: %s", prefix, sc.Text())
		}
	}()
	return w
}
