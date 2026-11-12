package main

// Screenshots: from the quick menu (or the switcher, or its keys), the
// menu goes and the screen is kept as it is then; a rectangle dragged
// round a part of it (a finger or the pen; its outline dashed) keeps
// that part, a tap all of it, Esc none. It's a PNG in Screenshots on
// the Kobo's storage (over USB, or file sharing), and the clipboard's
// picture, to paste into a notebook.

import (
	"bytes"
	"image"
	"image/draw"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"time"

	"hellobro/kobo"
)

const (
	ownShot   = 21 // a contact taking a screenshot
	shotDir   = "Screenshots"
	shotStill = 12 // px a tap may move
	shotWait  = 500 * time.Millisecond
)

// shot is a screenshot being taken: the screen as it was, and the
// rectangle dragged.
type shot struct {
	img   *image.Gray
	g     *kobo.Gesture
	from  image.Point
	shown []image.Rectangle // the outline's, on screen
}

// shotStart keeps the screen once what's over it (the menu) has gone.
func (a *bro) shotStart() {
	time.AfterFunc(shotWait, func() {
		a.Do(func() {
			img := image.NewGray(a.C.Bounds())
			copy(img.Pix, a.C.Gray.Pix)
			a.shot = &shot{img: img}
			a.Toast("screenshot: drag round a part, or tap for all; Esc: none", 8*time.Second)
			log.Print("screenshot: waiting for a rectangle")
		})
	})
}

// shotDown takes a contact while a screenshot's being taken.
func (a *bro) shotDown(p image.Point, g *kobo.Gesture) bool {
	s := a.shot
	if s == nil || s.g != nil {
		return s != nil // a second contact is nothing
	}
	s.g, s.from, g.Owner = g, p, ownShot
	return true
}

// shotMove draws the rectangle as it's dragged.
func (a *bro) shotMove(p image.Point, g *kobo.Gesture) bool {
	s := a.shot
	if s == nil || s.g != g {
		return false
	}
	if s.shown == nil && g.Moved < shotStill {
		return true
	}
	if s.shown == nil {
		a.Ink(true)
	}
	old := s.shown
	a.shotUndraw()
	r := image.Rectangle{s.from, p}.Canon()
	cs := corners(r)
	s.shown = nil
	for i, q := range cs {
		n := cs[(i+1)%4]
		dashedLine(a.C, q, n, a.C.Bounds())
		s.shown = append(s.shown, image.Rectangle{q, n}.Canon().Inset(-3))
	}
	for _, r := range append(old, s.shown...) {
		a.Mark(r, false)
	}
	return true
}

// shotUndraw puts the screen back where the outline is.
func (a *bro) shotUndraw() {
	for _, r := range a.shot.shown {
		draw.Draw(a.C.Gray, r, a.shot.img, r.Min, draw.Src)
	}
}

// shotLift keeps what the rectangle's round, or all of it for a tap.
func (a *bro) shotLift(g *kobo.Gesture) bool {
	s := a.shot
	if s == nil || s.g != g {
		return false
	}
	a.shot = nil
	if s.shown != nil {
		a.Ink(false)
		for _, r := range s.shown {
			draw.Draw(a.C.Gray, r, s.img, r.Min, draw.Src)
			a.Mark(r, true)
		}
	}
	r := s.img.Bounds()
	if s.shown != nil && g.Moved >= shotStill {
		r = image.Rectangle{s.from, g.Last}.Canon().Intersect(r)
	}
	if r.Dx() < 8 || r.Dy() < 8 {
		a.Toast("screenshot: too small, none taken", 3*time.Second)
		return true
	}
	name, err := saveShot(s.img.SubImage(r).(*image.Gray), time.Now())
	if err != nil {
		log.Printf("screenshot: %v", err)
		a.Notify("screenshot: " + err.Error())
		return true
	}
	log.Printf("screenshot: %v as %s", r, name)
	a.Toast("screenshot: "+filepath.Join(shotDir, filepath.Base(name))+", and the clipboard's picture", 5*time.Second)
	return true
}

// shotCancel lets a screenshot go, untaken.
func (a *bro) shotCancel() {
	if a.shot == nil {
		return
	}
	if a.shot.shown != nil {
		a.Ink(false)
		a.shotUndraw()
		for _, r := range a.shot.shown {
			a.Mark(r, true)
		}
	}
	a.shot = nil
	a.Toast("screenshot: none", 2*time.Second)
}

// saveShot writes m in Screenshots, named for when, and makes it the
// clipboard's picture; the file's path.
func saveShot(m *image.Gray, when time.Time) (string, error) {
	var b bytes.Buffer
	if err := png.Encode(&b, m); err != nil {
		return "", err
	}
	dir := filepath.Join(shareRoot, shotDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := filepath.Join(dir, when.Format("hellobro-2006-01-02-150405.png"))
	if err := os.WriteFile(name, b.Bytes(), 0o644); err != nil {
		return "", err
	}
	if err := os.WriteFile(kobo.AppFile(clipPic), b.Bytes(), 0o600); err != nil {
		log.Printf("screenshot: the clipboard: %v", err)
	}
	return name, nil
}
