package main

// All Notebooks: the notebooks in Notebooks/ (and the folders in it),
// in a window of their own, with the books' header and tabs
// (library.go): Recent, the last written first; All, by name; Folders,
// a folder at a time. Under the header: a new notebook (in the folder
// shown) and a new folder. Each row renames it, or deletes it: tapped
// twice, it goes to Notebooks/.trash (a folder, only if it's empty). A
// name is typed in its row, the keyboard's or the one on the screen,
// which shows for it while there's no keyboard. A notebook's bar has
// "‹ All Notebooks" at its left (notebook.go); opened from one, the
// list has "back to the notebook" at its right.

import (
	"errors"
	"fmt"
	"image"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"hellobro/kobo"
)

const noteAskFor = 4 * time.Second // a delete asked: tap again this soon

type notesWin struct {
	p     kobo.Panel
	tab   int
	dir   string // the folder shown, under Notebooks/
	back  string // the notebook it was opened from, to go back to
	items []libItem
	n     int // notebooks there are, all told
	sel   int
	top   int
	listN int
	rows  []bookRow
	edit  *noteEdit // a name being typed
	asked string    // a delete asked for: what
	askAt time.Time
}

// noteEdit is a name being typed: a notebook's or a folder's new one, or
// a new folder's (key "").
type noteEdit struct {
	key  string
	dir  bool
	text string
}

func (w *notesWin) Title() string    { return "Notebooks" }
func (w *notesWin) Class() string    { return "hellobro" }
func (w *notesWin) Instance() string { return "notebooks" }

// newNotesWin is the list, opened from the notebook back ("": none).
func (a *bro) newNotesWin(back string) *notesWin {
	w := &notesWin{back: back, tab: max(0, min(libFolders, a.mem.NotesTab)), dir: a.mem.NotesDir}
	w.load()
	return w
}

// openNotes opens the list in a window.
func (a *bro) openNotes() { a.openWin(a.newNotesWin("")) }

// load reads what's in Notebooks/ again.
func (w *notesWin) load() {
	es, dirs := noteEntries()
	w.n = len(es)
	if w.dir != "" && !isNoteDir(w.dir) {
		w.dir = ""
	}
	w.items = libView(es, dirs, w.tab, w.dir)
}

func isNoteDir(dir string) bool {
	fi, err := os.Stat(filepath.Join(notesDir(), filepath.FromSlash(dir)))
	return err == nil && fi.IsDir()
}

func (w *notesWin) place(a *bro, p kobo.Panel) {
	p.Name = w.Title()
	w.p, w.rows = p, nil
}

func (w *notesWin) panel(a *bro) kobo.Panel {
	p := w.p
	p.Name = w.Title()
	return a.winPanel(p, a.focusedWin() == window(w))
}

func (w *notesWin) drawTitle(a *bro) {
	if !w.p.Outer.Empty() {
		a.DrawPanelTitle(w.panel(a), fmt.Sprintf("%d notebooks", w.n))
	}
}

// show shows tab, in folder dir; the choice at the top.
func (w *notesWin) show(a *bro, tab int, dir string) {
	w.tab, w.dir, w.sel, w.top, w.edit, w.asked = tab, dir, 0, 0, nil, ""
	a.mem.NotesTab, a.mem.NotesDir = tab, dir
	a.mem.save()
	w.load()
	w.draw(a)
}

