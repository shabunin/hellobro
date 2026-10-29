// Package share is the Kobo's door for a computer: an SSH server with
// SFTP (so a file manager, sftp, scp or sshfs reach the Kobo's storage),
// a shell, and commands, plus an mDNS responder that advertises it.
//
// Only paired keys get in. Pairing is the Bluetooth way: while it's open,
// a client whose key isn't known yet proves it holds the key, then types
// the code shown on the Kobo, and that key is added to authorized_keys.
package share

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/subtle"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// Port is where the server listens, unless Config says otherwise.
const Port = 2022

// Shell is a program on a pseudo-terminal: what's read is its output.
type Shell interface {
	io.ReadWriteCloser
	Resize(cols, rows int)
	Wait() error
}

type Config struct {
	Addr     string // ":2022" if empty
	StateDir string // the host key and authorized_keys live here
	Root     string // where SFTP sessions start, "/" if empty
	// StartShell runs cmd (a login shell if empty) on a PTY.
	StartShell func(cmd string, cols, rows int) (Shell, error)
	// Builtin runs a command of the app's own, if cmd is one: its output
	// goes to out; ok is false for any other command.
	Builtin func(cmd string, out io.Writer) (status int, ok bool)
	// Notify hears what happens (connections, pairing), from the server's
	// goroutines.
	Notify func(Event)
	// AskKeys asks on the screen whether who (at addr) may type here:
	// another device logging in as KeysUser with a key not paired yet.
	// Allowed, its key is saved to do only that. nil: nobody's asked.
	AskKeys func(who, addr string) bool
}

// KeysUser is the user another device logs in as to type here, and the
// command a key paired for that runs, whatever it asks for.
const KeysUser = "hellobro-keys"

// Event is something that happened, for the UI.
type Event struct {
	Kind string // "connected", "closed", "paired", "pairing over", "refused"
	Conn ConnInfo
	Note string
}

// ConnInfo is a connection, as the UI shows it.
type ConnInfo struct {
	ID        int
	Addr      string
	User      string
	Since     time.Time
	What      string // "sftp", "shell", "exec: …", or "" until a session starts
	In, Out   int64  // bytes through its sessions: in from the computer, out to it
	WroteSFTP bool   // an SFTP session sent data in
}

type Server struct {
	cfg   Config
	ln    net.Listener
	ssh   *ssh.ServerConfig
	host  ssh.Signer
	mu    sync.Mutex
	conns map[int]*conn
	next  int

	pairCode  string
	pairUntil time.Time
	pairTries int
	asking    sync.Mutex // one AskKeys at a time
}

type conn struct {
	info    ConnInfo
	in, out atomic.Int64
	c       *ssh.ServerConn
}

const (
	hostKeyFile    = "ssh_host_ed25519_key"
	authorizedFile = "authorized_keys"
	maxConns       = 8
	pairTriesMax   = 5
)

// Start listens and serves, until Close.
func Start(cfg Config) (*Server, error) {
	if cfg.Addr == "" {
		cfg.Addr = fmt.Sprintf(":%d", Port)
	}
	if cfg.Root == "" {
		cfg.Root = "/"
	}
	if cfg.Notify == nil {
		cfg.Notify = func(Event) {}
	}
	host, err := hostKey(filepath.Join(cfg.StateDir, hostKeyFile))
	if err != nil {
		return nil, fmt.Errorf("host key: %w", err)
	}
	s := &Server{cfg: cfg, host: host, conns: map[int]*conn{}}
	s.ssh = &ssh.ServerConfig{
		PublicKeyCallback:         s.publicKey,
		VerifiedPublicKeyCallback: s.verifiedKey,
		ServerVersion:             "SSH-2.0-hellobro",
		BannerCallback: func(ssh.ConnMetadata) string {
			if s.Pairing() != "" {
				return "hellobro on the Kobo: pairing is open. Type the code the Kobo shows.\n"
			}
			return ""
		},
	}
	s.ssh.AddHostKey(host)
	ln, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return nil, err
	}
	s.ln = ln
	log.Printf("share: listening on %s, host key %s", ln.Addr(), ssh.FingerprintSHA256(host.PublicKey()))
	go s.serve()
	return s, nil
}

// Addr is where it listens.
func (s *Server) Addr() net.Addr { return s.ln.Addr() }

// HostKey is the server's key's fingerprint, for checking on the computer.
func (s *Server) HostKey() string { return ssh.FingerprintSHA256(s.host.PublicKey()) }

// Close stops listening and hangs up on everyone.
func (s *Server) Close() {
	s.ln.Close()
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.conns {
		c.c.Close()
	}
}

