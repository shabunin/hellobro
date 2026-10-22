package main

// The jail's first process: hellobro --alpine-init, started by
// startAlpine in new mount, PID, UTS and IPC namespaces. It makes the
// jail, starts the shell in it and reaps what's left over; once the shell
// is gone, so is it, and with it every process in the jail.
//
// The jail: the image (alpine.go) as /, a /dev of a few devices (none of
// the Kobo's: not the screen, the input, ntx_io or the disks), its own
// /proc, a read-only /sys, and the Kobo's storage at /mnt/onboard, but
// .adds and .kobo read-only: what's there runs as root outside (the
// launcher, NickelMenu's entries, KoboRoot.tgz). The shell is root in
// it, as the storage is FAT and only root writes there, but with root's
// powers over the system taken away: no mount, modules, raw I/O, mknod,
// reboot, ptrace, clock or network setup, and no way to get them back
// (no_new_privs).
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
		"LANG=C.UTF-8", "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"}
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

func alpineJail() error {
	resolv, _ := os.ReadFile("/etc/resolv.conf") // the Kobo's, for the jail's
	if err := syscall.Mount("", "/", "", syscall.MS_REC|syscall.MS_PRIVATE, ""); err != nil {
		return fmt.Errorf("making mounts private: %w", err)
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
