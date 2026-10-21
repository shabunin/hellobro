package main

// The Files tab: sharing with a computer (share.go) over what's on the
// Kobo. The top panel says how to reach the Kobo, shows the pairing code
// and who's connected, sending what; the one under it lists the Kobo's
// storage (/mnt/onboard): folders open, files and folders can be deleted.

import (
	"fmt"
	"image"
	"log"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"hellobro/kobo"
	"hellobro/share"
)

const fileRowH = 52

type fileEnt struct {
	name string
	dir  bool
	size int64
	mod  time.Time
}

type filesView struct {
	shareP, listP kobo.Panel
	text          image.Rectangle // the share panel's lines
	shareBtns     []radioButton
	path          string // the folder listed, under shareRoot
	ents          []fileEnt
	hidden        bool // dot files shown
	sel, top      int
	list, pathR   image.Rectangle
	acts          image.Rectangle
	listBtns      []radioButton
	rows          []image.Rectangle
	rowsOn        int
	err, free     string
	listAt        time.Time
	shownShare    string
	shownList     string
}

func (a *bro) layoutFiles(K kobo.Layout) {
	v := &a.fv
	fs, fl := a.folded("sharing"), a.folded("files")
	var at []image.Rectangle
	if K.Landscape {
		at = K.Arrange(K.Body, kobo.Row{Height: 100, Widths: []int{45, 55}, Folded: []bool{fs, fl}})
	} else {
		at = K.Arrange(K.Body, kobo.Row{Height: 31, Folded: []bool{fs}}, kobo.Row{Height: 69, Folded: []bool{fl}})
	}
	v.shareP = kobo.WithFold(a.F, kobo.NewPanel(a.F, "Sharing", at[0], ""), fs)
	v.listP = kobo.WithFold(a.F, kobo.NewPanel(a.F, "On this Kobo", at[1], ""), fl)
	v.shownShare, v.shownList = "", ""
	v.text, v.shareBtns = image.Rectangle{}, v.shareBtns[:0]
	if !fs {
		b := v.shareP.Body.Inset(kobo.Pad)
		v.text = image.Rect(b.Min.X, b.Min.Y, b.Max.X, b.Max.Y-76)
		for i, label := range []string{"on/off", "pair", "forget paired"} {
			x := b.Min.X + i*262
			v.shareBtns = append(v.shareBtns, radioButton{r: image.Rect(x, b.Max.Y-64, x+250, b.Max.Y), label: label})
		}
	}
	v.pathR, v.acts, v.list, v.rowsOn, v.rows = image.Rectangle{}, image.Rectangle{}, image.Rectangle{}, 1, nil
	v.listBtns = v.listBtns[:0]
	if fl { // nothing in it to draw or tap
		return
	}
	l := v.listP.Body.Inset(kobo.Pad)
	v.pathR = image.Rect(l.Min.X, l.Min.Y, l.Max.X, l.Min.Y+44)
	v.acts = image.Rect(l.Min.X, l.Max.Y-64, l.Max.X, l.Max.Y)
	v.list = image.Rect(l.Min.X, v.pathR.Max.Y+8, l.Max.X, v.acts.Min.Y-12)
	v.rowsOn = max(1, v.list.Dy()/fileRowH)
	for i, label := range []string{"open", "up", "delete", "refresh"} {
		x := l.Min.X + i*212
		v.listBtns = append(v.listBtns, radioButton{r: image.Rect(x, v.acts.Min.Y, x+200, v.acts.Max.Y), label: label})
	}
}

func (a *bro) showingFiles() bool {
	return a.Tab == tabFiles && !a.Calibrating() && !a.overlay() && !a.Dozing()
}

func (a *bro) enterFiles() {
	v := &a.fv
	if v.path == "" {
		v.path = shareRoot
		if st, err := os.Stat(filepath.Join(shareRoot, "Music")); err == nil && st.IsDir() {
			v.path = filepath.Join(shareRoot, "Music")
		}
	}
	a.listFiles()
	a.sharePoll()
}

func (a *bro) drawFiles() {
	a.fv.shownShare, a.fv.shownList = "", ""
	a.drawShare()
	a.drawFileList()
}

func (a *bro) filesTick() {
	if a.Tab != tabFiles {
		return
	}
	every := 10 * time.Second
	if a.shareBusy() {
		every = 3 * time.Second // things arrive
	}
	if time.Since(a.fv.listAt) > every {
		a.listFiles()
	}
}

// --- the share panel ---

