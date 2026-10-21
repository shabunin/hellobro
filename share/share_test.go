package share

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"io"
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
