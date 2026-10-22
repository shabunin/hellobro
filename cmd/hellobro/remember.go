package main

// What hellobro remembers of how it was left, for the next start: the
// light, the terminal's font. Not settings (those are chosen, in the
// settings file): just where things were, in hellobro.json.

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const rememberFile = "hellobro.json"

type remembered struct {
	Light    *int `json:"light,omitempty"`     // the frontlight as last set, 0-100
	TermFont int  `json:"term-font,omitempty"` // the terminal font's size, px
	dir      string
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
	return r
}

func (r remembered) save() error {
	b, _ := json.MarshalIndent(r, "", "\t")
	err := os.WriteFile(filepath.Join(r.dir, rememberFile), append(b, '\n'), 0o644)
	if err != nil {
		log.Printf("%s: %v", rememberFile, err)
	}
	return err
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
		a.mem.save()
	}
}
