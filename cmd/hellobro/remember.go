package main

// What hellobro remembers, for the next start, in hellobro.json: how it
// was left (the light, the terminal's font) and the SSH tab's saved
// hosts. Not settings: those are chosen, in the settings file.

import (
	"bufio"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"hellobro/wm"
)

const rememberFile = "hellobro.json"

type remembered struct {
	Light       *int                 `json:"light,omitempty"`     // the frontlight as last set, 0-100
	LightOn     int                  `json:"light-on,omitempty"`  // and as last set on: light.toggle's way back
	TermFont    int                  `json:"term-font,omitempty"` // the terminal font's size, px
	Hosts       []savedHost          `json:"hosts,omitempty"`     // the SSH tab's, in order
	Layout      *wm.Saved            `json:"layout,omitempty"`    // the windows, as they were left (wm.go)
	Books       map[string]bookPlace `json:"books,omitempty"`     // where each book was left (bookwin.go)
	BookFont    string               `json:"book-font,omitempty"`
	ShelfTab    int                  `json:"shelf-tab,omitempty"` // the shelf's tab, and folder (library.go)
	ShelfDir    string               `json:"shelf-dir,omitempty"`
	NotesTab    int                  `json:"notes-tab,omitempty"` // the notebooks' list's, as the shelf's (notelist.go)
	NotesDir    string               `json:"notes-dir,omitempty"`
	BookPx      int                  `json:"book-px,omitempty"`
	BookLeading int                  `json:"book-leading,omitempty"` // a line's height, % of the size
	BookMargin  int                  `json:"book-margin,omitempty"`  // 1-5, the sides' margins
	BookRagged  bool                 `json:"book-ragged,omitempty"`  // no justified text
	BookNoHyph  bool                 `json:"book-no-hyphens,omitempty"`
	PDFWidth    bool                 `json:"pdf-fit-width,omitempty"` // PDFs' pages as wide as the screen, not whole (bookpdf.go)
	PDFNoTrim   bool                 `json:"pdf-no-trim,omitempty"`   // their white margins kept
	Pen         bool                 `json:"pen,omitempty"`           // a pen has written in a notebook: fingers turn its pages (notebook.go)
	NoteWidth   string               `json:"note-width,omitempty"`    // the notebooks' pen: fine, medium, bold (notepen.go)
	NoteInk     string               `json:"note-ink,omitempty"`      // black, gray, marker
	dir         string
}

// loadRemembered reads hellobro.json in dir, the app's.
func loadRemembered(dir string) remembered {
	r := remembered{dir: dir}
	if b, err := os.ReadFile(filepath.Join(dir, rememberFile)); err == nil {
		if err := json.Unmarshal(b, &r); err != nil {
			log.Printf("%s: %v", rememberFile, err)
		}
	}
	if r.TermFont == 0 { // as it was kept once, in its own file
		old := filepath.Join(dir, "term-font")
		if b, err := os.ReadFile(old); err == nil {
			r.TermFont, _ = strconv.Atoi(strings.TrimSpace(string(b)))
			if r.TermFont != 0 && r.save() == nil {
				os.Remove(old)
			}
		}
	}
	r.importSSHConf()
	return r
}

// importSSHConf takes in the hosts of an ssh.conf, if there is one (as
// they were kept once, and as the harness and make install leave them),
// those whose names are new; it's kept as ssh.conf.old.
func (r *remembered) importSSHConf() {
	conf := filepath.Join(r.dir, "ssh.conf")
	f, err := os.Open(conf)
	if err != nil {
		return
	}
	n := 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		h, ok := parseSSHConfLine(sc.Text())
		if ok && !slices.ContainsFunc(r.Hosts, func(s savedHost) bool { return s.Name == h.name }) {
			r.Hosts = append(r.Hosts, h.saved())
			n++
		}
	}
	f.Close()
	if r.save() == nil {
		os.Rename(conf, conf+".old")
		log.Printf("%s: %d hosts from ssh.conf (now ssh.conf.old)", rememberFile, n)
	}
}

// hosts are the saved hosts.
func (r remembered) hosts() []sshHost {
	var hs []sshHost
	for _, s := range r.Hosts {
		if h, ok := s.host(); ok {
			hs = append(hs, h)
		}
	}
	return hs
}

// saveHosts keeps the SSH tab's hosts.
func (a *bro) saveHosts() error {
	a.mem.Hosts = nil
	for _, h := range a.tv.hosts {
		a.mem.Hosts = append(a.mem.Hosts, h.saved())
	}
	return a.mem.save()
}

func (r remembered) save() error {
	b, _ := json.MarshalIndent(r, "", "\t")
	err := writeAtomic(filepath.Join(r.dir, rememberFile), append(b, '\n'))
	if err != nil {
		log.Printf("%s: %v", rememberFile, err)
	}
	return err
}

// writeAtomic writes b to path by way of a new file, on the card before
// it takes the old one's place: a crash or a dead battery halfway leaves
// the old one whole (FAT keeps no journal).
func writeAtomic(path string, b []byte) error {
	tmp := path + ".new"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	_, err = f.Write(b)
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	if d, err := os.Open(filepath.Dir(path)); err == nil { // the rename, on the card too
		d.Sync()
		d.Close()
	}
	return nil
}

// applyLightStart starts the light as it was left, unless the setting
// says Nickel's.
func (a *bro) applyLightStart() {
	a.LightStart = nil
	if a.setting("light-start") == "last" {
		a.LightStart = a.mem.Light
	}
	a.LightKept = func(l int) {
		a.mem.Light = &l
		if l > 0 {
			a.mem.LightOn = l
		}
		a.mem.save()
	}
}