// Conns are the connections now.
func (s *Server) Conns() []ConnInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []ConnInfo
	for _, c := range s.conns {
		out = append(out, c.snapshot())
	}
	sortConns(out)
	return out
}

func sortConns(cs []ConnInfo) {
	for i := 1; i < len(cs); i++ {
		for j := i; j > 0 && cs[j].ID < cs[j-1].ID; j-- {
			cs[j], cs[j-1] = cs[j-1], cs[j]
		}
	}
}

func (c *conn) snapshot() ConnInfo {
	i := c.info
	i.In, i.Out = c.in.Load(), c.out.Load()
	return i
}

// Pair opens pairing with a new code until d is up, and returns the code.
func (s *Server) Pair(d time.Duration) string {
	b := make([]byte, 4)
	rand.Read(b)
	code := fmt.Sprintf("%06d", (uint32(b[0])<<24|uint32(b[1])<<16|uint32(b[2])<<8|uint32(b[3]))%1000000)
	return s.PairWith(code, d)
}

// PairWith opens pairing with the code given: for tests, which have to
// know it.
func (s *Server) PairWith(code string, d time.Duration) string {
	s.mu.Lock()
	s.pairCode, s.pairUntil, s.pairTries = code, time.Now().Add(d), 0
	s.mu.Unlock()
	log.Printf("share: pairing open for %v", d)
	return code
}

// StopPairing closes pairing.
func (s *Server) StopPairing() {
	s.mu.Lock()
	s.pairCode = ""
	s.mu.Unlock()
}

// Pairing is the pairing code while pairing is open, else "".
func (s *Server) Pairing() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pairCode != "" && time.Now().After(s.pairUntil) {
		s.pairCode = ""
		go s.cfg.Notify(Event{Kind: "pairing over", Note: "the time ran out"})
	}
	return s.pairCode
}

// --- authentication ---

func (s *Server) publicKey(meta ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
	if k, _ := s.authorized(key); k != nil {
		p := &ssh.Permissions{Extensions: map[string]string{"key": ssh.FingerprintSHA256(key)}}
		if cmd, ok := k.Command(); ok {
			p.CriticalOptions = map[string]string{"force-command": cmd}
		}
		return p, nil
	}
	if meta.User() == KeysUser && s.cfg.AskKeys != nil {
		return &ssh.Permissions{Extensions: map[string]string{"askkeys": "1"}}, nil
	}
	if s.Pairing() != "" {
		return &ssh.Permissions{Extensions: map[string]string{"pair": "1"}}, nil
	}
	return nil, errors.New("not paired")
}

// verifiedKey: the client holds key. A paired one is in; an unknown one,
// while pairing, goes on to the code.
func (s *Server) verifiedKey(meta ssh.ConnMetadata, key ssh.PublicKey, perms *ssh.Permissions, _ string) (*ssh.Permissions, error) {
	if perms.Extensions["askkeys"] != "" {
		return s.askKeys(meta, key)
	}
	if perms.Extensions["pair"] == "" {
		return perms, nil
	}
	code := func(answer string) (*ssh.Permissions, error) {
		if err := s.checkCode(answer); err != nil {
			return nil, err
		}
		comment := fmt.Sprintf("paired-%s-%s@%s", time.Now().Format("2006-01-02"), meta.User(), hostOf(meta.RemoteAddr()))
		if err := s.addKey(key, comment); err != nil {
			return nil, err
		}
		log.Printf("share: paired %s (%s)", ssh.FingerprintSHA256(key), comment)
		s.cfg.Notify(Event{Kind: "paired", Note: comment})
		return &ssh.Permissions{Extensions: map[string]string{"key": ssh.FingerprintSHA256(key)}}, nil
	}
	return nil, &ssh.PartialSuccessError{Next: ssh.ServerAuthCallbacks{
		KeyboardInteractiveCallback: func(_ ssh.ConnMetadata, ask ssh.KeyboardInteractiveChallenge) (*ssh.Permissions, error) {
			ans, err := ask("Pairing with the Kobo", "", []string{"The code the Kobo shows: "}, []bool{true})
			if err != nil || len(ans) != 1 {
				return nil, errors.New("no code")
			}
			return code(ans[0])
		},
		PasswordCallback: func(_ ssh.ConnMetadata, pw []byte) (*ssh.Permissions, error) { return code(string(pw)) },
	}}
}

