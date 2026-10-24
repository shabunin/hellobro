package main

// read-kitty: a book or a document in the terminal, a page at a time.
// It's this binary, by that name in the Alpine jail (alpineinit.go).
// mutool (mupdf-tools) draws the page for the window's pixels exactly,
// so it's shown pixel for pixel, and it goes to the terminal by kitty's
// graphics protocol (kittygfx.go): PDF, EPUB, MOBI, FB2, XPS, CBZ,
// pictures, whatever MuPDF opens. A book's text is laid out for the
// window, at a size + and - change; a page that's set (a PDF, a
// picture) zooms with them instead, and mutool draws only the window's
// part of it. Where each file was left is kept for next time.

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

  Space, Enter, PgDn            on: down the page, then the next one
  b, Backspace, PgUp            back
  →, ↓, ←, ↑ (or l j h k)       the next page, the one before; zoomed in,
                                they move about the page
  Home, End                     the first page, the last
  a number, then Enter          that page
  +, -                          zoom in, out; in a book, its text bigger, smaller
  w                             zoom to the page's width, and back
  q, Esc                        quit
`

const (
	readDPI     = 227 // the Elipsa's: a book's points come out as on paper
	readEm      = 11  // a book's text, points
	readZoomMax = 8
	// The pages, a book's as laid out: "COUNT REFLOWABLE".
	readCount = `var doc = Document.openDocument(scriptArgs[0]);
if (doc.isReflowable()) doc.layout(+scriptArgs[1], +scriptArgs[2], +scriptArgs[3]);
print(doc.countPages() + " " + doc.isReflowable());
`
	// A page, zoomed (1: it fits W×H), the W×H of it at X, Y (kept in)
	// drawn to OUT: "the page's WIDTH HEIGHT as zoomed, X Y, the zoom
	// that fits its width".
	// Args: FILE PAGE W H ZOOM X Y OUT [BOOK-W BOOK-H EM].
	readDraw = `var a = scriptArgs;
