package main

// read-kitty: a book or a document in the terminal, a page at a time.
// It's this binary, by that name in the Alpine jail (alpineinit.go).
// mutool (mupdf-tools) draws each page for the window's pixels exactly,
// so it's shown pixel for pixel, and it goes to the terminal by kitty's
// graphics protocol (kittygfx.go): PDF, EPUB, MOBI, FB2, XPS, CBZ,
// whatever MuPDF opens. A book's text is laid out for the window, at a
// size + and - change. Where each file was left is kept for next time.

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"image"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"unsafe"
)

const readUsage = `usage: read-kitty FILE [PAGE]

  Space, Enter, j, →, ↓, PgDn   the next page
  b, Backspace, k, ←, ↑, PgUp   the one before
  Home, End                     the first, the last
  a number, then Enter          that page
  +, -                          a book's text bigger, smaller
  q, Esc                        quit
`

const (
	readDPI   = 227 // the Elipsa's: a book's points come out as on paper
	readEm    = 11  // a book's text, points
	readCount = `var doc = Document.openDocument(scriptArgs[0]);
if (doc.isReflowable()) doc.layout(+scriptArgs[1], +scriptArgs[2], +scriptArgs[3]);
print(doc.countPages() + " " + doc.isReflowable());
`
)

type reader struct {
	file, tmp   string
	page, pages int // page from 1
	em          int
	reflow      bool
	cols, rows  int
	px          image.Point // the window's pixels
	cell        image.Point

	mu    sync.Mutex     // mutool, one at a time, and the pages it drew
	drawn map[int][]byte // PNGs, by page: this one and the next
	typed string         // a page number, being typed
	note  string
}

func readMain(args []string) int {
	if len(args) < 1 || len(args) > 2 || strings.HasPrefix(args[0], "-") {
		fmt.Fprint(os.Stderr, readUsage)
		return 2
	}
	if _, err := exec.LookPath("mutool"); err != nil {
		fmt.Fprintln(os.Stderr, "read-kitty needs mutool: apk add mupdf-tools")
		return 1
	}
	file, err := filepath.Abs(args[0])
	if err == nil {
		_, err = os.Stat(file)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "read-kitty:", err)
		return 1
	}
	r := &reader{file: file, em: readEm, page: 1}
	r.page, r.em = readPlace(file)
	if len(args) == 2 {
		if r.page, err = strconv.Atoi(args[1]); err != nil {
			fmt.Fprint(os.Stderr, readUsage)
			return 2
		}
	}
	if r.tmp, err = os.MkdirTemp("", "read-kitty"); err != nil {
		fmt.Fprintln(os.Stderr, "read-kitty:", err)
		return 1
	}
	defer os.RemoveAll(r.tmp)
	if err := os.WriteFile(filepath.Join(r.tmp, "count.js"), []byte(readCount), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "read-kitty:", err)
		return 1
	}
	old, err := termRaw(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read-kitty: not a terminal")
		return 1
	}
	defer termSet(os.Stdin, old)
	if !r.measure(true) {
		termSet(os.Stdin, old)
		fmt.Fprintln(os.Stderr, "read-kitty: the terminal doesn't say its size in pixels: it doesn't show images")
		return 1
	}
	if err := r.layout(); err != nil {
		termSet(os.Stdin, old)
		fmt.Fprintln(os.Stderr, "read-kitty:", err)
		return 1
	}
	os.Stdout.WriteString("\x1b[?1049h\x1b[?25l")
	defer os.Stdout.WriteString("\x1b_Ga=d,d=A,q=2\x1b\\\x1b[?25h\x1b[?1049l")
	defer func() { readKeep(r.file, r.page, r.em) }() // as it is then

	keys := make(chan []byte)
	go func() {
		defer close(keys)
		for {
			b := make([]byte, 64)
			n, err := os.Stdin.Read(b)
			if err != nil {
				return
			}
			keys <- b[:n]
		}
	}()
	sigs := make(chan os.Signal, 4)
	signal.Notify(sigs, syscall.SIGWINCH, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGINT)
	r.show()
	for {
		select {
		case b, ok := <-keys:
			if !ok || !r.keys(b) {
				return 0
			}
		case s := <-sigs:
			if s != syscall.SIGWINCH {
				return 0
			}
			if r.measure(false) {
				r.relayout()
			}
		}
	}
}

