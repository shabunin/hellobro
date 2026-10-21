package main

// SSH for the SSH tab: hosts from ssh.conf, our own key (made on
// first use), trust-on-first-use host keys in known_hosts, and password /
// keyboard-interactive prompts answered from the Kobo's keyboard. A host
// can be reached through another (ssh -J): "via kobo@vps" in ssh.conf.

import (
	"bufio"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"hellobro/kobo"
)

type sshHost struct {
	name, user, addr string   // addr is host:port
	via              *sshHost // the jump host it's reached through, or nil
	cmd              string   // run instead of a login shell, e.g. tmux new -A -s kobo
}

func (h sshHost) String() string {
	if h.via != nil {
		return h.user + "@" + h.addr + " via " + h.via.user + "@" + h.via.addr
	}
	return h.user + "@" + h.addr
}

// line is h as a line of ssh.conf.
func (h sshHost) line() string { return strings.TrimSpace(h.name + "  " + h.String() + "  " + h.cmd) }

func (h sshHost) same(o sshHost) bool { return h.String() == o.String() && h.cmd == o.cmd }

const sshConfExample = `# hellobro ssh hosts, one per line:
#   name  user@host[:port]  [via user@jumphost[:port]]  [command]
# The command, if any, runs instead of a login shell; with tmux, a dropped
# connection comes back where it was. "via" reaches the host through a
# jump host, as ssh -J does: a VPS both ends can reach, say, where the
# PC keeps a tunnel open to its sshd (see the README).
# hellobro-server  bro@192.168.1.10:2222
# pc  me@192.168.1.10  tmux new -A -s kobo
# home  me@127.0.0.1:2222  via kobo@203.0.113.7  tmux new -A -s kobo
`

// loadSSHHosts reads ssh.conf, creating an example if there's none.
func loadSSHHosts() []sshHost {
	path := kobo.AppFile("ssh.conf")
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		os.WriteFile(path, []byte(sshConfExample), 0o644)
		return nil
	}
	if err != nil {
		return nil
	}
	defer f.Close()
	var hosts []sshHost
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if h, ok := parseSSHConfLine(sc.Text()); ok {
			hosts = append(hosts, h)
		}
	}
	return hosts
}

// parseSSHConfLine reads a line of ssh.conf: name  user@host[:port]
// [via user@jumphost[:port]]  [command].
func parseSSHConfLine(line string) (sshHost, bool) {
	fields := strings.Fields(line)
	if len(fields) < 2 || strings.HasPrefix(fields[0], "#") {
		return sshHost{}, false
	}
	h, err := parseSSHTarget(fields[1])
	if err != nil {
		return sshHost{}, false
	}
	rest := fields[2:]
	if len(rest) >= 2 && rest[0] == "via" {
		via, err := parseSSHTarget(rest[1])
		if err != nil {
			return sshHost{}, false
		}
		h.via, rest = &via, rest[2:]
	}
	h.name, h.cmd = fields[0], strings.Join(rest, " ")
	return h, true
}

// saveSSHHost adds h to ssh.conf, under a name no other host has; it
// returns that name.
func saveSSHHost(hosts []sshHost, h sshHost) (string, error) {
	base, _, _ := net.SplitHostPort(h.addr)
	base = strings.Trim(base, "[]")
	name := base
	for n := 2; slices.ContainsFunc(hosts, func(o sshHost) bool { return o.name == name }); n++ {
		name = fmt.Sprintf("%s-%d", base, n)
	}
	h.name = name
	path := kobo.AppFile("ssh.conf")
	b, _ := os.ReadFile(path)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if len(b) > 0 && b[len(b)-1] != '\n' {
		fmt.Fprintln(f)
	}
	_, err = fmt.Fprintln(f, h.line())
	return name, err
}