func (w *notesWin) draw(a *bro) {
	if w.p.Outer.Empty() {
		return
	}
	pn := w.panel(a)
	a.DrawPanel(pn, fmt.Sprintf("%d notebooks", w.n))
	c := a.C
	rowH := a.libRowH()
	area := w.p.Body.Inset(kobo.Pad)
	back := ""
	if w.back != "" {
		back = "back to the notebook"
	}
	h := a.drawLibHeader(image.Rect(area.Min.X, area.Min.Y, area.Max.X, area.Min.Y+rowH), "All Notebooks", true,
		func() { w.show(a, w.tab, "") }, back, func() { w.toNotebook(a, w.back) },
		libTabs(w.tab, func(k int) { w.show(a, k, w.dir) }))
	w.rows = h.rows
	area.Min.Y += rowH + 12
	// what's made here: a notebook, a folder
	x := area.Min.X
	for _, b := range []libTab{{"+ new notebook", false, func() { w.newNotebook(a) }}, {"+ new folder", false, func() { w.startEdit(a, "", true, "") }}} {
		r := image.Rect(x, area.Min.Y, x+kobo.TextWidth(a.F.Panel, b.name)+48, area.Min.Y+rowH-12)
		c.Frame(r, 2, kobo.Black)
		c.Text(a.F.Panel, r, b.name, kobo.Black, kobo.AlignCenter)
		w.rows = append(w.rows, bookRow{r, b.do})
		x = r.Max.X + 16
	}
	if w.tab == libFolders && w.dir != "" {
		c.Text(a.F.Small, image.Rect(x+16, area.Min.Y, area.Max.X, area.Min.Y+rowH-12), "in "+w.dir, kobo.Mid, kobo.AlignLeft)
	}
	area.Min.Y += rowH
	items := w.items
	if e := w.edit; e != nil && e.key == "" { // a new folder's name: at the top
		items = append([]libItem{{dir: true}}, items...)
	}
	n := max(1, area.Dy()/rowH)
	w.listN = n
	if len(items) == 0 {
		c.Text(a.F.Body, image.Rect(area.Min.X, area.Min.Y, area.Max.X, area.Min.Y+rowH), "no notebooks yet: + new notebook makes one", kobo.Dark, kobo.AlignLeft)
		a.Mark(pn.Outer, true)
		return
	}
	w.sel = max(0, min(w.sel, len(items)-1))
	if w.sel < w.top {
		w.top = w.sel
	} else if w.sel >= w.top+n {
		w.top = w.sel - n + 1
	}
	w.top = max(0, min(w.top, len(items)-n))
	for k := 0; k < n && w.top+k < len(items); k++ {
		i := w.top + k
		it := items[i]
		r := image.Rect(area.Min.X, area.Min.Y+k*rowH, area.Max.X, area.Min.Y+(k+1)*rowH)
		if e := w.edit; e != nil && e.key == it.key && e.dir == it.dir && !it.up {
			w.drawEdit(a, r)
			continue
		}
		var btns []libTab
		if !it.up {
			del := "×"
			if w.asking(it.key) {
				del = "delete?"
			}
			btns = []libTab{{"rename", false, func() { w.startEdit(a, it.key, it.dir, path.Base(it.key)) }}, {del, false, func() { w.remove(a, it) }}}
		}
		w.rows = append(w.rows, a.drawLibRow(r, it, i == w.sel, func() { w.open(a, it) }, btns...)...)
	}
	a.Mark(pn.Outer, true)
}

// drawEdit draws the name being typed in r, with ok and cancel.
func (w *notesWin) drawEdit(a *bro, r image.Rectangle) {
	c := a.C
	e := w.edit
	bw := r.Dy() + 64
	ok := image.Rect(r.Max.X-2*bw-8, r.Min.Y, r.Max.X-bw-8, r.Max.Y).Inset(6)
	no := image.Rect(r.Max.X-bw, r.Min.Y, r.Max.X, r.Max.Y).Inset(6)
	box := image.Rect(r.Min.X, r.Min.Y+4, ok.Min.X-12, r.Max.Y-4)
	c.Fill(box, kobo.White)
	c.Frame(box, 3, kobo.Black)
	prompt := "name: "
	if e.key == "" {
		prompt = "new folder: "
	}
	t := box.Inset(12)
	pw := kobo.TextWidth(a.F.Small, prompt)
	c.Text(a.F.Small, image.Rect(t.Min.X, t.Min.Y, t.Min.X+pw, t.Max.Y), prompt, kobo.Mid, kobo.AlignLeft)
	t.Min.X += pw
	c.Text(a.F.Body, t, e.text, kobo.Black, kobo.AlignLeft)
	cx := min(t.Max.X-3, t.Min.X+kobo.TextWidth(a.F.Body, e.text)+2) // the caret
	c.Fill(image.Rect(cx, t.Min.Y+4, cx+3, t.Max.Y-4), kobo.Black)
	for _, b := range []struct {
		r     image.Rectangle
		label string
		do    func()
	}{{ok, "ok", func() { w.endEdit(a, true) }}, {no, "cancel", func() { w.endEdit(a, false) }}} {
		c.Frame(b.r, 2, kobo.Black)
		c.Text(a.F.Panel, b.r, b.label, kobo.Black, kobo.AlignCenter)
		w.rows = append(w.rows, bookRow{b.r, b.do})
	}
	w.rows = append(w.rows, bookRow{box, func() {}})
}