// measure reads the window's size, in cells and pixels: from the pty,
// or, first, by asking the terminal if the pty doesn't say.
func (r *reader) measure(ask bool) bool {
	var ws struct{ row, col, x, y uint16 }
	if ioctl(os.Stdin, syscall.TIOCGWINSZ, unsafe.Pointer(&ws)) != nil || ws.col == 0 || ws.row < 2 {
		return false
	}
	r.cols, r.rows = int(ws.col), int(ws.row)
	switch {
	case ws.x > 0 && ws.y > 0:
		r.cell = image.Pt(int(ws.x)/r.cols, int(ws.y)/r.rows)
	case ask:
		r.cell = askCell()
	}
	r.px = image.Pt(r.cols*r.cell.X, r.rows*r.cell.Y)
	return r.cell.X > 0 && r.cell.Y > 0
}

// askCell asks the terminal for its cells' size (CSI 16 t), waiting a
// moment for the answer, CSI 6 ; h ; w t.
func askCell() image.Point {
	t, err := termGet(os.Stdin)
	if err != nil {
		return image.Point{}
	}
	wait := *t
	wait.Cc[syscall.VMIN], wait.Cc[syscall.VTIME] = 0, 5
	termSet(os.Stdin, &wait)
	defer termSet(os.Stdin, t)
	os.Stdout.WriteString("\x1b[16t")
	var got []byte
	b := make([]byte, 32)
	for len(got) < 64 && !bytes.HasSuffix(got, []byte("t")) {
		n, _ := os.Stdin.Read(b)
		if n == 0 {
			break
		}
		got = append(got, b[:n]...)
	}
	var h, w int
	if i := bytes.Index(got, []byte("\x1b[6;")); i >= 0 {
		fmt.Sscanf(string(got[i:]), "\x1b[6;%d;%dt", &h, &w)
	}
	return image.Pt(w, h)
}

// area is where a page goes: all but the last row, the status line's.
func (r *reader) area() image.Point { return image.Pt(r.px.X, r.px.Y-r.cell.Y) }

// book is a book's layout, in points: the page's size and the text's.
func (r *reader) book() []string {
	a := r.area()
	return []string{strconv.Itoa(a.X * 72 / readDPI), strconv.Itoa(a.Y * 72 / readDPI), strconv.Itoa(r.em)}
}

// layout counts the pages, a book's as laid out for the window.
func (r *reader) layout() error {
	out, err := exec.Command("mutool", append([]string{"run", filepath.Join(r.tmp, "count.js"), r.file}, r.book()...)...).CombinedOutput()
	var reflow string
	if _, e := fmt.Sscanf(string(out), "%d %s", &r.pages, &reflow); err != nil || e != nil || r.pages < 1 {
		return fmt.Errorf("mutool can't open it: %s", strings.TrimSpace(string(out)))
	}
	r.reflow = reflow == "true"
	r.page = max(1, min(r.pages, r.page))
	r.mu.Lock()
	r.drawn = map[int][]byte{}
	r.mu.Unlock()
	return nil
}

// relayout lays a book out again, the window or its text's size changed,
// and shows about where it was.
func (r *reader) relayout() {
	was, pages := r.page, r.pages
	if err := r.layout(); err != nil {
		r.note = err.Error()
	} else if r.reflow && pages > 1 {
		r.page = max(1, min(r.pages, 1+(was-1)*r.pages/pages))
	}
	r.show()
}

// draw is page n as a PNG, fit to the window.
func (r *reader) draw(n int) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if b := r.drawn[n]; b != nil {
		return b, nil
	}
	a := r.area()
	out := filepath.Join(r.tmp, "page.png")
	args := []string{"draw", "-q", "-F", "png", "-c", "gray", "-w", strconv.Itoa(a.X), "-h", strconv.Itoa(a.Y), "-o", out}
	if r.reflow {
		b := r.book()
		args = append(args, "-W", b[0], "-H", b[1], "-S", b[2])
	}
	os.Remove(out)
	if msg, err := exec.Command("mutool", append(args, r.file, strconv.Itoa(n))...).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("page %d: %s", n, strings.TrimSpace(string(msg)))
	}
	b, err := os.ReadFile(out)
	if err != nil {
		return nil, err
	}
	for k := range r.drawn { // only this one and the next are kept
		if k != n-1 && k != n+1 {
			delete(r.drawn, k)
		}
	}
	r.drawn[n] = b
	return b, nil
}