// parseSSHTarget reads user@host[:port] (user defaults to root, port to
// 22), or the same as an ssh command line, since that's what fingers type:
// "ssh -p 2222 bro@host", "ssh -l bro host", "ssh://bro@host:2222". As
// with ssh, anything after the host is a command to run there.
func parseSSHTarget(s string) (sshHost, error) {
	fields := strings.Fields(s)
	if len(fields) > 0 && fields[0] == "ssh" {
		fields = fields[1:]
	}
	var target, user, port, cmd, jump string
	for i := 0; i < len(fields); i++ {
		f := fields[i]
		switch {
		case target != "" && !strings.HasPrefix(f, "-"):
			cmd = strings.Join(fields[i:], " ")
			i = len(fields)
		case f == "-t" || f == "-tt": // we always ask for a terminal
		case (f == "-p" || f == "-l" || f == "-J") && i+1 < len(fields):
			i++
			switch f {
			case "-p":
				port = fields[i]
			case "-l":
				user = fields[i]
			default:
				jump = fields[i]
			}
		case strings.HasPrefix(f, "-p") && len(f) > 2:
			port = f[2:]
		case strings.HasPrefix(f, "-l") && len(f) > 2:
			user = f[2:]
		case strings.HasPrefix(f, "-"):
			return sshHost{}, fmt.Errorf("only -p PORT, -l USER and -J JUMPHOST are understood, not %s", f)
		default:
			target = strings.TrimSuffix(strings.TrimPrefix(f, "ssh://"), "/")
		}
	}
	host := target
	if u, h, ok := strings.Cut(target, "@"); ok {
		host = h
		if user == "" {
			user = u
		}
	}
	if h, p, err := net.SplitHostPort(host); err == nil {
		host = h
		if port == "" {
			port = p
		}
	}
	if user == "" {
		user = "root"
	}
	if port == "" {
		port = "22"
	}
	if host == "" {
		return sshHost{}, fmt.Errorf("want user@host[:port], got %q", s)
	}
	if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
		return sshHost{}, fmt.Errorf("bad port %q", port)
	}
	h := sshHost{name: target, user: user, addr: net.JoinHostPort(host, port), cmd: cmd}
	if jump != "" {
		via, err := parseSSHTarget(jump)
		if err != nil {
			return sshHost{}, fmt.Errorf("-J: %w", err)
		}
		h.via = &via
	}
	return h, nil
}

// clientKey loads id_ed25519, or makes one (plus id_ed25519.pub, to copy
// into a server's authorized_keys).
func clientKey() (ssh.Signer, error) {
	path := kobo.AppFile("id_ed25519")
	if b, err := os.ReadFile(path); err == nil {
		return ssh.ParsePrivateKey(b)
	}
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	block, err := ssh.MarshalPrivateKey(priv, "hellobro@kobo")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0o600); err != nil {
		return nil, err
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		return nil, err
	}
	os.WriteFile(path+".pub", []byte(authorizedKey(signer)+"\n"), 0o644)
	return signer, nil
}

func authorizedKey(s ssh.Signer) string {
	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(s.PublicKey()))) + " hellobro@kobo"
}

// tofuHostKeys accepts and remembers a host's key the first time, and
// refuses a different one afterwards. note gets what happened.
func tofuHostKeys(note func(string)) ssh.HostKeyCallback {
	path := kobo.AppFile("known_hosts")
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		if f, err := os.OpenFile(path, os.O_CREATE|os.O_RDONLY, 0o644); err == nil {
			f.Close()
		}
		check, err := knownhosts.New(path)
		if err != nil {
			return err
		}
		err = check(hostname, remote, key)
		var kerr *knownhosts.KeyError
		switch {
		case err == nil:
			return nil
		case errors.As(err, &kerr) && len(kerr.Want) == 0: // never seen: trust it
			f, ferr := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
			if ferr != nil {
				return ferr
			}
			defer f.Close()
			fmt.Fprintln(f, knownhosts.Line([]string{knownhosts.Normalize(hostname)}, key))
			note(fmt.Sprintf("new host key %s, remembered", ssh.FingerprintSHA256(key)))
			return nil
		case errors.As(err, &kerr):
			return fmt.Errorf("host key CHANGED (%s; %s:%d has another): remove that line to trust the new one",
				ssh.FingerprintSHA256(key), path, kerr.Want[0].Line)
		}
		return err
	}
}