var doc = Document.openDocument(a[0]);
if (doc.isReflowable()) doc.layout(+a[8], +a[9], +a[10]);
var page = doc.loadPage(+a[1] - 1);
var b = page.getBounds();
var pw = b[2] - b[0], ph = b[3] - b[1];
var W = +a[2], H = +a[3], fit = Math.min(W / pw, H / ph), s = fit * +a[4];
var iw = Math.round(pw * s), ih = Math.round(ph * s);
var w = Math.min(W, iw), h = Math.min(H, ih);
var x = Math.max(0, Math.min(iw - w, +a[5])), y = Math.max(0, Math.min(ih - h, +a[6]));
var pix = new Pixmap(ColorSpace.DeviceGray, [0, 0, w, h], false);
pix.clear(255);
var dev = new DrawDevice(Matrix.identity, pix);
page.run(dev, [s, 0, 0, s, -b[0] * s - x, -b[1] * s - y]);
dev.close();
pix.saveAsPNG(a[7]);
print(iw + " " + ih + " " + x + " " + y + " " + (W / pw / fit));
`
)

// readView is what's on screen: a page, zoomed, and where the window is
// in it.
type readView struct {
	page int     // from 1
	zoom float64 // 1: the page fits the window
	x, y int     // the window's top left, in the page as zoomed; mutool keeps it in
}

// readDrawn is a view drawn.
type readDrawn struct {
	png       []byte
	size      image.Point // the page, as zoomed
	at        image.Point // the window's top left in it, as it came out
	widthZoom float64     // the zoom that fits the page's width to the window
}

type reader struct {
	file, tmp  string
	v          readView
	d          *readDrawn // v, drawn
	pages      int
	em         int
	reflow     bool
	cols, rows int
	px         image.Point // the window's pixels
	cell       image.Point

	mu    sync.Mutex // mutool, one at a time, and the views it drew
	drawn map[readView]*readDrawn
	typed string // a page number, being typed
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
	r := &reader{file: file, v: readView{page: 1, zoom: 1}}
	r.v.page, r.em = readPlace(file)
	if len(args) == 2 {
		if r.v.page, err = strconv.Atoi(args[1]); err != nil {
			fmt.Fprint(os.Stderr, readUsage)
			return 2
		}
	}
	if r.tmp, err = os.MkdirTemp("", "read-kitty"); err != nil {
		fmt.Fprintln(os.Stderr, "read-kitty:", err)
		return 1
	}
	defer os.RemoveAll(r.tmp)
	for name, js := range map[string]string{"count.js": readCount, "draw.js": readDraw} {
		if err := os.WriteFile(filepath.Join(r.tmp, name), []byte(js), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, "read-kitty:", err)
			return 1
		}
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
	defer func() { readKeep(r.file, r.v.page, r.em) }() // as it is then

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
	// Its warnings (EPUB 3's "unknown epub version", say) go to stderr:
	// only the answer is read.
	out, msg, err := r.mutool("count.js", r.file)
	var reflow string
	if _, e := fmt.Sscanf(out, "%d %s", &r.pages, &reflow); err != nil || e != nil || r.pages < 1 {
		return fmt.Errorf("mutool can't open it: %s", msg)
	}
	r.reflow = reflow == "true"
	r.v.page = max(1, min(r.pages, r.v.page))
	if r.reflow {
		r.v.zoom, r.v.x, r.v.y = 1, 0, 0
	}
	r.mu.Lock()
	r.drawn = map[readView]*readDrawn{}
	r.mu.Unlock()
	return nil
}

// mutool runs one of the scripts, a book laid out for the window, and
// says what it printed, and what it said on stderr.
func (r *reader) mutool(script string, args ...string) (out, msg string, err error) {
	var o, m bytes.Buffer
	cmd := exec.Command("mutool", append(append([]string{"run", filepath.Join(r.tmp, script)}, args...), r.book()...)...)
	cmd.Stdout, cmd.Stderr = &o, &m
	err = cmd.Run()
	return o.String(), strings.TrimSpace(m.String()), err
}

// relayout lays a book out again, the window or its text's size changed,
// and shows about where it was.
func (r *reader) relayout() {
	was, pages := r.v.page, r.pages
	if err := r.layout(); err != nil {
		r.note = err.Error()
	} else if r.reflow && pages > 1 {
		r.v.page = max(1, min(r.pages, 1+(was-1)*r.pages/pages))
	}
	r.show()
}

// draw is v drawn, a PNG fit to the window.
func (r *reader) draw(v readView) (*readDrawn, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if d := r.drawn[v]; d != nil {
		return d, nil
	}
	a := r.area()
	png := filepath.Join(r.tmp, "page.png")
	os.Remove(png)
	out, msg, err := r.mutool("draw.js", r.file, strconv.Itoa(v.page), strconv.Itoa(a.X), strconv.Itoa(a.Y),
		strconv.FormatFloat(v.zoom, 'f', 3, 64), strconv.Itoa(v.x), strconv.Itoa(v.y), png)
	d := &readDrawn{}
	if _, e := fmt.Sscanf(out, "%d %d %d %d %g", &d.size.X, &d.size.Y, &d.at.X, &d.at.Y, &d.widthZoom); err != nil || e != nil {
		return nil, fmt.Errorf("page %d: %s", v.page, msg)
	}
	if d.png, err = os.ReadFile(png); err != nil {
		return nil, err
	}
	if len(r.drawn) > 4 { // what's on screen, what's next: the rest goes
		clear(r.drawn)
	}
	r.drawn[v] = d
	return d, nil
}

// show puts the view on screen, in the middle, and the status line under
// it; then what Space shows next is drawn, for when it's wanted.
func (r *reader) show() {
	d, err := r.draw(r.v)
	var b strings.Builder
	b.WriteString("\x1b_Ga=d,d=A,q=2\x1b\\\x1b[2J")
	if err != nil {
		r.note = err.Error()
	} else if w, h, ok := pngSize(d.png); ok {
		r.d, r.v.x, r.v.y = d, d.at.X, d.at.Y
		a := r.area()
		fmt.Fprintf(&b, "\x1b[%d;%dH", 1+max(0, (a.Y-h)/2/r.cell.Y), 1+max(0, (a.X-w)/2/r.cell.X))
		data := base64.StdEncoding.EncodeToString(d.png)
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
	if next := r.forward(); next != r.v {
		go r.draw(next)
	}
}

// zoomed says whether the page is bigger than the window, there to be
// moved about.
func (r *reader) zoomed() bool {
	a := r.area()
	return r.d != nil && (r.d.size.X > a.X || r.d.size.Y > a.Y)
}

// forward is what Space shows: down the page, or the next one's top.
func (r *reader) forward() readView {
	v, a := r.v, r.area()
	if r.d != nil && v.y+a.Y < r.d.size.Y {
		v.y += a.Y * 9 / 10
	} else if v.page < r.pages {
		v.page, v.y = v.page+1, 0
	}
	return v
}

// backward is what b shows: up the page, or the one before's bottom.
func (r *reader) backward() readView {
	v, a := r.v, r.area()
	if v.y > 0 {
		v.y = max(0, v.y-a.Y*9/10)
	} else if v.page > 1 {
		v.page, v.y = v.page-1, 1<<30 // mutool keeps it in
	}
	return v
}

// move is the window moved about the page by dx, dy.
func (r *reader) move(dx, dy int) readView {
	v, a := r.v, r.area()
	v.x = max(0, min(r.d.size.X-a.X, v.x+dx))
	v.y = max(0, min(r.d.size.Y-a.Y, v.y+dy))
	return r.keepIn(v)
}

func (r *reader) keepIn(v readView) readView {
	v.x, v.y = max(0, v.x), max(0, v.y)
	return v
}

// zoomTo is the page at zoom z, the window's middle where it was.
func (r *reader) zoomTo(z float64) readView {
	v, a := r.v, r.area()
	z = max(1, min(readZoomMax, z))
	if z < 1.02 {
		return readView{page: v.page, zoom: 1}
	}
	if r.d == nil {
		v.zoom = z
		return v
	}
	k := z / v.zoom
	mx, my := float64(v.x+min(a.X, r.d.size.X)/2), float64(v.y+min(a.Y, r.d.size.Y)/2)
	v.zoom, v.x, v.y = z, int(mx*k)-a.X/2, int(my*k)-a.Y/2
	return r.keepIn(v)
}

// status is the last row: the file, the page, what's being typed.
func (r *reader) status() string {
	left := fmt.Sprintf("%s · %d/%d", filepath.Base(r.file), r.v.page, r.pages)
	if r.reflow {
		left += fmt.Sprintf(" · %d pt", r.em)
	}
	if r.v.zoom > 1 {
		left += fmt.Sprintf(" · %.0f%%", r.v.zoom*100)
	}
	right := "space b: pages · +: zoom · q: quit"
	switch {
	case r.typed != "":
		right = "page " + r.typed + "_ · Enter: go"
	case r.note != "":
		right = r.note
	case r.reflow:
		right = "space b: pages · + -: text · q: quit"
	case r.zoomed():
		right = "arrows: move · w: width · -: out"
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
		v, a := r.v, r.area()
		zoomed := r.zoomed()
		switch {
		case k >= "0" && k <= "9" && len(k) == 1:
			if len(r.typed) < 6 {
				r.typed += k
			}
		case k == "enter" && r.typed != "":
			p, _ := strconv.Atoi(r.typed)
			v.page, v.x, v.y, r.typed = max(1, min(r.pages, p)), 0, 0, ""
		case k == "back" && r.typed != "":
			r.typed = r.typed[:len(r.typed)-1]
		case k == "esc" && r.typed != "":
			r.typed = ""
		case k == "q" || k == "esc" || k == "\x03":
			return false
		case k == " " || k == "enter" || k == "pgdn" || k == "n":
			v = r.forward()
		case k == "b" || k == "back" || k == "pgup" || k == "p":
			v = r.backward()
		case (k == "down" || k == "j") && zoomed:
			v = r.move(0, a.Y/3)
		case (k == "up" || k == "k") && zoomed:
			v = r.move(0, -a.Y/3)
		case (k == "right" || k == "l") && zoomed:
			v = r.move(a.X/2, 0)
		case (k == "left" || k == "h") && zoomed:
			v = r.move(-a.X/2, 0)
		case k == "down" || k == "j" || k == "right" || k == "l":
			v.page, v.y = min(r.pages, v.page+1), 0
		case k == "up" || k == "k" || k == "left" || k == "h":
			v.page, v.y = max(1, v.page-1), 0
		case k == "home" || k == "g":
			v.page, v.x, v.y = 1, 0, 0
		case k == "end" || k == "G":
			v.page, v.x, v.y = r.pages, 0, 0
		case (k == "+" || k == "=" || k == "-") && r.reflow:
			if k == "-" {
				r.em = max(6, r.em-1)
			} else {
				r.em = min(30, r.em+1)
			}
			r.relayout()
			readKeep(r.file, r.v.page, r.em)
			continue
		case k == "+" || k == "=":
			v = r.zoomTo(v.zoom * 1.5)
		case k == "-":
			v = r.zoomTo(v.zoom / 1.5)
		case k == "w" && r.d != nil && !r.reflow:
			if z := r.d.widthZoom; z > 1.02 && (v.zoom < z-0.01 || v.zoom > z+0.01) {
				v = r.zoomTo(z)
			} else {
				v = r.zoomTo(1)
			}
		default:
			continue
		}
		if v != r.v {
			page := r.v.page
			r.v = v
			r.show()
			if v.page != page {
				readKeep(r.file, v.page, r.em)
			}
		} else {
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
	if k, ok := map[string]string{"A": "up", "B": "down", "C": "right", "D": "left", "5~": "pgup", "6~": "pgdn",
		"H": "home", "1~": "home", "7~": "home", "F": "end", "4~": "end", "8~": "end"}[seq]; ok {
		return k, n
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