// show puts the page on screen, in the middle, and the status line under
// it; then the next page is drawn, for when it's wanted.
func (r *reader) show() {
	png, err := r.draw(r.page)
	var b strings.Builder
	b.WriteString("\x1b_Ga=d,d=A,q=2\x1b\\\x1b[2J")
	if err != nil {
		r.note = err.Error()
	} else if w, h, ok := pngSize(png); ok {
		a := r.area()
		fmt.Fprintf(&b, "\x1b[%d;%dH", 1+max(0, (a.Y-h)/2/r.cell.Y), 1+max(0, (a.X-w)/2/r.cell.X))
		data := base64.StdEncoding.EncodeToString(png)
		for first := true; data != "" || first; first = false {
			part := data[:min(len(data), 4096)]
			data = data[len(part):]
			more := 0
			if data != "" {
				more = 1
			}
			if first {
				fmt.Fprintf(&b, "\x1b_Ga=T,f=100,q=2,C=1,m=%d;%s\x1b\\", more, part)
			} else {
				fmt.Fprintf(&b, "\x1b_Gm=%d;%s\x1b\\", more, part)
			}
		}
	}
	b.WriteString(r.status())
	os.Stdout.WriteString(b.String())
	r.note = ""
	if r.page < r.pages {
		go r.draw(r.page + 1)
	}
}

// status is the last row: the file, the page, what's being typed.
func (r *reader) status() string {
	left := fmt.Sprintf("%s · %d/%d", filepath.Base(r.file), r.page, r.pages)
	if r.reflow {
		left += fmt.Sprintf(" · %d pt", r.em)
	}
	right := "space b: pages · q: quit"
	switch {
	case r.typed != "":
		right = "page " + r.typed + "_ · Enter: go"
	case r.note != "":
		right = r.note
	}
	line := left + "   " + right
	if gap := r.cols - len([]rune(left)) - len([]rune(right)); gap > 3 {
		line = left + strings.Repeat(" ", gap) + right
	}
	if rs := []rune(line); len(rs) > r.cols {
		line = string(rs[:r.cols])
	}
	return fmt.Sprintf("\x1b[%d;1H\x1b[2K%s", r.rows, line)
}

func pngSize(b []byte) (w, h int, ok bool) {
	if len(b) < 24 || string(b[1:4]) != "PNG" {
		return 0, 0, false
	}
	return int(binary.BigEndian.Uint32(b[16:])), int(binary.BigEndian.Uint32(b[20:])), true
}

// keys does what keys say; false: quit.
func (r *reader) keys(b []byte) bool {
	for len(b) > 0 {
		k, n := readKey(b)
		b = b[n:]
		page := r.page
		switch {
		case k >= "0" && k <= "9" && len(k) == 1:
			if len(r.typed) < 6 {
				r.typed += k
			}
			os.Stdout.WriteString(r.status())
			continue
		case k == "enter" && r.typed != "":
			page, _ = strconv.Atoi(r.typed)
			r.typed = ""
		case k == "back" && r.typed != "":
			r.typed = r.typed[:len(r.typed)-1]
			os.Stdout.WriteString(r.status())
			continue
		case k == "esc" && r.typed != "":
			r.typed = ""
			os.Stdout.WriteString(r.status())
			continue
		case k == "q" || k == "esc" || k == "\x03":
			return false
		case k == "next" || k == "enter" || strings.Contains(" jln", k):
			page++
		case k == "prev" || k == "back" || strings.Contains("bkhp", k):
			page--
		case k == "home" || k == "g":
			page = 1
		case k == "end" || k == "G":
			page = r.pages
		case (k == "+" || k == "=" || k == "-") && r.reflow:
			if k == "-" {
				r.em = max(6, r.em-1)
			} else {
				r.em = min(30, r.em+1)
			}
			r.relayout()
			readKeep(r.file, r.page, r.em)
			continue
		default:
			continue
		}
		if page = max(1, min(r.pages, page)); page != r.page {
			r.page = page
			r.show()
			readKeep(r.file, r.page, r.em)
		} else if r.typed == "" {
			os.Stdout.WriteString(r.status())
		}
	}
	return true
}