// askKeys has the screen ask whether the device may type here, and saves
// its key for only that if so.
func (s *Server) askKeys(meta ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
	if !s.asking.TryLock() {
		return nil, errors.New("asking about another already")
	}
	defer s.asking.Unlock()
	who, addr := PeerName(string(meta.ClientVersion())), hostOf(meta.RemoteAddr())
	if !s.cfg.AskKeys(who, addr) {
		return nil, errors.New("not allowed to type here")
	}
	comment := fmt.Sprintf("keys-%s-%s@%s", time.Now().Format("2006-01-02"), strings.ReplaceAll(who, " ", "_"), addr)
	if err := s.addKey(key, comment, `command="`+KeysUser+`"`); err != nil {
		return nil, err
	}
	log.Printf("share: %s (%s) may type here: %s", who, addr, ssh.FingerprintSHA256(key))
	s.cfg.Notify(Event{Kind: "paired", Note: comment})
	return &ssh.Permissions{CriticalOptions: map[string]string{"force-command": KeysUser},
		Extensions: map[string]string{"key": ssh.FingerprintSHA256(key)}}, nil
}

// PeerName is who an SSH client's version says it is: another hellobro
// says its device ("SSH-2.0-hellobro Kobo Elipsa"), anything else its
// software. Only letters, digits and a few marks: it's shown and kept.
func PeerName(version string) string {
	v := strings.TrimPrefix(version, "SSH-2.0-")
	if rest, ok := strings.CutPrefix(v, "hellobro "); ok {
		v = rest
	}
	var b strings.Builder
	for _, r := range v {
		switch {
		case b.Len() >= 40:
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', strings.ContainsRune(" ._-", r):
			b.WriteRune(r)
		}
	}
	if n := strings.TrimSpace(b.String()); n != "" {
		return n
	}
	return "a computer"
}

func (s *Server) checkCode(answer string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pairCode == "" || time.Now().After(s.pairUntil) {
		return errors.New("pairing is closed")
	}
	if subtle.ConstantTimeCompare([]byte(strings.TrimSpace(answer)), []byte(s.pairCode)) == 1 {
		s.pairCode = "" // one computer per code
		return nil
	}
	if s.pairTries++; s.pairTries >= pairTriesMax {
		s.pairCode = ""
		go s.cfg.Notify(Event{Kind: "pairing over", Note: "too many wrong codes"})
	}
	return errors.New("wrong code")
}

// authorized is key's line in authorized_keys, nil if it's not there.
func (s *Server) authorized(key ssh.PublicKey) (*AuthorizedKey, error) {
	keys, err := s.Keys()
	for _, k := range keys {
		if subtle.ConstantTimeCompare(k.Key.Marshal(), key.Marshal()) == 1 {
			return &k, nil
		}
	}
	return nil, err
}

// AuthorizedKey is a paired computer's key.
type AuthorizedKey struct {
	Key         ssh.PublicKey
	Comment     string
	Fingerprint string
	Options     []string // OpenSSH's: command="…" is the one heeded
}

// Command is the command the key runs whatever it asks for (OpenSSH's
// command="…"): a key that may only do that, as hellobro-keys.
func (k AuthorizedKey) Command() (string, bool) {
	for _, o := range k.Options {
		if v, ok := strings.CutPrefix(o, "command="); ok {
			return strings.ReplaceAll(strings.Trim(v, `"`), `\"`, `"`), true
		}
	}
	return "", false
}

// line is k as authorized_keys has it.
func (k AuthorizedKey) line() string {
	l := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(k.Key))) + " " + k.Comment
	if len(k.Options) > 0 {
		l = strings.Join(k.Options, ",") + " " + l
	}
	return l
}