// open opens the folder, or the notebook in this window (or focuses
// it, if it's open elsewhere).
func (w *notesWin) open(a *bro, it libItem) {
	if it.dir {
		w.show(a, libFolders, it.key)
		return
	}
	w.toNotebook(a, it.key)
}

// toNotebook puts the notebook name in the list's place; if it's open
// in another window, the list goes and that one's focused.
func (w *notesWin) toNotebook(a *bro, name string) {
	if name == "" {
		return
	}
	if a.noteWindow(name) != nil {
		a.closeWin(w)
		a.openNotebook(name)
		return
	}
	if nw := a.noteOpen(name); nw != nil {
		a.replaceWin(w, nw)
	}
}

// notesFrom puts the list in the notebook's place, to come back to it.
func (a *bro) notesFrom(nw *noteWin) {
	nw.keep(a)
	l := a.newNotesWin(nw.nb.name)
	a.replaceWin(nw, l)
}

// newNotebook makes a notebook in the folder shown (or the top one) and
// opens it here.
func (w *notesWin) newNotebook(a *bro) {
	dir := ""
	if w.tab == libFolders {
		dir = w.dir
	}
	name := newNoteName(time.Now(), func(n string) bool {
		n = path.Join(dir, n)
		_, err := os.Stat(notePath(n))
		return err == nil || a.noteWindow(n) != nil
	})
	if nw := a.noteOpen(path.Join(dir, name)); nw != nil {
		a.replaceWin(w, nw)
	}
}

// startEdit types a new name for key (a notebook's, or a folder's if
// dir), or a new folder's ("" key); the keyboard on the screen shows
// for it if there's no other.
func (w *notesWin) startEdit(a *bro, key string, dir bool, text string) {
	w.edit, w.asked = &noteEdit{key: key, dir: dir, text: text}, ""
	if key == "" {
		w.sel = 0
	}
	a.vk.ask = true
	a.vkApply()
	w.draw(a)
}

// endEdit takes the name typed (ok) or leaves it.
func (w *notesWin) endEdit(a *bro, ok bool) {
	e := w.edit
	w.edit = nil
	a.vk.ask = false
	defer a.vkApply()
	if ok && e != nil {
		if err := w.rename(a, e); err != nil {
			a.Notify(err.Error())
		}
	}
	w.load()
	w.draw(a)
}

// noteNameOK says what's wrong with a name typed, if anything.
func noteNameOK(name string) error {
	switch {
	case name == "":
		return errors.New("a name, please")
	case strings.ContainsAny(name, `/\:*?"<>|`):
		return errors.New(`a name without / \ : * ? " < > |`)
	case strings.HasPrefix(name, "."):
		return errors.New("a name not starting with a dot")
	}
	return nil
}

