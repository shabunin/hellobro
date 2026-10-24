package main

// The Shell tab's backend: /bin/sh on the Kobo itself, in a pseudo-terminal.

import (
	"errors"
	"fmt"
	"image"
	"io"
	"log"
	"os"
	"os/exec"
	"os/user"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"hellobro/kobo"
)

type localShell struct {
	pty  *os.File // the master side
	mu   sync.Mutex
	cmd  *exec.Cmd     // once it's started
	shut bool          // closed: nothing more is started
	exit chan struct{} // closed once the shell is gone
	err  error         // how it went, once exit is closed
}

// openPTY makes a new PTY of cols×rows: our side, and the shell's.
func openPTY(cols, rows int) (pty, tty *os.File, err error) {
	ptmx, err := ptyMaster()
	if err != nil {
		return nil, nil, err
	}
	// O_NOCTTY: the PTY becomes the shell's controlling terminal, not ours.
	pty, err = os.OpenFile(ptmx, os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, nil, err
	}
	var n uint32
	unlock := int32(0)
	if err := ioctl(pty, syscall.TIOCSPTLCK, unsafe.Pointer(&unlock)); err != nil {
		pty.Close()
		return nil, nil, fmt.Errorf("unlockpt: %w", err)
	}
	if err := ioctl(pty, syscall.TIOCGPTN, unsafe.Pointer(&n)); err != nil {
		pty.Close()
		return nil, nil, fmt.Errorf("ptsname: %w", err)
	}
	tty, err = os.OpenFile(fmt.Sprintf("/dev/pts/%d", n), os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		pty.Close()
		return nil, nil, err
	}
	setWinsize(pty, cols, rows, image.Point{})
	return pty, tty, nil
}

// startShell runs a login-ish shell on a new PTY of cols×rows; or, given
// a command, the shell runs that.
func startShell(cols, rows int, command ...string) (*localShell, error) {
	pty, tty, err := openPTY(cols, rows)
	if err != nil {
		return nil, err
	}
	defer tty.Close() // the shell has its own copies

	shell := "/bin/sh"
	cmd := exec.Command(shell, "-l")
	if len(command) > 0 && command[0] != "" {
		cmd = exec.Command(shell, "-c", command[0])
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = tty, tty, tty
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true, Ctty: 0}
	cmd.Env = shellEnv()
	cmd.Dir = "/"
	for _, d := range []string{"/mnt/onboard", os.Getenv("HOME")} {
		if st, err := os.Stat(d); d != "" && err == nil && st.IsDir() {
			cmd.Dir = d
			break
		}
	}
	s := &localShell{pty: pty, exit: make(chan struct{})}
	if err := s.start(cmd); err != nil {
		pty.Close()
		return nil, err
	}
	return s, nil
}

// start runs cmd as the shell, unless it was closed meanwhile.
func (s *localShell) start(cmd *exec.Cmd) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.shut {
		return errors.New("closed")
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	s.cmd = cmd
	go func() {
		s.err = cmd.Wait()
		close(s.exit)
	}()
	return nil
}

// fail ends a shell that never started.
func (s *localShell) fail(err error) {
	s.err = err
	close(s.exit)
}

// ptyMaster finds the PTY multiplexer. The Kobo has neither /dev/ptmx nor
// devpts mounted (Nickel needs no terminals), so we mount devpts at
// /dev/pts, which has its own ptmx; it stays mounted until a reboot.
func ptyMaster() (string, error) {
	if _, err := os.Stat("/dev/pts/ptmx"); err != nil {
		if err := os.MkdirAll("/dev/pts", 0o755); err != nil {
			return "", err
		}
		if err := syscall.Mount("devpts", "/dev/pts", "devpts", syscall.MS_NOSUID|syscall.MS_NOEXEC, "mode=620,ptmxmode=666"); err != nil {
			return "", fmt.Errorf("no PTYs here: mounting devpts on /dev/pts: %w", err)
		}
		log.Print("shell: mounted devpts on /dev/pts")
	}
	if _, err := os.Stat("/dev/ptmx"); err == nil {
		return "/dev/ptmx", nil
	}
	// For other programs that look for it; ours does without.
	if err := syscall.Mknod("/dev/ptmx", syscall.S_IFCHR|0o666, 5<<8|2); err != nil {
		log.Printf("shell: no /dev/ptmx, and making it: %v", err)
	}
	return "/dev/pts/ptmx", nil
}