func (a *bro) drawShare() {
	if !a.showingFiles() {
		return
	}
	v, s, c := &a.fv, &a.share, a.C
	type line struct {
		text string
		big  bool
		g    uint8
	}
	var lines []line
	add := func(g uint8, f string, args ...any) { lines = append(lines, line{fmt.Sprintf(f, args...), false, g}) }
	ip := share.IPv4(shareIf)
	_, port, _ := net.SplitHostPort(shareAddr)
	status := "off"
	switch {
	case s.srv == nil:
		add(kobo.Black, "File sharing is off: o turns it on (or Settings).")
		add(kobo.Dark, "Then a computer reaches the Kobo's storage with a file manager (sftp://), sftp, scp or sshfs, or a shell with ssh.")
		if s.err != "" {
			add(kobo.Black, "It didn't start: %s", s.err)
		}
	default:
		status = fmt.Sprintf("on · %d connected", len(s.conns))
		host := ip
		if a.setting("mdns") == "on" {
			host = mdnsHost + ".local"
		}
		if ip == "" {
			add(kobo.Black, "No Wi-Fi address: turn Wi-Fi on (Settings).")
		} else {
			add(kobo.Black, "sftp://%s:%s%s", host, port, shareRoot)
			add(kobo.Dark, "ssh -p %s %s  ·  scp -P %s song.mp3 %s:Music/  ·  sshfs -p %s %s:%s ~/kobo", port, host, port, host, port, host, shareRoot)
			add(kobo.Mid, "%s:%s  ·  host key %s", ip, port, s.srv.HostKey())
		}
		if s.code != "" {
			status = "pairing"
			left := max(0, time.Until(s.until)).Round(time.Second)
			lines = append(lines, line{"Pairing code: " + s.code[:3] + " " + s.code[3:], true, kobo.Black})
			add(kobo.Dark, "On the computer: ssh -p %s %s, then type the code (%s left).", port, host, left)
		} else {
			add(kobo.Dark, "Paired computers: %d  (p: pair one more)", len(s.keys))
		}
		switch r := &a.relay; {
		case r.stop != nil && r.up:
			add(kobo.Black, "Relay %s · from afar: ssh -J you@%s -p %d root@127.0.0.1", r.status, r.conf.server(), r.conf.port)
		case r.stop != nil:
			add(kobo.Black, "Relay %s", r.status)
		case r.err != "":
			add(kobo.Black, "Relay: %s", r.err)
		}
		for _, cn := range s.conns {
			what := cn.What
			if what == "" {
				what = "connecting"
			}
			rate := ""
			if r := s.rate[cn.ID]; r > 1024 {
				rate = fmt.Sprintf(" (%s/s)", humanBytes(int64(r)))
			}
			add(kobo.Black, "● %s@%s · %s · ↓ %s%s ↑ %s · %s", cn.User, cn.Addr, what, humanBytes(cn.In), rate, humanBytes(cn.Out),
				time.Since(cn.Since).Round(time.Second))
		}
	}
	if s.note != "" {
		add(kobo.Mid, "%s", s.note)
	}
	key := fmt.Sprint(status, lines)
	if key == v.shownShare {
		return
	}
	v.shownShare = key
	a.DrawPanel(v.shareP, status)
	if v.shareP.Folded {
		return
	}
	y := v.text.Min.Y
	for _, l := range lines {
		f, h := a.F.Small, 40
		if l.big {
			f, h = a.F.Panel, 64
		}
		if y+h > v.text.Max.Y {
			break
		}
		c.Text(f, image.Rect(v.text.Min.X, y, v.text.Max.X, y+h), l.text, l.g, kobo.AlignLeft)
		y += h
	}
	for _, b := range v.shareBtns {
		label := b.label
		if label == "on/off" {
			label = map[bool]string{true: "turn off", false: "turn on"}[s.srv != nil]
		}
		c.Fill(b.r, kobo.White)
		c.Frame(b.r, 2, kobo.Black)
		c.Text(a.F.Small, b.r, label, kobo.Black, kobo.AlignCenter)
	}
	a.Mark(v.shareP.Outer, true)
}

func (a *bro) shareToggle() {
	i := settingIndex("share")
	if a.setting("share") == "on" {
		a.set["share"] = indexOf(settingList[i].values, "off")
	} else {
		a.set["share"] = indexOf(settingList[i].values, "on")
	}
	a.set.save()
	a.shareApply()
}

func (a *bro) shareForgetAll() {
	s := &a.share
	if s.srv == nil || len(s.keys) == 0 {
		return
	}
	a.ask("Forget the paired computers?", fmt.Sprintf("%d of them will have to pair again to get in.", len(s.keys)), "forget", func() {
		for _, k := range s.keys {
			if err := s.srv.Forget(k.Fingerprint); err != nil {
				log.Printf("share: forget: %v", err)
			}
		}
		s.keys, _ = s.srv.Keys()
		s.note = "forgot the paired computers"
		a.drawShare()
	})
}