// rename does what e asks: a folder made, or a notebook or a folder
// renamed. Open notebooks follow.
func (w *notesWin) rename(a *bro, e *noteEdit) error {
	name := strings.TrimSpace(e.text)
	if err := noteNameOK(name); err != nil {
		return err
	}
	if e.key == "" { // a new folder, in the one shown
		dir := path.Join(w.folder(), name)
		if err := os.MkdirAll(filepath.Join(notesDir(), filepath.FromSlash(dir)), 0o755); err != nil {
			return err
		}
		log.Printf("notebooks: folder %s made", dir)
		return nil
	}
	to := path.Join(path.Dir(e.key), name)
	if to == e.key {
		return nil
	}
	if e.dir {
		from, dst := filepath.Join(notesDir(), filepath.FromSlash(e.key)), filepath.Join(notesDir(), filepath.FromSlash(to))
		if _, err := os.Stat(dst); err == nil {
			return fmt.Errorf("there's a %s already", name)
		}
		for _, nw := range a.noteWins() { // what's open in it, kept first
			if strings.HasPrefix(nw.nb.name, e.key+"/") {
				nw.keep(a)
			}
		}
		if err := os.Rename(from, dst); err != nil {
			return err
		}
		for _, nw := range a.noteWins() {
			if rest, ok := strings.CutPrefix(nw.nb.name, e.key+"/"); ok {
				nw.nb.name = to + "/" + rest
				nw.drawTitle(a)
			}
		}
		a.wm.changed = time.Now() // the layout names them
		if w.dir == e.key || strings.HasPrefix(w.dir, e.key+"/") {
			w.dir = to + strings.TrimPrefix(w.dir, e.key)
			a.mem.NotesDir = w.dir
			a.mem.save()
		}
		log.Printf("notebooks: folder %s is %s now", e.key, to)
		return nil
	}
	if err := renameNotebook(a, e.key, to); err != nil {
		return err
	}
	if w.back == e.key {
		w.back = to
	}
	log.Printf("notebooks: %s is %s now", e.key, to)
	return nil
}

