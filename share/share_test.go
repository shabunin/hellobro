package share

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/dns/dnsmessage"
)

func newKey(t *testing.T) ssh.Signer {
	_, priv, _ := ed25519.GenerateKey(rand.Reader)
	s, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func dial(s *Server, key ssh.Signer, code string) (*ssh.Client, error) {
	auth := []ssh.AuthMethod{ssh.PublicKeys(key)}
	if code != "" {
		auth = append(auth, ssh.KeyboardInteractive(func(_, _ string, qs []string, _ []bool) ([]string, error) {
			return []string{code}, nil
		}))
	}
	return ssh.Dial("tcp", s.Addr().String(), &ssh.ClientConfig{User: "bro", Auth: auth, HostKeyCallback: ssh.InsecureIgnoreHostKey()})
}

func TestPairAndShare(t *testing.T) {
	state, root := t.TempDir(), t.TempDir()
	events := make(chan Event, 32)
	s, err := Start(Config{Addr: "127.0.0.1:0", StateDir: state, Root: root,
		Builtin: func(cmd string, out io.Writer) (int, bool) {
			if cmd != "hellobro-version" {
				return 0, false
			}
			fmt.Fprintln(out, "v1")
			return 0, true
		},
		Notify: func(e Event) { events <- e }})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	key := newKey(t)

	if _, err := dial(s, key, ""); err == nil {
		t.Fatal("got in unpaired")
	}
	code := s.Pair(60e9)
	if _, err := dial(s, key, "000000x"); err == nil {
		t.Fatal("got in with a wrong code")
	}
	c, err := dial(s, key, code)
	if err != nil {
		t.Fatalf("pairing: %v", err)
	}
	c.Close()
	if s.Pairing() != "" {
		t.Error("pairing still open after a computer paired")
	}
	keys, _ := s.Keys()
	if len(keys) != 1 || !strings.HasPrefix(keys[0].Comment, "paired-") {
		t.Fatalf("keys after pairing: %+v", keys)
	}

	// Paired: the key alone gets in, and SFTP works.
	c, err = dial(s, key, "")
	if err != nil {
		t.Fatalf("paired key: %v", err)
	}
	fc, err := sftp.NewClient(c)
	if err != nil {
		t.Fatal(err)
	}
	data := bytes.Repeat([]byte("music "), 50000)
	f, err := fc.Create("song.mp3") // relative: in Root
	if err != nil {
		t.Fatal(err)
	}
	f.Write(data)
	f.Close()
	fc.Close()
	got, _ := os.ReadFile(filepath.Join(root, "song.mp3"))
	if !bytes.Equal(got, data) {
		t.Fatalf("uploaded %d bytes, %d arrived", len(data), len(got))
	}

	// Commands: ours, and the shell's.
	for cmd, want := range map[string]string{"hellobro-version": "v1\n", "echo hi; exit 3": "hi\n"} {
		sess, _ := c.NewSession()
		out, err := sess.Output(cmd)
		if string(out) != want {
			t.Errorf("%q: %q, want %q (%v)", cmd, out, want, err)
		}
	}
	c.Close()

	var closed *Event
	for closed == nil {
		if e := <-events; e.Kind == "closed" && e.Conn.WroteSFTP {
			closed = &e
		}
	}
	if closed.Conn.In < int64(len(data)) {
		t.Errorf("counted %d bytes in, sent %d", closed.Conn.In, len(data))
	}

	// Forgotten: out.
	if err := s.Forget(keys[0].Fingerprint); err != nil {
		t.Fatal(err)
	}
	if _, err := dial(s, key, ""); err == nil {
		t.Fatal("got in after being forgotten")
	}
}

func TestPairingClosesAfterWrongCodes(t *testing.T) {
	s, err := Start(Config{Addr: "127.0.0.1:0", StateDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	code := s.Pair(60e9)
	for range pairTriesMax {
		dial(s, newKey(t), "nope")
	}
	if _, err := dial(s, newKey(t), code); err == nil {
		t.Fatal("paired after too many wrong codes")
	}
}

func TestMDNSAnswers(t *testing.T) {
	z := zone{host: "kobo.local.", instance: "Kobo Elipsa", ip: [4]byte{192, 168, 2, 31}, port: 2022, txt: []string{"u=root"}}
	q := func(name string, typ dnsmessage.Type) []dnsmessage.Question {
		return []dnsmessage.Question{{Name: dnsmessage.MustNewName(name), Type: typ, Class: dnsmessage.ClassINET}}
	}
	ans, _ := z.answer(q("KOBO.local.", dnsmessage.TypeA))
	if len(ans) != 1 || ans[0].Body.(*dnsmessage.AResource).A != z.ip {
		t.Fatalf("A: %v", ans)
	}
	ans, extra := z.answer(q("_sftp-ssh._tcp.local.", dnsmessage.TypePTR))
	if len(ans) != 1 || len(extra) != 3 {
		t.Fatalf("PTR: %d answers, %d extra", len(ans), len(extra))
	}
	if ptr := ans[0].Body.(*dnsmessage.PTRResource).PTR.String(); ptr != "Kobo Elipsa._sftp-ssh._tcp.local." {
		t.Errorf("PTR to %q", ptr)
	}
	if ans, _ := z.answer(q("other.local.", dnsmessage.TypeA)); len(ans) != 0 {
		t.Errorf("answered for another name: %v", ans)
	}
	m := dnsmessage.Message{Header: dnsmessage.Header{Response: true}, Answers: z.records(func(t uint32) uint32 { return t })}
	if _, err := m.Pack(); err != nil {
		t.Fatal(err)
	}
}

// A key with command="…" runs that and nothing else: not what it asks
// for, not SFTP; and forgetting another key keeps its option.
func TestForcedCommand(t *testing.T) {
	state := t.TempDir()
	s, err := Start(Config{Addr: "127.0.0.1:0", StateDir: state, Root: t.TempDir(),
		Builtin: func(cmd string, out io.Writer) (int, bool) {
			fmt.Fprintf(out, "ran %s\n", cmd)
			return 0, true
		},
		Notify: func(Event) {}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	keys, other := newKey(t), newKey(t)
	line := func(k ssh.Signer, comment string) string {
		return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(k.PublicKey()))) + " " + comment + "\n"
	}
	os.WriteFile(filepath.Join(state, authorizedFile), []byte(`command="hellobro-keys" `+line(keys, "kobo")+line(other, "pc")), 0o600)
	if err := s.Forget(ssh.FingerprintSHA256(other.PublicKey())); err != nil {
		t.Fatal(err)
	}
	c, err := dial(s, keys, "")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	for _, cmd := range []string{"rm -rf /", ""} {
		sess, _ := c.NewSession()
		var out []byte
		if cmd == "" {
			var b bytes.Buffer
			sess.Stdout = &b
			sess.Shell()
			sess.Wait()
			out = b.Bytes()
		} else {
			out, _ = sess.Output(cmd)
		}
		if string(out) != "ran hellobro-keys\n" {
			t.Errorf("%q: %q", cmd, out)
		}
	}
	if fc, err := sftp.NewClient(c); err == nil {
		fc.Close()
		t.Error("sftp for a key that runs one command")
	}
}

// Browse makes peers of what a responder answers, and leaves itself out.
func TestBrowseAnswers(t *testing.T) {
	z := zone{host: "reader.local.", instance: "Another Reader", ip: [4]byte{192, 168, 1, 30}, port: 2023}
	ans, extra := z.answer([]dnsmessage.Question{{Name: dnsmessage.MustNewName(HelloType), Type: dnsmessage.TypePTR, Class: dnsmessage.ClassINET}})
	b, err := (&dnsmessage.Message{Header: dnsmessage.Header{Response: true}, Answers: ans, Additionals: extra}).Pack()
	if err != nil {
		t.Fatal(err)
	}
	var h browsed
	h.take(b)
	want := []Peer{{"Another Reader", "192.168.1.30:2023"}}
	if got := h.peers(net.IPv4(192, 168, 1, 20)); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("peers %v, want %v", got, want)
	}
	if got := h.peers(net.IPv4(192, 168, 1, 30)); len(got) != 0 {
		t.Errorf("itself among the peers: %v", got)
	}
}

// Another device asking to type: the screen says no, it's out; yes, its
// key is kept, for typing only.
func TestAskKeys(t *testing.T) {
	state := t.TempDir()
	allow := make(chan bool, 1)
	var asked string
	s, err := Start(Config{Addr: "127.0.0.1:0", StateDir: state, Root: t.TempDir(),
		StartShell: func(cmd string, cols, rows int) (Shell, error) { return nil, fmt.Errorf("ran %q", cmd) },
		AskKeys: func(who, addr string) bool {
			asked = who + " at " + addr
			return <-allow
		}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	key := newKey(t)
	dialAs := func(user string) (*ssh.Client, error) {
		return ssh.Dial("tcp", s.Addr().String(), &ssh.ClientConfig{User: user, Auth: []ssh.AuthMethod{ssh.PublicKeys(key)},
			ClientVersion: "SSH-2.0-hellobro Kobo Elipsa", HostKeyCallback: ssh.InsecureIgnoreHostKey()})
	}
	if _, err := dialAs("root"); err == nil {
		t.Fatal("in as root, not paired")
	}
	allow <- false
	if _, err := dialAs(KeysUser); err == nil {
		t.Fatal("in when the screen said no")
	}
	if asked != "Kobo Elipsa at 127.0.0.1" {
		t.Errorf("asked about %q", asked)
	}
	allow <- true
	c, err := dialAs(KeysUser)
	if err != nil {
		t.Fatalf("allowed: %v", err)
	}
	c.Close()
	keys, _ := s.Keys()
	if cmd, ok := keys[0].Command(); len(keys) != 1 || !ok || cmd != KeysUser {
		t.Fatalf("kept %+v", keys)
	}
	if _, err := dialAs("root"); err != nil { // known now, whatever the user: and it may only type
		t.Fatal(err)
	}
}

func TestPeerName(t *testing.T) {
	for v, want := range map[string]string{
		"SSH-2.0-hellobro Kobo Elipsa": "Kobo Elipsa",
		"SSH-2.0-OpenSSH_9.6":          "OpenSSH_9.6",
		"SSH-2.0-hellobro \x1b[2J!!":   "2J",
		"SSH-2.0-":                     "a computer",
	} {
		if got := PeerName(v); got != want {
			t.Errorf("%q: %q, want %q", v, got, want)
		}
	}
}

// A key for typing only, without a terminal, runs nothing: the shell
// that would run hellobro-keys is a root one.
func TestKeysNoPty(t *testing.T) {
	state := t.TempDir()
	s, err := Start(Config{Addr: "127.0.0.1:0", StateDir: state, Root: t.TempDir(), Notify: func(Event) {}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	keys := newKey(t)
	os.WriteFile(filepath.Join(state, authorizedFile), []byte(`command="hellobro-keys" `+
		strings.TrimSpace(string(ssh.MarshalAuthorizedKey(keys.PublicKey())))+" kobo\n"), 0o600)
	c, err := dial(s, keys, "")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	sess, _ := c.NewSession()
	var exited *ssh.ExitError
	if out, err := sess.Output("id"); err == nil || errors.As(err, &exited) { // ran, and ended: refused is wanted
		t.Errorf("ran without a terminal: %q, %v", out, err)
	}
}