// sshDial connects to h, through its jump host if it has one, as ssh -J
// does: we log in to the jump host with our key, and it only carries the
// connection to h, end to end.
func sshDial(h sshHost, cfg *ssh.ClientConfig, signer ssh.Signer) (*ssh.Client, error) {
	var jump *ssh.Client
	var conn net.Conn
	var err error
	if h.via == nil {
		conn, err = net.DialTimeout("tcp", h.addr, cfg.Timeout)
	} else {
		jump, err = sshDial(*h.via, &ssh.ClientConfig{
			User: h.via.user, Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)},
			HostKeyCallback: cfg.HostKeyCallback, Timeout: cfg.Timeout,
		}, signer)
		if err != nil {
			return nil, fmt.Errorf("jump host %s@%s: %w", h.via.user, h.via.addr, err)
		}
		if conn, err = jump.Dial("tcp", h.addr); err != nil {
			jump.Close()
			return nil, fmt.Errorf("through %s to %s: %w", h.via.addr, h.addr, err)
		}
	}
	if err != nil {
		return nil, err
	}
	// A handshake that never finishes, the server gone quiet: give up
	// (prompts get their time: a password typed is part of it).
	stuck := time.AfterFunc(2*time.Minute, func() { conn.Close() })
	c, chans, reqs, err := ssh.NewClientConn(conn, h.addr, cfg)
	stuck.Stop()
	if err != nil {
		conn.Close()
		if jump != nil {
			jump.Close()
		}
		return nil, err
	}
	client := ssh.NewClient(c, chans, reqs)
	if jump != nil {
		go func() {
			client.Wait()
			jump.Close() // the jump host's connection goes with the one it carried
		}()
	}
	return client, nil
}

// sshSession is a connected shell with a PTY.
type sshSession struct {
	client  *ssh.Client
	session *ssh.Session
	stdin   io.WriteCloser
	stdout  io.Reader
	lost    atomic.Bool // the keepalives went unanswered
	done    chan struct{}
}

// prompter asks the user something; echo=false for passwords.
type prompter func(question string, echo bool) (string, error)

type dialOpts struct {
	cols, rows int
	password   string // tried first, if set: the one typed for the last connection
	ask        prompter
	confirm    func(question string) bool // nil: don't offer to install our key
	note       func(string)
}

// dialSSH connects, and returns the session and the password typed for
// it, if one was.
func dialSSH(h sshHost, o dialOpts) (*sshSession, string, error) {
	signer, err := clientKey()
	if err != nil {
		return nil, "", fmt.Errorf("client key: %w", err)
	}
	saved, typed := o.password, ""
	password := func(q string) (string, error) {
		if saved != "" {
			p := saved
			saved = ""
			return p, nil
		}
		p, err := o.ask(q, false)
		typed = p
		return p, err
	}
	cfg := &ssh.ClientConfig{
		User: h.user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
			ssh.KeyboardInteractive(func(name, instruction string, questions []string, echos []bool) ([]string, error) {
				if instruction != "" {
					o.note(instruction)
				}
				answers := make([]string, len(questions))
				for i, q := range questions {
					var a string
					var err error
					if echos[i] {
						a, err = o.ask(q, true)
					} else {
						a, err = password(q)
					}
					if err != nil {
						return nil, err
					}
					answers[i] = a
				}
				return answers, nil
			}),
			ssh.RetryableAuthMethod(ssh.PasswordCallback(func() (string, error) {
				return password(fmt.Sprintf("%s's password: ", h))
			}), 3),
		},
		HostKeyCallback: tofuHostKeys(o.note),
		Timeout:         10 * time.Second,
	}
	client, err := sshDial(h, cfg, signer)
	if err != nil {
		return nil, "", err
	}
	if typed != "" && o.confirm != nil &&
		o.confirm(fmt.Sprintf("Put this Kobo's key in %s's ~/.ssh/authorized_keys, so no password next time?", h)) {
		if err := installKey(client, authorizedKey(signer)); err != nil {
			o.note("couldn't install the key: " + err.Error())
		} else {
			o.note("key installed: next time needs no password")
		}
	}
	sess, err := client.NewSession()
	if err != nil {
		client.Close()
		return nil, "", err
	}
	fail := func(err error) (*sshSession, string, error) {
		sess.Close()
		client.Close()
		return nil, "", err
	}
	modes := ssh.TerminalModes{ssh.ECHO: 1, ssh.TTY_OP_ISPEED: 115200, ssh.TTY_OP_OSPEED: 115200}
	if err := sess.RequestPty("xterm-256color", o.rows, o.cols, modes); err != nil {
		return fail(fmt.Errorf("pty: %w", err))
	}
	sess.Setenv("LANG", "C.UTF-8") // most servers refuse it; harmless
	s := &sshSession{client: client, session: sess, done: make(chan struct{})}
	if s.stdin, err = sess.StdinPipe(); err != nil {
		return fail(err)
	}
	if s.stdout, err = sess.StdoutPipe(); err != nil {
		return fail(err)
	}
	sess.Stderr = nil // with a PTY, stderr comes through stdout
	if h.cmd != "" {
		err = sess.Start(h.cmd)
	} else {
		err = sess.Shell()
	}
	if err != nil {
		return fail(fmt.Errorf("shell: %w", err))
	}
	go s.keepalive()
	return s, typed, nil
}