// renameNotebook renames the notebook from as to: its file, its export,
// and its window if it's open.
func renameNotebook(a *bro, from, to string) error {
	if _, err := os.Stat(notePath(to)); err == nil || a.noteWindow(to) != nil {
		return fmt.Errorf("there's a %s already", path.Base(to))
	}
	nw := a.noteWindow(from)
	if nw != nil {
		nw.keep(a)
	}
	if _, err := os.Stat(notePath(from)); err == nil {
		if err := os.Rename(notePath(from), notePath(to)); err != nil {
			return err
		}
	} else { // an .hbnote's, or not written yet: written as it's to be
		nb, err := loadNotebook(from)
		if err != nil {
			return err
		}
		nb.name = to
		if err := nb.save(); err != nil {
			return err
		}
	}
	if pdf := filepath.Join(notesDir(), filepath.FromSlash(from)+".pdf"); fileExists(pdf) { // its export
		os.Rename(pdf, filepath.Join(notesDir(), filepath.FromSlash(to)+".pdf"))
	}
	if nw != nil {
		nw.nb.name, nw.nb.old = to, ""
		nw.drawTitle(a)
		a.wm.changed = time.Now()
	}
	return nil
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// folder is where what's made goes: the folder shown, if it's Folders.
func (w *notesWin) folder() string {
	if w.tab == libFolders {
		return w.dir
	}
	return ""
}

// asking is whether key's delete is asked for, and waits for the
// second tap.
func (w *notesWin) asking(key string) bool {
	return w.asked == key && time.Since(w.askAt) < noteAskFor
}

// remove asks first; asked, it moves the notebook to Notebooks/.trash
// (closing its window) or takes the folder away, if it's empty.
func (w *notesWin) remove(a *bro, it libItem) {
	if !w.asking(it.key) {
		w.asked, w.askAt, w.edit = it.key, time.Now(), nil
		w.draw(a)
		return
	}
	w.asked = ""
	if err := removeNote(a, it); err != nil {
		a.Notify(err.Error())
	}
	w.load()
	w.draw(a)
}

func removeNote(a *bro, it libItem) error {
	if it.dir {
		if err := os.Remove(filepath.Join(notesDir(), filepath.FromSlash(it.key))); err != nil {
			return errors.New("only an empty folder goes: " + it.name + " has notebooks in it")
		}
		log.Printf("notebooks: folder %s taken away", it.key)
		return nil
	}
	if nw := a.noteWindow(it.key); nw != nil {
		a.closeWin(nw) // kept first, then it goes
	}
	trash := filepath.Join(notesDir(), ".trash")
	if err := os.MkdirAll(trash, 0o755); err != nil {
		return err
	}
	base := strings.ReplaceAll(it.key, "/", " · ")
	dst := filepath.Join(trash, base+noteExt)
	for i := 2; fileExists(dst); i++ {
		dst = filepath.Join(trash, fmt.Sprintf("%s (%d)%s", base, i, noteExt))
	}
	switch {
	case fileExists(notePath(it.key)):
		if err := os.Rename(notePath(it.key), dst); err != nil {
			return err
		}
	case fileExists(filepath.Join(oldNotesDir(), it.key+noteOldExt)):
		if err := os.Rename(filepath.Join(oldNotesDir(), it.key+noteOldExt), strings.TrimSuffix(dst, noteExt)+noteOldExt); err != nil {
			return err
		}
	}
	log.Printf("notebooks: %s to the trash", it.key)
	a.Notify("deleted " + it.name + " (it's in Notebooks/.trash)")
	return nil
}

// press: ↑ ↓ PgUp PgDn and Enter; Tab the next tab, Backspace up a
// folder; n a new notebook, F2 rename, Delete delete (twice); Esc back
// to the notebook. A name being typed takes the keys.
func (w *notesWin) press(a *bro, code uint16, value int32) bool {
	if value == 0 {
		return false
	}
	m := &a.tv.mods
	if m.ctrl || m.alt || m.meta {
		return false
	}
	ch, _ := kobo.KeyChar(code, m.shift, m.caps)
	if e := w.edit; e != nil {
		switch {
		case code == kobo.KeyEnter:
			w.endEdit(a, true)
		case code == kobo.KeyEsc:
			w.endEdit(a, false)
		case code == kobo.KeyBackspace:
			if r := []rune(e.text); len(r) > 0 {
				e.text = string(r[:len(r)-1])
			}
			w.draw(a)
		case ch != "" && ch != "\t" && ch != "\n":
			e.text += ch
			w.draw(a)
		}
		return true
	}
	it, ok := libItem{}, w.sel < len(w.items)
	if ok {
		it = w.items[w.sel]
	}
	switch {
	case code == 103 || ch == "k":
		w.sel--
	case code == 108 || ch == "j":
		w.sel++
	case code == 104:
		w.sel -= max(1, w.listN-1)
	case code == 109:
		w.sel += max(1, w.listN-1)
	case code == kobo.KeyEnter && ok:
		w.open(a, it)
		return true
	case code == kobo.KeyTab:
		w.show(a, (w.tab+1)%len(libTabNames), w.dir)
		return true
	case code == kobo.KeyBackspace && w.tab == libFolders && w.dir != "":
		w.open(a, w.items[0])
		return true
	case ch == "n":
		w.newNotebook(a)
		return true
	case code == 60 && ok && !it.up: // F2
		w.startEdit(a, it.key, it.dir, path.Base(it.key))
		return true
	case code == 111 && ok && !it.up: // Delete
		w.remove(a, it)
		return true
	case code == kobo.KeyEsc && w.back != "":
		w.toNotebook(a, w.back)
		return true
	default:
		return false
	}
	w.draw(a)
	return true
}

func (w *notesWin) tap(a *bro, p image.Point) {
	for _, r := range w.rows {
		if p.In(r.r) {
			r.do()
			return
		}
	}
}

// swipe up or down goes a screenful on or back.
func (w *notesWin) swipe(a *bro, from, to image.Point) {
	d := to.Sub(from)
	if kobo.Abs(d.Y) < 60 || kobo.Abs(d.Y) < 2*kobo.Abs(d.X) {
		return
	}
	n := max(1, w.listN-1)
	if d.Y > 0 {
		n = -n
	}
	w.sel += n
	w.top += n
	w.draw(a)
}

func (w *notesWin) closing(a *bro) {
	if w.edit != nil {
		w.edit = nil
		a.vk.ask = false
		a.vkApply()
	}
}
