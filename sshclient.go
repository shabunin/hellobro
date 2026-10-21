package main

// SSH for the Terminal tab: hosts from ssh.conf, our own key (made on
// first use), trust-on-first-use host keys in known_hosts, and password /
// keyboard-interactive prompts answered from the Kobo's keyboard.

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
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

type sshHost struct {
	name, user, addr string // addr is host:port
}

func (h sshHost) String() string { return h.user + "@" + h.addr }

// appFile is a file next to the binary.
func appFile(name string) string {
	if exe, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exe), name)
	}
	return name
}

const sshConfExample = `# hellobro ssh hosts, one per line:  name  user@host[:port]
# hellobro-server  bro@192.168.1.10:2222
`

// loadSSHHosts reads ssh.conf, creating an example if there's none.
func loadSSHHosts() []sshHost {
	path := appFile("ssh.conf")
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
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 || strings.HasPrefix(fields[0], "#") {
			continue
		}
		if h, err := parseSSHTarget(fields[1]); err == nil {
			h.name = fields[0]
			hosts = append(hosts, h)
		}
	}
	return hosts
}

// parseSSHTarget reads user@host[:port] (user defaults to root, port to
// 22), or the same as an ssh command line, since that's what fingers type:
// "ssh -p 2222 bro@host", "ssh -l bro host", "ssh://bro@host:2222".
func parseSSHTarget(s string) (sshHost, error) {
	fields := strings.Fields(s)
	if len(fields) > 0 && fields[0] == "ssh" {
		fields = fields[1:]
	}
	var target, user, port string
	for i := 0; i < len(fields); i++ {
		f := fields[i]
		switch {
		case (f == "-p" || f == "-l") && i+1 < len(fields):
			i++
			if f == "-p" {
				port = fields[i]
			} else {
				user = fields[i]
			}
		case strings.HasPrefix(f, "-p") && len(f) > 2:
			port = f[2:]
		case strings.HasPrefix(f, "-l") && len(f) > 2:
			user = f[2:]
		case strings.HasPrefix(f, "-"):
			return sshHost{}, fmt.Errorf("only -p PORT and -l USER are understood, not %s", f)
		case target == "":
			target = strings.TrimSuffix(strings.TrimPrefix(f, "ssh://"), "/")
		default:
			return sshHost{}, fmt.Errorf("want user@host[:port], got %q", s)
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
	return sshHost{name: target, user: user, addr: net.JoinHostPort(host, port)}, nil
}

// clientKey loads id_ed25519, or makes one (plus id_ed25519.pub, to copy
// into a server's authorized_keys).
func clientKey() (ssh.Signer, error) {
	path := appFile("id_ed25519")
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
	path := appFile("known_hosts")
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

// sshSession is a connected shell with a PTY.
type sshSession struct {
	client  *ssh.Client
	session *ssh.Session
	stdin   io.WriteCloser
	stdout  io.Reader
}

// prompter asks the user something; echo=false for passwords.
type prompter func(question string, echo bool) (string, error)

func dialSSH(h sshHost, cols, rows int, ask prompter, note func(string)) (*sshSession, error) {
	signer, err := clientKey()
	if err != nil {
		return nil, fmt.Errorf("client key: %w", err)
	}
	cfg := &ssh.ClientConfig{
		User: h.user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
			ssh.KeyboardInteractive(func(name, instruction string, questions []string, echos []bool) ([]string, error) {
				if instruction != "" {
					note(instruction)
				}
				answers := make([]string, len(questions))
				for i, q := range questions {
					a, err := ask(q, echos[i])
					if err != nil {
						return nil, err
					}
					answers[i] = a
				}
				return answers, nil
			}),
			ssh.RetryableAuthMethod(ssh.PasswordCallback(func() (string, error) {
				return ask(fmt.Sprintf("%s's password: ", h), false)
			}), 3),
		},
		HostKeyCallback: tofuHostKeys(note),
		Timeout:         10 * time.Second,
	}
	client, err := ssh.Dial("tcp", h.addr, cfg)
	if err != nil {
		return nil, err
	}
	sess, err := client.NewSession()
	if err != nil {
		client.Close()
		return nil, err
	}
	fail := func(err error) (*sshSession, error) {
		sess.Close()
		client.Close()
		return nil, err
	}
	modes := ssh.TerminalModes{ssh.ECHO: 1, ssh.TTY_OP_ISPEED: 115200, ssh.TTY_OP_OSPEED: 115200}
	if err := sess.RequestPty("xterm-256color", rows, cols, modes); err != nil {
		return fail(fmt.Errorf("pty: %w", err))
	}
	sess.Setenv("LANG", "C.UTF-8") // most servers refuse it; harmless
	s := &sshSession{client: client, session: sess}
	if s.stdin, err = sess.StdinPipe(); err != nil {
		return fail(err)
	}
	if s.stdout, err = sess.StdoutPipe(); err != nil {
		return fail(err)
	}
	sess.Stderr = nil // with a PTY, stderr comes through stdout
	if err := sess.Shell(); err != nil {
		return fail(fmt.Errorf("shell: %w", err))
	}
	return s, nil
}

func (s *sshSession) resize(cols, rows int) { s.session.WindowChange(rows, cols) }

func (s *sshSession) close() {
	s.session.Close()
	s.client.Close()
}
