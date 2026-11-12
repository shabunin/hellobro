package main

// All Notebooks: the notebooks in Notebooks/ (and the folders in it),
// in a window of their own, with the books' header and tabs
// (library.go): Recent, the last written first; All, by name; Folders,
// a folder at a time. Under the header: a new notebook (in the folder
// shown) and a new folder. A row held is picked, and the rows get boxes
// to tick: the ones picked are copied, cut or deleted (to
// Notebooks/.trash, asked first), one alone renamed. Copied or cut,
// "paste here" puts them in the folder shown. A
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
	"slices"
	"strings"
	"time"

	"hellobro/kobo"
)

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
	lib   *libUI    // the list, in ink

	picking bool      // the rows have boxes: a row held, or Space
	picks   []libItem // the ones ticked, in the order they were
	clip    *noteClip // copied or cut, to paste in a folder
}

// noteClip is what's copied or cut from the list, to paste.
type noteClip struct {
	items []libItem
	cut   bool
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
	w.picks = slices.DeleteFunc(w.picks, func(p libItem) bool { // gone meanwhile
		return !slices.ContainsFunc(w.items, func(it libItem) bool { return it.key == p.key && it.dir == p.dir && !it.up })
	})
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
	if dir != w.dir || tab != w.tab { // what's picked is here only
		w.picking, w.picks = false, nil
	}
	w.tab, w.dir, w.sel, w.top, w.edit = tab, dir, 0, 0, nil
	a.mem.NotesTab, a.mem.NotesDir = tab, dir
	a.mem.save()
	w.load()
	w.update(a)
}

func (w *notesWin) draw(a *bro) {
	if w.p.Outer.Empty() {
		return
	}
	pn := w.panel(a)
	a.DrawPanel(pn, fmt.Sprintf("%d notebooks", w.n))
	w.libOf(a).put(a, w.p.Body, true)
	a.Mark(pn.Outer, true)
}

// libOf is the list's ink, made the first time.
func (w *notesWin) libOf(a *bro) *libUI {
	if w.lib == nil {
		w.lib = newLibUI(a, func() libSpec { return w.libSpec(a) })
	}
	return w.lib
}

func (w *notesWin) inkBodies(a *bro) []inkBody {
	if b := w.inkBody(a); b != nil {
		return []inkBody{b}
	}
	return nil
}

func (w *notesWin) inkBody(a *bro) inkBody {
	if w.lib != nil && !w.p.Outer.Empty() && a.shows(w) && w.lib.pane.At == w.p.Body {
		return w.lib
	}
	return nil
}

// update draws what changed of the list, if it shows; else all of it.
func (w *notesWin) update(a *bro) {
	if w.inkBody(a) != nil && !a.overlay() {
		w.lib.put(a, w.p.Body, false)
		return
	}
	w.draw(a)
}