func whoami() string {
	if u, err := user.Current(); err == nil {
		return u.Username
	}
	return "uid " + strconv.Itoa(os.Getuid())
}

// shellEnv is ours, minus what's only for hellobro, plus what a terminal sets.
func shellEnv() []string {
	var env []string
	home := false
	for _, kv := range os.Environ() {
		k, v, _ := strings.Cut(kv, "=")
		switch {
		case k == "TERM" || k == "PS1" || strings.HasPrefix(k, "HELLOBRO_") || strings.HasPrefix(k, "FAKE_"):
			continue
		case k == "HOME" && v != "" && v != "/":
			home = true
		case k == "HOME":
			continue
		}
		env = append(env, kv)
	}
	if !home {
		env = append(env, "HOME="+kobo.AppFile(""))
	}
	host, _ := os.Hostname()
	if host == "" || host == "(none)" || host == "localhost" {
		host = "kobo"
	}
	return append(env, "TERM=xterm-256color", `PS1=\u@`+host+`:\w\$ `)
}

func ioctl(f *os.File, req uintptr, arg unsafe.Pointer) error {
	c, err := f.SyscallConn() // not Fd(): that would make reads block, and close() wouldn't stop them
	if err != nil {
		return err
	}
	var errno syscall.Errno
	if err := c.Control(func(fd uintptr) {
		_, _, errno = syscall.Syscall(syscall.SYS_IOCTL, fd, req, uintptr(arg))
	}); err != nil {
		return err
	}
	if errno != 0 {
		return errno
	}
	return nil
}

func (s *localShell) Read(p []byte) (int, error) {
	n, err := s.pty.Read(p)
	if errors.Is(err, syscall.EIO) { // what a PTY says once the other side is all closed
		err = io.EOF
	}
	return n, err
}

func (s *localShell) Write(p []byte) (int, error) { return s.pty.Write(p) }

func (s *localShell) resize(cols, rows int, px image.Point) { setWinsize(s.pty, cols, rows, px) }

func setWinsize(pty *os.File, cols, rows int, px image.Point) {
	ws := struct{ row, col, x, y uint16 }{uint16(rows), uint16(cols), uint16(px.X), uint16(px.Y)}
	ioctl(pty, syscall.TIOCSWINSZ, unsafe.Pointer(&ws))
}

// close hangs up on the shell and everything it started.
func (s *localShell) close() {
	s.mu.Lock()
	s.shut = true
	cmd := s.cmd
	s.mu.Unlock()
	if cmd != nil && cmd.Process != nil {
		p := cmd.Process
		syscall.Kill(-p.Pid, syscall.SIGHUP)
		go func() {
			select {
			case <-s.exit:
			case <-time.After(2 * time.Second):
				syscall.Kill(-p.Pid, syscall.SIGKILL)
			}
		}()
	}
	s.pty.Close()
}

func (s *localShell) ended() (lost bool, how string) {
	select {
	case <-s.exit:
	case <-time.After(3 * time.Second): // its output ended, but it lingers: close() hangs up on it
		return false, ""
	}
	var exit *exec.ExitError
	switch {
	case s.err == nil:
		return false, ""
	case errors.As(s.err, &exit):
		if st, ok := exit.Sys().(syscall.WaitStatus); ok && st.Signaled() {
			return false, "killed by " + st.Signal().String()
		}
		return false, fmt.Sprintf("exit status %d", exit.ExitCode())
	}
	return false, s.err.Error()
}
