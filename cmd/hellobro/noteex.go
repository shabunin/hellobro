package main

// A notebook's ":" commands (excmd.go): its pages, the tools, the paper,
// keeping and exporting it, another notebook, its name and its folder.

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

func (w *noteWin) exCmds(a *bro) []exCmd {
	page := func(id string) func(string) error { // an edit of the page shown, as the pages' view's buttons do
		return func(string) error {
			if !w.overview {
				w.ovSel = w.cur
			}
			if w.pageEdit(id) {
				w.cur = min(w.ovSel, len(w.nb.pages)-1)
				w.ovShow()
				w.place(a, w.p) // the counter
			}
			return nil
		}
	}
	tool := func(id string) func(string) error {
		return func(string) error { w.setTool(a, id); return nil }
	}
	var cmds []exCmd
	cmds = []exCmd{
		{"page p", "N", func(arg string) error {
			n, pct, err := exNumber(arg)
			if pct {
				n = max(1, n*len(w.nb.pages)/100)
			}
			if err != nil || n < 1 || n > len(w.nb.pages) {
				return fmt.Errorf("a page from 1 to %d", len(w.nb.pages))
			}
			w.openPage(a, n-1)
			return nil
		}},
		{"new", "", page("new after")},
		{"copy", "", page("copy page")},
		{"delete", "", page("delete")},
		{"clear", "", page("clear")},
		{"pen", "", tool("pen")},
		{"eraser", "", tool("eraser")},
		{"select", "", tool("select")},
		{"undo u", "", func(string) error { w.doUndo(a, true); return nil }},
		{"redo", "", func(string) error { w.doUndo(a, false); return nil }},
		{"paper", strings.ReplaceAll(notePapers, " ", "|"), func(arg string) error {
			if !strings.Contains(" "+notePapers+" ", " "+arg+" ") || arg == "" {
				return errors.New("paper: " + strings.ReplaceAll(notePapers, " ", ", "))
			}
			w.nb.paper = arg
			w.changed = time.Now()
			w.place(a, w.p) // its button says another word
			return nil
		}},
		{"pages o", "", func(string) error { w.overviewOn(a, !w.overview); return nil }},
		{"export", "", func(string) error { w.export(a); return nil }},
		{"write w", "", func(string) error {
			if !w.keep(a) {
				return errors.New("it can't be written now: it's kept as soon as it can be")
			}
			a.Notify(w.nb.name + " written")
			return nil
		}},
		{"edit e open", "name", func(arg string) error { return w.exOpen(a, arg) }},
		{"rename", "name", func(arg string) error {
			if err := noteNameOK(arg); err != nil {
				return err
			}
			return renameNotebook(a, w.nb.name, path.Join(path.Dir(w.nb.name), arg))
		}},
		{"move mv", "folder", func(arg string) error {
			dir := strings.Trim(arg, "/")
			for _, d := range strings.Split(dir, "/") {
				if err := noteNameOK(d); err != nil && dir != "" {
					return err
				}
			}
			if err := os.MkdirAll(filepath.Join(notesDir(), filepath.FromSlash(dir)), 0o755); err != nil {
				return err
			}
			return renameNotebook(a, w.nb.name, path.Join(dir, path.Base(w.nb.name)))
		}},
		{"list ls all", "", func(string) error { a.notesFrom(w); return nil }},
		{"quit q close wq x", "", func(string) error { a.closeWin(w); return nil }},
	}
	return append(cmds, exCmd{"help h", "", exHelp(a, cmds)})
}

// exOpen opens the notebook named so in this window: its name as it's
// listed, or the one name begins; none so named, a new one, beside this.
func (w *noteWin) exOpen(a *bro, name string) error {
	if name == "" {
		return errors.New(":e which notebook?")
	}
	names := noteNames()
	to := ""
	for _, n := range names {
		if strings.EqualFold(n, name) || strings.EqualFold(path.Base(n), name) {
			to = n
			break
		}
	}
	if to == "" {
		var begun []string
		for _, n := range names {
			if strings.HasPrefix(strings.ToLower(path.Base(n)), strings.ToLower(name)) {
				begun = append(begun, n)
			}
		}
		if len(begun) == 1 {
			to = begun[0]
		}
	}
	if to == "" { // a new one, where this one is
		for _, d := range strings.Split(name, "/") {
			if err := noteNameOK(d); err != nil {
				return err
			}
		}
		to = path.Join(path.Dir(w.nb.name), name)
	}
	if to == w.nb.name {
		return nil
	}
	if o := a.noteWindow(to); o != nil {
		a.openNotebook(to) // focused where it is
		return nil
	}
	w.keep(a)
	if nw := a.noteOpen(to); nw != nil {
		a.replaceWin(w, nw)
	}
	return nil
}