// installKey appends key to the remote's authorized_keys, unless it's
// there already.
func installKey(client *ssh.Client, key string) error {
	sess, err := client.NewSession()
	if err != nil {
		return err
	}
	defer sess.Close()
	sess.Stdin = strings.NewReader(key + "\n")
	const script = `sh -c 'umask 077; mkdir -p ~/.ssh && f=~/.ssh/authorized_keys && touch $f && read -r k &&
{ grep -qxF "$k" $f || { [ -s $f ] && [ -n "$(tail -c 1 $f)" ] && echo >>$f; printf "%s\n" "$k" >>$f; }; }'`
	if out, err := sess.CombinedOutput(script); err != nil {
		if msg := strings.TrimSpace(string(out)); msg != "" {
			return fmt.Errorf("%w: %s", err, msg)
		}
		return err
	}
	return nil
}

// keepalive asks the server for a reply now and then: a connection that
// died quietly (Wi-Fi gone to sleep, a laptop lid closed) gets closed, so
// it can be noticed and made again, instead of hanging there.
func (s *sshSession) keepalive() {
	t := time.NewTicker(15 * time.Second)
	defer t.Stop()
	for {
		select {
		case <-s.done:
			return
		case <-t.C:
		}
		reply := make(chan error, 1)
		go func() {
			_, _, err := s.client.SendRequest("keepalive@openssh.com", true, nil)
			reply <- err
		}()
		select {
		case <-s.done:
			return
		case err := <-reply:
			if err == nil {
				continue
			}
		case <-time.After(10 * time.Second):
		}
		s.lost.Store(true)
		s.client.Close()
		return
	}
}

func (s *sshSession) Read(p []byte) (int, error)  { return s.stdout.Read(p) }
func (s *sshSession) Write(p []byte) (int, error) { return s.stdin.Write(p) }
func (s *sshSession) resize(cols, rows int)       { s.session.WindowChange(rows, cols) }

func (s *sshSession) close() {
	select {
	case <-s.done:
	default:
		close(s.done)
	}
	s.session.Close()
	s.client.Close()
}

// ended says how the session ended, once its output did: a remote that
// exits sends its exit status, a connection that drops can't.
func (s *sshSession) ended() (lost bool, how string) {
	err := s.session.Wait()
	var exit *ssh.ExitError
	var missing *ssh.ExitMissingError
	switch {
	case s.lost.Load():
		return true, "the server stopped answering"
	case err == nil:
		return false, ""
	case errors.As(err, &exit):
		return false, fmt.Sprintf("exit status %d", exit.ExitStatus())
	case errors.As(err, &missing):
		return true, "connection lost"
	}
	return true, err.Error()
}