// Keys are the paired keys.
func (s *Server) Keys() ([]AuthorizedKey, error) {
	b, err := os.ReadFile(filepath.Join(s.cfg.StateDir, authorizedFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	var out []AuthorizedKey
	for len(b) > 0 {
		k, comment, opts, rest, err := ssh.ParseAuthorizedKey(b)
		if err != nil {
			break
		}
		out = append(out, AuthorizedKey{k, comment, ssh.FingerprintSHA256(k), opts})
		b = rest
	}
	return out, err
}

func (s *Server) addKey(key ssh.PublicKey, comment string, opts ...string) error {
	f, err := os.OpenFile(filepath.Join(s.cfg.StateDir, authorizedFile), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	line := AuthorizedKey{Key: key, Comment: comment, Options: opts}.line() + "\n"
	if _, err := f.WriteString(line); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// Forget unpairs the key with fingerprint fp.
func (s *Server) Forget(fp string) error {
	keys, err := s.Keys()
	if err != nil {
		return err
	}
	var b strings.Builder
	for _, k := range keys {
		if k.Fingerprint != fp {
			b.WriteString(k.line() + "\n")
		}
	}
	return os.WriteFile(filepath.Join(s.cfg.StateDir, authorizedFile), []byte(b.String()), 0o600)
}

// hostKey loads the server's key, or makes one.
func hostKey(path string) (ssh.Signer, error) {
	if b, err := os.ReadFile(path); err == nil {
		return ssh.ParsePrivateKey(b)
	}
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	block, err := ssh.MarshalPrivateKey(priv, "hellobro host key")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0o600); err != nil {
		return nil, err
	}
	return ssh.NewSignerFromKey(priv)
}

func hostOf(a net.Addr) string {
	h, _, err := net.SplitHostPort(a.String())
	if err != nil {
		return a.String()
	}
	return h
}

// --- connections ---

func (s *Server) serve() {
	for {
		nc, err := s.ln.Accept()
		if err != nil {
			if !errors.Is(err, net.ErrClosed) {
				log.Printf("share: %v", err)
			}
			return
		}
		go s.handle(nc)
	}
}

// ServeConn serves a connection that came another way (the relay's),
// till it's over.
func (s *Server) ServeConn(nc net.Conn) {
	defer nc.Close()
	s.handle(nc)
}

func (s *Server) handle(nc net.Conn) {
	s.mu.Lock()
	full := len(s.conns) >= maxConns
	s.mu.Unlock()
	if full {
		nc.Close()
		return
	}
	nc.SetDeadline(time.Now().Add(2 * time.Minute)) // to get through the handshake: typing a code takes a while
	sc, chans, reqs, err := ssh.NewServerConn(nc, s.ssh)
	if err != nil {
		log.Printf("share: %s: %v", nc.RemoteAddr(), err)
		s.cfg.Notify(Event{Kind: "refused", Note: fmt.Sprintf("%s: %v", hostOf(nc.RemoteAddr()), err)})
		nc.Close()
		return
	}
	nc.SetDeadline(time.Time{})
	s.mu.Lock()
	s.next++
	c := &conn{c: sc, info: ConnInfo{ID: s.next, Addr: hostOf(sc.RemoteAddr()), User: sc.User(), Since: time.Now()}}
	s.conns[c.info.ID] = c
	s.mu.Unlock()
	log.Printf("share: %s@%s connected (%s)", sc.User(), c.info.Addr, sc.Permissions.Extensions["key"])
	s.cfg.Notify(Event{Kind: "connected", Conn: c.snapshot()})

	go keepalive(sc)
	go ssh.DiscardRequests(reqs) // no port forwarding
	var wg sync.WaitGroup
	for nch := range chans {
		if nch.ChannelType() != "session" {
			nch.Reject(ssh.UnknownChannelType, "sessions only")
			continue
		}
		ch, creqs, err := nch.Accept()
		if err != nil {
			continue
		}
		wg.Go(func() { s.session(c, ch, creqs) })
	}
	wg.Wait()
	s.mu.Lock()
	delete(s.conns, c.info.ID)
	s.mu.Unlock()
	info := c.snapshot()
	log.Printf("share: %s@%s gone, %d bytes in, %d out", info.User, info.Addr, info.In, info.Out)
	s.cfg.Notify(Event{Kind: "closed", Conn: info})
}

// keepalive hangs up on a computer that went away without saying so.
func keepalive(sc *ssh.ServerConn) {
	for range time.Tick(30 * time.Second) {
		done := make(chan error, 1)
		go func() {
			_, _, err := sc.SendRequest("keepalive@openssh.com", true, nil)
			done <- err
		}()
		select {
		case err := <-done:
			if err == nil { // a "no" is an answer too
				continue
			}
		case <-time.After(30 * time.Second):
		}
		sc.Close()
		return
	}
}

// counted counts what goes through a session, both ways.
type counted struct {
	ssh.Channel
	c *conn
}

func (cc counted) Read(p []byte) (int, error) {
	n, err := cc.Channel.Read(p)
	cc.c.in.Add(int64(n))
	return n, err
}

func (cc counted) Write(p []byte) (int, error) {
	n, err := cc.Channel.Write(p)
	cc.c.out.Add(int64(n))
	return n, err
}

type noClose struct{ io.ReadWriter }

func (noClose) Close() error { return nil }

type ptyReq struct {
	term       string
	cols, rows int
}

func (s *Server) session(c *conn, ch ssh.Channel, reqs <-chan *ssh.Request) {
	defer ch.Close()
	forced, isForced := c.c.Permissions.CriticalOptions["force-command"] // that, and nothing else
	cc := counted{ch, c}
	var pty *ptyReq
	var shell Shell
	for req := range reqs {
		switch req.Type {
		case "pty-req":
			var p struct {
				Term             string
				Cols, Rows, W, H uint32
				Modes            string
			}
			if err := ssh.Unmarshal(req.Payload, &p); err != nil {
				req.Reply(false, nil)
				continue
			}
			pty = &ptyReq{p.Term, int(p.Cols), int(p.Rows)}
			req.Reply(true, nil)
		case "window-change":
			var p struct{ Cols, Rows, W, H uint32 }
			if ssh.Unmarshal(req.Payload, &p) == nil && shell != nil {
				shell.Resize(int(p.Cols), int(p.Rows))
			}
		case "env":
			req.Reply(true, nil)
		case "subsystem":
			var p struct{ Name string }
			ssh.Unmarshal(req.Payload, &p)
			if p.Name != "sftp" || isForced {
				req.Reply(false, nil)
				continue
			}
			req.Reply(true, nil)
			s.setWhat(c, "sftp")
			before := c.in.Load()
			// noClose: the server closing it would beat the exit status.
			srv, err := sftp.NewServer(noClose{cc}, sftp.WithServerWorkingDirectory(s.cfg.Root))
			if err == nil {
				err = srv.Serve()
				srv.Close()
			}
			if c.in.Load()-before > 64<<10 {
				s.mu.Lock()
				c.info.WroteSFTP = true
				s.mu.Unlock()
			}
			status := 0
			if err != nil && !errors.Is(err, io.EOF) {
				log.Printf("share: sftp: %v", err)
				status = 1
			}
			exit(ch, status) // scp goes by it
			return
		case "shell", "exec":
			var cmd string
			if req.Type == "exec" {
				var p struct{ Cmd string }
				ssh.Unmarshal(req.Payload, &p)
				cmd = p.Cmd
			}
			if isForced {
				cmd = forced
			}
			if cmd != "" && s.cfg.Builtin != nil {
				if status, ok := s.cfg.Builtin(strings.TrimSpace(cmd), cc); ok {
					req.Reply(true, nil)
					s.setWhat(c, "exec: "+cmd)
					exit(ch, status)
					return
				}
			}
			s.setWhat(c, map[bool]string{true: "shell", false: "exec: " + cmd}[cmd == ""])
			if pty != nil && s.cfg.StartShell != nil {
				sh, err := s.cfg.StartShell(cmd, pty.cols, pty.rows)
				if err != nil {
					req.Reply(false, nil)
					fmt.Fprintf(ch.Stderr(), "hellobro: %v\r\n", err)
					return
				}
				shell = sh
				req.Reply(true, nil)
				go func() {
					io.Copy(sh, cc) // the keys
				}()
				go func() {
					io.Copy(cc, sh)
					ch.CloseWrite()
				}()
				go func() {
					err := sh.Wait()
					time.Sleep(100 * time.Millisecond) // the last output
					exit(ch, exitStatus(err))
					sh.Close()
					ch.Close()
				}()
				continue // window changes yet
			}
			if isForced && cmd == KeysUser { // a keyboard wants a terminal: no shell runs it without
				req.Reply(false, nil)
				return
			}
			req.Reply(true, nil)
			exit(ch, s.run(cmd, cc, ch.Stderr()))
			return
		default:
			if req.WantReply {
				req.Reply(false, nil)
			}
		}
	}
	if shell != nil {
		shell.Close()
	}
}

func (s *Server) setWhat(c *conn, what string) {
	s.mu.Lock()
	c.info.What = what
	s.mu.Unlock()
}

// run runs cmd without a terminal: what scp's old protocol and
// "ssh kobo command" want.
func (s *Server) run(cmd string, rw io.ReadWriter, stderr io.Writer) int {
	if cmd == "" {
		cmd = "exec /bin/sh -l"
	}
	c := exec.Command("/bin/sh", "-c", cmd)
	c.Stdout, c.Stderr = rw, stderr
	c.Dir = s.cfg.Root
	c.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	// Through a pipe Wait closes, not c.Stdin = rw: Wait would wait for the
	// computer to close its end, which it does only once we exit.
	in, err := c.StdinPipe()
	if err != nil {
		return 255
	}
	if err := c.Start(); err != nil {
		fmt.Fprintf(stderr, "hellobro: %v\n", err)
		return 127
	}
	go func() {
		io.Copy(in, rw)
		in.Close()
	}()
	return exitStatus(c.Wait())
}

func exitStatus(err error) int {
	var ee *exec.ExitError
	switch {
	case err == nil:
		return 0
	case errors.As(err, &ee):
		return ee.ExitCode()
	}
	return 255
}

func exit(ch ssh.Channel, status int) {
	ch.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{uint32(status)}))
}