// libSpec is the list as it is: under the header, a new notebook (in
// the folder shown) and a new folder; picking, what's done to the ones
// picked; copied or cut, paste here.
func (w *notesWin) libSpec(a *bro) libSpec {
	back := ""
	if w.back != "" {
		back = "back to the notebook"
	}
	s := libSpec{
		id: fmt.Sprint(w.tab, w.dir), sel: &w.sel, shown: &w.listN, edit: -1,
		head: libHeadSpec{all: "All Notebooks", here: true, toAll: func() { w.show(a, w.tab, "") },
			back: back, toBack: func() { w.toNotebook(a, w.back) }, tabs: libTabs(w.tab, func(k int) { w.show(a, k, w.dir) })},
		tools: []libTab{{"+ new notebook", false, func() { w.newNotebook(a) }}, {"+ new folder", false, func() { w.startEdit(a, "", true, "") }}},
		empty: "no notebooks yet: + new notebook makes one",
	}
	where := "the top"
	if f := w.folder(); f != "" {
		where = f
	}
	switch {
	case w.picking:
		s.tools = nil
		if len(w.picks) == 1 {
			s.tools = append(s.tools, libTab{"rename", false, func() { w.renamePicked(a) }})
		}
		if len(w.picks) > 0 {
			s.tools = append(s.tools, libTab{"copy", false, func() { w.clipPicked(a, false) }}, libTab{"cut", false, func() { w.clipPicked(a, true) }},
				libTab{"delete", false, func() { w.deletePicked(a) }})
		}
		s.tools = append(s.tools, libTab{"cancel", false, func() { w.pickNone(a) }})
		s.toolNote = fmt.Sprintf("%d picked", len(w.picks))
	case w.clip != nil:
		s.tools = []libTab{{"paste here", false, func() { w.paste(a) }}, {"cancel", false, func() { w.clip = nil; w.update(a) }},
			{"+ new folder", false, func() { w.startEdit(a, "", true, "") }}}
		what := "copied"
		if w.clip.cut {
			what = "cut"
		}
		s.toolNote = fmt.Sprintf("%d %s: paste in %s, or open a folder (Folders)", len(w.clip.items), what, where)
	case w.tab == libFolders && w.dir != "":
		s.toolNote = "in " + w.dir + " · hold one to pick it"
	default:
		s.toolNote = "hold one to pick it: copy, cut, delete, rename"
	}
	items := w.items
	if e := w.edit; e != nil {
		if e.key == "" { // a new folder's name: at the top
			items = append([]libItem{{dir: true}}, items...)
			s.edit = 0
		}
		for i, it := range items {
			if s.edit < 0 && e.key == it.key && e.dir == it.dir && !it.up {
				s.edit = i
			}
		}
		s.prompt, s.typed = "name:", e.text
		if e.key == "" {
			s.prompt = "new folder:"
		}
		s.editOK, s.editNo = func() { w.endEdit(a, true) }, func() { w.endEdit(a, false) }
	}
	s.items = items
	s.picking = w.picking
	s.picked = func(i int) bool { return w.isPicked(items[i]) }
	s.open = func(i int) {
		if w.picking && !items[i].up {
			w.pick(a, items[i])
			return
		}
		w.open(a, items[i])
	}
	s.hold = func(i int) {
		if !items[i].up && w.edit == nil {
			w.picking = true
			w.pick(a, items[i])
		}
	}
	return s
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

func (w *notesWin) isPicked(it libItem) bool {
	return !it.up && slices.ContainsFunc(w.picks, func(p libItem) bool { return p.key == it.key && p.dir == it.dir })
}

// pick ticks it, or unticks it; none left ticked, the boxes stay till
// cancel.
func (w *notesWin) pick(a *bro, it libItem) {
	if w.isPicked(it) {
		w.picks = slices.DeleteFunc(w.picks, func(p libItem) bool { return p.key == it.key && p.dir == it.dir })
	} else {
		w.picks = append(w.picks, it)
	}
	w.picking = true
	w.update(a)
}

// pickNone leaves picking.
func (w *notesWin) pickNone(a *bro) {
	w.picking, w.picks = false, nil
	w.update(a)
}

func (w *notesWin) renamePicked(a *bro) {
	if len(w.picks) != 1 {
		return
	}
	it := w.picks[0]
	w.picking, w.picks = false, nil
	w.startEdit(a, it.key, it.dir, path.Base(it.key))
}

// clipPicked copies or cuts what's picked, to paste in a folder.
func (w *notesWin) clipPicked(a *bro, cut bool) {
	w.clip = &noteClip{items: w.picks, cut: cut}
	w.picking, w.picks = false, nil
	w.update(a)
}

// paste puts what's copied or cut in the folder shown: copies, named
// "(copy)" if there's one so named; or moved, open notebooks following.
func (w *notesWin) paste(a *bro) {
	c := w.clip
	if c == nil {
		return
	}
	w.clip = nil
	dir := w.folder()
	n, last := 0, ""
	var errs []string
	for _, it := range c.items {
		to := path.Join(dir, path.Base(it.key))
		var err error
		switch {
		case c.cut && to == it.key: // it's here already
			continue
		case c.cut:
			err = w.moveTo(a, it.key, it.dir, to)
		default:
			to, err = copyNote(a, it, dir)
		}
		if err != nil {
			errs = append(errs, path.Base(it.key)+": "+err.Error())
			continue
		}
		n, last = n+1, to
	}
	if len(errs) > 0 {
		a.Notify(strings.Join(errs, "; "))
	} else if n > 0 {
		a.Notify(fmt.Sprintf("%d %s here", n, map[bool]string{true: "moved", false: "copied"}[c.cut]))
	}
	w.load()
	for i, it := range w.items {
		if it.key == last && !it.up {
			w.sel = i
		}
	}
	w.draw(a)
}

// deletePicked asks, then puts what's picked in Notebooks/.trash.
func (w *notesWin) deletePicked(a *bro) {
	its := slices.Clone(w.picks)
	if len(its) == 0 {
		return
	}
	what := its[0].name
	if len(its) > 1 {
		what = fmt.Sprintf("%d of them", len(its))
	}
	say := "they go to Notebooks/.trash, to be taken back from there"
	if slices.ContainsFunc(its, func(it libItem) bool { return it.dir }) {
		say = "they go to Notebooks/.trash, a folder with all in it, to be taken back from there"
	}
	a.choose("Delete "+what+"?", say, askOpt{"delete", "d", func() {
		var errs []string
		for _, it := range its {
			if err := removeNote(a, it); err != nil {
				errs = append(errs, err.Error())
			} else if w.back == it.key || it.dir && strings.HasPrefix(w.back, it.key+"/") { // nothing to go back to now
				w.back = ""
			}
		}
		if len(errs) > 0 {
			a.Notify(strings.Join(errs, "; "))
		} else if len(its) > 1 {
			a.Notify(fmt.Sprintf("deleted %d (they're in Notebooks/.trash)", len(its)))
		}
		w.picking, w.picks = false, nil
		w.load()
		w.draw(a)
	}}, askOpt{"cancel", "", nil})
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
	a.newNotebookWiz(w, dir) // plain, or the AI's (notewizard.go)
}

// startEdit types a new name for key (a notebook's, or a folder's if
// dir), or a new folder's ("" key); the keyboard on the screen shows
// for it if there's no other.
func (w *notesWin) startEdit(a *bro, key string, dir bool, text string) {
	w.edit = &noteEdit{key: key, dir: dir, text: text}
	if key == "" {
		w.sel = 0
	}
	a.vkAsk(true)
	w.draw(a)
}

// endEdit takes the name typed (ok) or leaves it.
func (w *notesWin) endEdit(a *bro, ok bool) {
	e := w.edit
	w.edit = nil
	defer a.vkAsk(false)
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
	return w.moveTo(a, e.key, e.dir, path.Join(path.Dir(e.key), name))
}

// moveTo renames the notebook key, or the folder (dir), as to: in
// another folder, or the same one. Open notebooks follow.
func (w *notesWin) moveTo(a *bro, key string, dir bool, to string) error {
	if to == key {
		return nil
	}
	if dir {
		if strings.HasPrefix(to, key+"/") {
			return errors.New("a folder doesn't go into itself")
		}
		name := path.Base(to)
		from, dst := filepath.Join(notesDir(), filepath.FromSlash(key)), filepath.Join(notesDir(), filepath.FromSlash(to))
		if _, err := os.Stat(dst); err == nil {
			return fmt.Errorf("there's a %s already", name)
		}
		for _, nw := range a.noteWins() { // what's open in it, kept first
			if strings.HasPrefix(nw.nb.name, key+"/") {
				nw.keep(a)
			}
		}
		if err := os.Rename(from, dst); err != nil {
			return err
		}
		for _, nw := range a.noteWins() {
			if rest, ok := strings.CutPrefix(nw.nb.name, key+"/"); ok {
				nw.nb.name = to + "/" + rest
				nw.drawTitle(a)
			}
		}
		for _, nb := range a.nt.unsaved {
			if rest, ok := strings.CutPrefix(nb.name, key+"/"); ok {
				nb.name = to + "/" + rest
			}
		}
		if rest, ok := strings.CutPrefix(w.back, key+"/"); ok { // the notebook to go back to went with it
			w.back = to + "/" + rest
		}
		a.wm.changed = time.Now() // the layout names them
		if w.dir == key || strings.HasPrefix(w.dir, key+"/") {
			w.dir = to + strings.TrimPrefix(w.dir, key)
			a.mem.NotesDir = w.dir
			a.mem.save()
		}
		log.Printf("notebooks: folder %s is %s now", key, to)
		return nil
	}
	if err := renameNotebook(a, key, to); err != nil {
		return err
	}
	if w.back == key {
		w.back = to
	}
	log.Printf("notebooks: %s is %s now", key, to)
	return nil
}

// renameNotebook renames the notebook from as to: its file, its export,
// and its window if it's open.
func renameNotebook(a *bro, from, to string) error {
	if _, err := os.Stat(notePath(to)); err == nil || a.noteWindow(to) != nil {
		return fmt.Errorf("there's a %s already", path.Base(to))
	}
	nw := a.noteWindow(from)
	if nw != nil && !nw.keep(a) {
		return errors.New("it can't be written now, so it's not renamed")
	}
	if nb := a.unsavedNote(from); nb != nil { // closed unkept: it's kept as it's to be called, when it can be
		nb.name = to
		if fileExists(notePath(from)) {
			os.Rename(notePath(from), notePath(to))
		}
		return nil
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
	top := filepath.Join(notesDir(), filepath.FromSlash(to))
	for after, p := range exportsOf(from) { // its exports, and only those
		if !fileExists(top + after) {
			os.Rename(p, top+after)
		}
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

func removeNote(a *bro, it libItem) error {
	trash := filepath.Join(notesDir(), ".trash")
	if err := os.MkdirAll(trash, 0o755); err != nil {
		return err
	}
	if it.dir { // all in it, to the trash as it is
		for _, nw := range a.noteWins() {
			if strings.HasPrefix(nw.nb.name, it.key+"/") {
				a.closeWin(nw) // kept first
			}
		}
		a.nt.unsaved = slices.DeleteFunc(a.nt.unsaved, func(n *notebook) bool { return strings.HasPrefix(n.name, it.key+"/") })
		base := strings.ReplaceAll(it.key, "/", " · ")
		dst := filepath.Join(trash, base)
		for i := 2; fileExists(dst); i++ {
			dst = filepath.Join(trash, fmt.Sprintf("%s (%d)", base, i))
		}
		if err := os.Rename(filepath.Join(notesDir(), filepath.FromSlash(it.key)), dst); err != nil {
			return err
		}
		log.Printf("notebooks: folder %s to the trash", it.key)
		a.Notify("deleted " + it.name + " (it's in Notebooks/.trash)")
		return nil
	}
	if nw := a.noteWindow(it.key); nw != nil {
		a.closeWin(nw) // kept first, then it goes
	}
	if nb := a.unsavedNote(it.key); nb != nil { // it's not to be kept now
		a.nt.unsaved = slices.DeleteFunc(a.nt.unsaved, func(n *notebook) bool { return n == nb })
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

// copyNote copies the notebook or the folder it into the folder dir:
// named as it is, or "(copy)" after if that's taken. It says what the
// copy is.
func copyNote(a *bro, it libItem, dir string) (string, error) {
	base := path.Base(it.key)
	taken := func(to string) bool {
		if it.dir {
			return fileExists(filepath.Join(notesDir(), filepath.FromSlash(to)))
		}
		return fileExists(notePath(to)) || a.noteWindow(to) != nil || a.unsavedNote(to) != nil
	}
	to := path.Join(dir, base)
	for i := 1; taken(to); i++ {
		to = path.Join(dir, base+" (copy)")
		if i > 1 {
			to = path.Join(dir, fmt.Sprintf("%s (copy %d)", base, i))
		}
	}
	if it.dir {
		if dir == it.key || strings.HasPrefix(dir, it.key+"/") {
			return "", errors.New("a folder doesn't go into itself")
		}
		for _, nw := range a.noteWins() { // what's open in it, kept first
			if strings.HasPrefix(nw.nb.name, it.key+"/") {
				nw.keep(a)
			}
		}
		if err := copyTree(filepath.Join(notesDir(), filepath.FromSlash(it.key)), filepath.Join(notesDir(), filepath.FromSlash(to))); err != nil {
			return "", err
		}
		log.Printf("notebooks: folder %s copied to %s", it.key, to)
		return to, nil
	}
	if nw := a.noteWindow(it.key); nw != nil {
		nw.keep(a)
	}
	var err error
	switch nb := a.unsavedNote(it.key); {
	case nb != nil: // newer than its file
		var b []byte
		if b, err = nb.xopp(); err == nil {
			err = writeAtomic(notePath(to), b)
		}
	case fileExists(notePath(it.key)):
		err = copyFile(notePath(it.key), notePath(to))
	default: // an .hbnote: written as a .xopp
		var nb *notebook
		if nb, err = loadNotebook(it.key); err == nil {
			nb.name, nb.old = to, ""
			err = nb.save()
		}
	}
	if err != nil {
		return "", err
	}
	log.Printf("notebooks: %s copied to %s", it.key, to)
	return to, nil
}

func copyFile(from, to string) error {
	b, err := os.ReadFile(from)
	if err != nil {
		return err
	}
	return writeAtomic(to, b)
}

// copyTree copies the folder from, all in it, as to.
func copyTree(from, to string) error {
	return filepath.WalkDir(from, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(from, p)
		dst := filepath.Join(to, rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		return copyFile(p, dst)
	})
}

// press: ↑ ↓ PgUp PgDn and Enter; Tab the next tab, Backspace up a
// folder; n a new notebook. Space picks the row (or not), Ctrl+C,
// Ctrl+X copy or cut what's picked (the row, if none), Ctrl+V pastes,
// F2 renames, Delete deletes (asked); Esc leaves picking, a copy, or
// goes back to the notebook. A name being typed takes the keys.
func (w *notesWin) press(a *bro, code uint16, value int32) bool {
	if value == 0 {
		return false
	}
	m := &a.tv.mods
	ch, _ := kobo.KeyChar(code, m.shift, m.caps)
	if e := w.edit; e != nil {
		if m.ctrl || m.alt || m.meta {
			return false
		}
		if done, ok := nameKey(&e.text, code, ch); done {
			w.endEdit(a, ok)
		} else {
			w.update(a)
		}
		return true
	}
	it, ok := libItem{}, w.sel < len(w.items)
	if ok {
		it = w.items[w.sel]
	}
	ok = ok && !it.up
	orRow := func() bool { // what's picked, or the row if nothing is
		if len(w.picks) == 0 && ok {
			w.picking, w.picks = true, []libItem{it}
		}
		return len(w.picks) > 0
	}
	if m.alt || m.meta {
		return false
	}
	if m.ctrl {
		switch code {
		case keyC, keyX:
			if orRow() {
				w.clipPicked(a, code == keyX)
			}
		case keyV:
			w.paste(a)
		default:
			return false
		}
		return true
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
	case code == kobo.KeyEnter && w.sel < len(w.items):
		w.open(a, w.items[w.sel])
		return true
	case code == kobo.KeySpace && ok:
		w.pick(a, it)
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
	case code == 60: // F2
		switch {
		case len(w.picks) == 1:
			w.renamePicked(a)
		case len(w.picks) == 0 && ok:
			w.startEdit(a, it.key, it.dir, path.Base(it.key))
		}
		return true
	case code == 111: // Delete
		if orRow() {
			w.deletePicked(a)
		}
		return true
	case code == kobo.KeyEsc && w.picking:
		w.pickNone(a)
		return true
	case code == kobo.KeyEsc && w.clip != nil:
		w.clip = nil
	case code == kobo.KeyEsc && w.back != "":
		w.toNotebook(a, w.back)
		return true
	default:
		return false
	}
	w.update(a)
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

func (w *notesWin) closing(a *bro) {
	if w.edit != nil {
		w.edit = nil
		a.vkAsk(false)
	}
}