// --- the list ---

// listFiles reads the folder again.
func (a *bro) listFiles() {
	v := &a.fv
	v.listAt = time.Now()
	ds, err := os.ReadDir(v.path)
	v.err = ""
	if err != nil {
		v.err = err.Error()
	}
	cur := ""
	if v.sel < len(v.ents) {
		cur = v.ents[v.sel].name
	}
	v.ents = v.ents[:0]
	for _, d := range ds {
		if strings.HasPrefix(d.Name(), ".") && !v.hidden {
			continue
		}
		e := fileEnt{name: d.Name(), dir: d.IsDir()}
		if info, err := d.Info(); err == nil {
			e.size, e.mod = info.Size(), info.ModTime()
		}
		v.ents = append(v.ents, e)
	}
	slices.SortFunc(v.ents, func(x, y fileEnt) int {
		if x.dir != y.dir {
			return map[bool]int{true: -1, false: 1}[x.dir]
		}
		return strings.Compare(strings.ToLower(x.name), strings.ToLower(y.name))
	})
	v.sel = 0
	for i, e := range v.ents {
		if e.name == cur {
			v.sel = i
		}
	}
	var st syscall.Statfs_t
	if syscall.Statfs(v.path, &st) == nil {
		v.free = humanBytes(int64(st.Bavail)*int64(st.Bsize)) + " free"
	}
	a.drawFileList()
}

func (a *bro) drawFileList() {
	if !a.showingFiles() {
		return
	}
	v, c := &a.fv, a.C
	v.sel = max(0, min(v.sel, len(v.ents)-1))
	if v.sel < v.top {
		v.top = v.sel
	}
	if v.sel >= v.top+v.rowsOn {
		v.top = v.sel - v.rowsOn + 1
	}
	v.top = max(0, min(v.top, len(v.ents)-v.rowsOn))
	key := fmt.Sprint(v.path, v.ents, v.sel, v.top, v.err, v.free)
	if key == v.shownList {
		return
	}
	v.shownList = key
	a.DrawPanel(v.listP, v.free)
	if v.listP.Folded {
		v.rows = v.rows[:0]
		return
	}
	c.Text(a.F.Small, v.pathR, v.path+"/", kobo.Dark, kobo.AlignLeft)
	c.Fill(image.Rect(v.list.Min.X, v.list.Min.Y-4, v.list.Max.X, v.list.Min.Y-2), kobo.Light)
	v.rows = v.rows[:0]
	switch {
	case v.err != "":
		c.Text(a.F.Small, image.Rect(v.list.Min.X, v.list.Min.Y, v.list.Max.X, v.list.Min.Y+fileRowH), v.err, kobo.Black, kobo.AlignLeft)
	case len(v.ents) == 0:
		c.Text(a.F.Small, image.Rect(v.list.Min.X, v.list.Min.Y, v.list.Max.X, v.list.Min.Y+fileRowH), "Empty. Send something from a computer.", kobo.Dark, kobo.AlignLeft)
	}
	for j := v.top; j < min(len(v.ents), v.top+v.rowsOn); j++ {
		e := v.ents[j]
		rr := image.Rect(v.list.Min.X, v.list.Min.Y+(j-v.top)*fileRowH, v.list.Max.X-14, v.list.Min.Y+(j-v.top+1)*fileRowH)
		v.rows = append(v.rows, rr)
		fg, dim := uint8(kobo.Black), uint8(kobo.Dark)
		if j == v.sel {
			c.Fill(rr, kobo.Black)
			fg, dim = kobo.White, kobo.Light
		}
		t := rr.Inset(4)
		t.Min.X += 12
		detail := e.mod.Format("2006-01-02 15:04")
		name := e.name
		if e.dir {
			name = "► " + name + "/"
		} else {
			detail = humanBytes(e.size) + "  ·  " + detail
			name = "   " + name
		}
		dw := kobo.TextWidth(a.F.Small, detail)
		c.Text(a.F.Small, image.Rect(t.Max.X-dw, t.Min.Y, t.Max.X, t.Max.Y), detail, dim, kobo.AlignRight)
		t.Max.X -= dw + 20
		c.Text(a.F.Body, t, name, fg, kobo.AlignLeft)
	}
	if n := len(v.ents); n > v.rowsOn {
		tr := image.Rect(v.list.Max.X-6, v.list.Min.Y, v.list.Max.X, v.list.Min.Y+v.rowsOn*fileRowH)
		c.Fill(tr, kobo.Light)
		h := max(24, tr.Dy()*v.rowsOn/n)
		y := tr.Min.Y + (tr.Dy()-h)*v.top/max(1, n-v.rowsOn)
		c.Fill(image.Rect(tr.Min.X, y, tr.Max.X, y+h), kobo.Dark)
	}
	for _, b := range v.listBtns {
		c.Fill(b.r, kobo.White)
		c.Frame(b.r, 2, kobo.Black)
		c.Text(a.F.Small, b.r, b.label, kobo.Black, kobo.AlignCenter)
	}
	hint := "Enter open · Backspace up · Del delete · r · h hidden · o sharing · p pair"
	hx := v.listBtns[len(v.listBtns)-1].r.Max.X + 20
	c.Text(a.F.Small, image.Rect(hx, v.acts.Min.Y, v.acts.Max.X, v.acts.Max.Y), hint, kobo.Mid, kobo.AlignRight)
	a.Mark(v.listP.Outer, true)
}

