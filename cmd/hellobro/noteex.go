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
		{"page p", "N", "go to page N", func(arg string) error {
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
		{"new", "", "a new page after this one", page("new after")},
		{"copy", "", "a copy of this page after it", page("copy page")},
		{"delete", "", "this page gone (Ctrl+Z brings it back)", page("delete")},
		{"clear", "", "this page cleared", page("clear")},
		{"pen", "", "the pen", tool("pen")},
		{"eraser", "", "the eraser", tool("eraser")},
		{"select", "", "the select tool", tool("select")},
		{"text", "", "the text tool: tap the page, type (Esc: done)", tool("text")},
		{"undo u", "", "undo", func(string) error { w.doUndo(a, true); return nil }},
		{"redo", "", "redo", func(string) error { w.doUndo(a, false); return nil }},
		{"paper", strings.ReplaceAll(notePapers, " ", "|"), "the paper", func(arg string) error {
			if !isPaper(arg) {
				return errors.New("paper: " + strings.ReplaceAll(notePapers, " ", ", "))
			}
			w.nb.paper = arg
			w.changed = time.Now()
			w.place(a, w.p) // its button shows it
			return nil
		}},
		{"settings", "", "its settings: a finger, export, the bar", func(string) error { w.settingsOn(a, true); return nil }},
		{"pages o", "", "the pages' view, on or off", func(string) error { w.overviewOn(a, !w.overview); return nil }},
		{"export", "pdf|png", "export: a PDF of the pages, or this page as a PNG (none: asks)", func(arg string) error {
			switch arg {
			case "":
				w.export(a)
			case "pdf", "png":
				w.exportAs(a, arg)
			default:
				return errors.New("export: pdf or png")
			}
			return nil
		}},
		{"write w", "", "write it now", func(string) error {
			if !w.keep(a) {
				return errors.New("it can't be written now: it's kept as soon as it can be")
			}
			a.Notify(w.nb.name + " written")
			return nil
		}},
		{"edit e open", "name", "another notebook here (a new one if none is so named)", func(arg string) error { return w.exEdit(a, arg) }},
		{"rename", "name", "the notebook's name", func(arg string) error {
			if err := noteNameOK(arg); err != nil {
				return err
			}
			return renameNotebook(a, w.nb.name, path.Join(path.Dir(w.nb.name), arg))
		}},
		{"move mv", "folder", "to a folder (/ the top)", func(arg string) error {
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
		{"list ls all", "", "All Notebooks", func(string) error { a.notesFrom(w); return nil }},
		{"quit q close wq x", "", "close the window", func(string) error { a.closeWin(w); return nil }},
	}
	return append(cmds, exCmd{"help h", "", "this list", nil})
}

// exOpen opens the ":" line, text typed on it already.
func (w *noteWin) exOpen(a *bro, text string) {
	w.endTyping(a)
	if !a.shows(w) {
		return
	}
	w.ex = &exLine{text: text}
	a.vkAsk(true)
	a.drawEx(a.exStrip(w.p.Body), w.ex, w.exCmds(a))
}

// exEdit opens the notebook named so in this window: its name as it's
// listed, or the one name begins; none so named, a new one, beside this.
func (w *noteWin) exEdit(a *bro, name string) error {
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