// readKey is the first key in b, and its length: the keys that do
// something by their names, the rest as they are.
func readKey(b []byte) (string, int) {
	switch b[0] {
	case '\r', '\n':
		return "enter", 1
	case 0x7f, 0x08:
		return "back", 1
	case 0x1b:
	default:
		return string(b[:1]), 1
	}
	if len(b) == 1 {
		return "esc", 1
	}
	if b[1] != '[' && b[1] != 'O' {
		return "", 2 // Alt+key
	}
	n := 2
	for n < len(b) && (b[n] < 0x40 || b[n] > 0x7e) {
		n++
	}
	if n == len(b) {
		return "", n
	}
	seq := string(b[2 : n+1])
	n++
	switch seq {
	case "C", "B", "6~":
		return "next", n
	case "D", "A", "5~":
		return "prev", n
	case "H", "1~", "7~":
		return "home", n
	case "F", "4~", "8~":
		return "end", n
	}
	return "", n
}

// The places: where each file was left, and its text's size.

func readPlacesFile() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state", "read-kitty", "places")
}

func readPlace(file string) (page, em int) {
	page, em = 1, readEm
	b, _ := os.ReadFile(readPlacesFile())
	for line := range strings.SplitSeq(string(b), "\n") {
		f := strings.SplitN(line, "\t", 3)
		if len(f) == 3 && f[2] == file {
			page, _ = strconv.Atoi(f[0])
			em, _ = strconv.Atoi(f[1])
		}
	}
	return max(1, page), max(6, min(30, em))
}

func readKeep(file string, page, em int) {
	path := readPlacesFile()
	b, _ := os.ReadFile(path)
	var keep []string
	for line := range strings.SplitSeq(string(b), "\n") {
		if f := strings.SplitN(line, "\t", 3); len(f) == 3 && f[2] != file {
			keep = append(keep, line)
		}
	}
	keep = append(keep, fmt.Sprintf("%d\t%d\t%s", page, em, file))
	if len(keep) > 200 {
		keep = keep[len(keep)-200:]
	}
	os.MkdirAll(filepath.Dir(path), 0o755)
	if os.WriteFile(path+".new", []byte(strings.Join(keep, "\n")+"\n"), 0o644) == nil {
		os.Rename(path+".new", path)
	}
}

// --- the terminal's modes ---

func termGet(f *os.File) (*syscall.Termios, error) {
	var t syscall.Termios
	if err := ioctl(f, syscall.TCGETS, unsafe.Pointer(&t)); err != nil {
		return nil, err
	}
	return &t, nil
}

func termSet(f *os.File, t *syscall.Termios) error {
	return ioctl(f, syscall.TCSETS, unsafe.Pointer(t))
}

// termRaw puts f's terminal in raw mode: keys as they come, no echo, no
// signals. It says the mode it was in.
func termRaw(f *os.File) (*syscall.Termios, error) {
	old, err := termGet(f)
	if err != nil {
		return nil, err
	}
	t := *old
	t.Iflag &^= syscall.IGNBRK | syscall.BRKINT | syscall.PARMRK | syscall.ISTRIP | syscall.INLCR | syscall.IGNCR | syscall.ICRNL | syscall.IXON
	t.Oflag &^= syscall.OPOST
	t.Lflag &^= syscall.ECHO | syscall.ECHONL | syscall.ICANON | syscall.ISIG | syscall.IEXTEN
	t.Cflag &^= syscall.CSIZE | syscall.PARENB
	t.Cflag |= syscall.CS8
	t.Cc[syscall.VMIN], t.Cc[syscall.VTIME] = 1, 0
	return old, termSet(f, &t)
}