func (a *bro) fileOpen() {
	v := &a.fv
	if v.sel >= len(v.ents) || !v.ents[v.sel].dir {
		return
	}
	v.path = filepath.Join(v.path, v.ents[v.sel].name)
	v.sel, v.top = 0, 0
	a.listFiles()
}

func (a *bro) fileUp() {
	v := &a.fv
	if v.path == shareRoot {
		return
	}
	from := filepath.Base(v.path)
	v.path = filepath.Dir(v.path)
	v.ents = v.ents[:0]
	a.listFiles()
	for i, e := range v.ents {
		if e.name == from {
			v.sel = i
		}
	}
	a.drawFileList()
}

func (a *bro) fileDelete() {
	v := &a.fv
	if v.sel >= len(v.ents) {
		return
	}
	e := v.ents[v.sel]
	path := filepath.Join(v.path, e.name)
	if !strings.HasPrefix(path, shareRoot+"/") {
		return
	}
	what := fmt.Sprintf("%s (%s)", e.name, humanBytes(e.size))
	if e.dir {
		what = e.name + "/ and everything in it"
	}
	a.ask("Delete it?", what+" goes for good.", "delete", func() {
		log.Printf("files: deleting %s", path)
		go func() {
			err := os.RemoveAll(path)
			a.Do(func() {
				if err != nil {
					a.share.note = "deleting: " + err.Error()
				}
				a.listFiles()
				a.rescanMusic()
			})
		}()
	})
}

func (a *bro) filesKey(code uint16, value int32) bool {
	if value == 0 {
		return true
	}
	v := &a.fv
	move := func(d int) {
		v.sel = max(0, min(v.sel+d, len(v.ents)-1))
		a.drawFileList()
	}
	ch, _ := kobo.KeyChar(code, false, false)
	switch {
	case code == kobo.KeyEsc:
		return false
	case code == keyUp:
		move(-1)
	case code == keyDown:
		move(1)
	case code == keyPgUp:
		move(-v.rowsOn)
	case code == keyPgDn:
		move(v.rowsOn)
	case code == keyHome:
		move(-len(v.ents))
	case code == keyEnd:
		move(len(v.ents))
	case code == kobo.KeyEnter || code == keyRight:
		a.fileOpen()
	case code == kobo.KeyBackspace || code == keyLeft:
		a.fileUp()
	case code == keyDelete || ch == "d":
		a.fileDelete()
	case ch == "r":
		a.listFiles()
	case ch == "h":
		v.hidden = !v.hidden
		a.listFiles()
	case ch == "o":
		a.shareToggle()
	case ch == "p":
		a.sharePair()
	}
	return true
}

func (a *bro) filesTap(p image.Point) {
	v := &a.fv
	for _, b := range v.shareBtns {
		if kobo.Hit(p, b.r) {
			switch b.label {
			case "on/off":
				a.shareToggle()
			case "pair":
				a.sharePair()
			case "forget paired":
				a.shareForgetAll()
			}
			return
		}
	}
	for _, b := range v.listBtns {
		if kobo.Hit(p, b.r) {
			switch b.label {
			case "open":
				a.fileOpen()
			case "up":
				a.fileUp()
			case "delete":
				a.fileDelete()
			case "refresh":
				a.listFiles()
			}
			return
		}
	}
	for k, rr := range v.rows {
		if p.In(rr) {
			if v.sel == v.top+k { // a second tap opens
				a.fileOpen()
				return
			}
			v.sel = v.top + k
			a.drawFileList()
			return
		}
	}
}
