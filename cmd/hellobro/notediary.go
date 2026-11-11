package main

// The diary: a notebook that answers what's written in it, as Tom
// Riddle's did. Turned on in its settings (each notebook its own, kept
// in its file), once the pen has rested a few seconds what's been
// written since its last answer goes to a language model as a picture
// (ai.conf in the app's folder: the key, OpenCode Go's, and the model).
// If it reads as finished, a question asked or a sentence ended, the
// answer is written under it by hand (package hand), a word at a time;
// if not, it waits for more. Its strokes are marked as the diary's in
// the file: under the last of them on a page is what's asked next.

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"hash/fnv"
	"image"
	"image/png"
	"log"
	"math"
	"slices"
	"strings"
	"time"

	"hellobro/ai"
	"hellobro/hand"
	"hellobro/kobo"
)

const (
	diaryPause      = 3 * time.Second // the pen rested this long: what's written goes
	diaryAttr       = "hellobro"      // a stroke's attribute in the file, "diary" on its own
	diaryOnAttr     = "hellobro-diary"
	diarySize       = 46  // an answer's capitals, page units (about 5 mm)
	diaryRadius     = 22  // its pen, tenths of a unit
	diaryMargin     = 90  // from the page's edges
	diaryDPI        = 150 // the picture sent
	diaryHistory    = 12  // messages of the talk before, sent with a question
	diaryWordTime   = 70 * time.Millisecond
	diaryStrokeTime = 22 * time.Millisecond
)

// noteDiary is a notebook's diary at work.
type noteDiary struct {
	page    int       // where the last stroke was written
	wroteAt time.Time // when: zero, nothing new to send
	asked   int       // the question out, numbered (0 none): a stroke since and its answer's not wanted
	cancel  context.CancelFunc
	history []ai.Msg    // the talk so far, as text
	writing []diaryWord // the answer, still to write
	wrote   []noteOp    // what it wrote: one step to undo
	box     image.Rectangle
	nextAt  time.Time
}

// diaryWord is a word of an answer: its strokes, on its page.
type diaryWord struct {
	page    int
	strokes []*noteStroke
}

func (nb *notebook) diaryOn() bool { return attr(nb.attrs, diaryOnAttr) == "on" }

func (nb *notebook) setDiary(on bool) {
	nb.attrs = slices.DeleteFunc(nb.attrs, func(a xml.Attr) bool { return a.Name.Local == diaryOnAttr })
	if on {
		nb.attrs = append(nb.attrs, xml.Attr{Name: xml.Name{Local: diaryOnAttr}, Value: "on"})
	}
}

func onOff(on bool) string { return map[bool]string{true: "on", false: "off"}[on] }

func isDiary(s *noteStroke) bool { return attr(s.attrs, diaryAttr) == "diary" }

// flipDiary turns w's diary on or off.
func (w *noteWin) flipDiary(a *bro) {
	on := !w.nb.diaryOn()
	w.nb.setDiary(on)
	w.changed = time.Now()
	if !on {
		w.diaryStop()
		return
	}
	if _, err := ai.Load(kobo.AppFile("ai.conf")); err != nil {
		a.Notify("the diary needs ai.conf in the app's folder: " + diaryConfErr(err))
	}
}

func diaryConfErr(err error) string {
	if strings.Contains(err.Error(), "no such file") {
		return "there's none"
	}
	return err.Error()
}

// diaryStop forgets a question out, and an answer being written (what's
// written of it stays, a step to undo).
func (w *noteWin) diaryStop() {
	d := w.diary
	if d == nil {
		return
	}
	if d.cancel != nil {
		d.cancel()
	}
	d.asked, d.cancel, d.wroteAt = 0, nil, time.Time{}
	if len(d.wrote) > 0 {
		w.diaryDone(nil)
	}
	d.writing = nil
}

// diaryWrote notes a stroke written on page: once the pen rests, it's
// asked; a question out is asked again, with it.
func (w *noteWin) diaryWrote(page int) {
	if !w.nb.diaryOn() {
		return
	}
	if w.diary == nil {
		w.diary = &noteDiary{}
	}
	d := w.diary
	if d.cancel != nil {
		d.cancel()
		d.asked, d.cancel = 0, nil
	}
	d.page, d.wroteAt = page, time.Now()
}

// diaryTick writes on the answers coming, and asks once the pen has
// rested (the pen in the air over the page: more is coming).
func (a *bro) diaryTick() {
	for _, w := range a.noteWins() {
		d := w.diary
		switch {
		case d == nil:
		case len(d.writing) > 0:
			if time.Now().After(d.nextAt) {
				w.diaryWriteWord(a)
			}
		case d.asked == 0 && !d.wroteAt.IsZero() && time.Since(d.wroteAt) >= diaryPause &&
			time.Since(a.nt.hoverAt) >= noteHoverGone && w.typing == nil:
			d.wroteAt = time.Time{}
			w.diaryAsk(a)
		}
	}
}

// diaryQuestion is what's been written on page since the diary's last
// answer there: the strokes under it (all of the page's, if it hasn't
// answered on it).
func diaryQuestion(ss []*noteStroke) []*noteStroke {
	top := math.MinInt
	for _, s := range ss {
		if isDiary(s) {
			top = max(top, s.box.Max.Y)
		}
	}
	var q []*noteStroke
	for _, s := range ss {
		if !isDiary(s) && s.pic == nil && (s.box.Min.Y+s.box.Max.Y)/2 > top {
			q = append(q, s)
		}
	}
	return q
}

// diaryPicture is the strokes drawn on white, around their box: a PNG.
func (w *noteWin) diaryPicture(page int, q []*noteStroke) ([]byte, error) {
	m, err := renderNotePage(exportPage{n: page, size: w.pageSize(page), strokes: q, pdfPage: -1}, "plain", nil, diaryDPI)
	if err != nil {
		return nil, err
	}
	k := diaryDPI / 227.0
	b := boxOf(q).Inset(-30)
	crop := image.Rect(int(float64(b.Min.X)*k), int(float64(b.Min.Y)*k), int(float64(b.Max.X)*k)+1, int(float64(b.Max.Y)*k)+1).Intersect(m.Bounds())
	var out bytes.Buffer
	if err := png.Encode(&out, m.SubImage(crop)); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

const diarySystem = `You are a diary that answers, like Tom Riddle's: the user writes by hand in a notebook, and what they wrote since your last answer comes to you as a picture. Earlier pages of the talk come as text.

Reply with a JSON object only, nothing around it.
- If the writing isn't finished yet: it doesn't end with a question mark, a full stop or an exclamation mark, or it reads as cut off: reply {"status":"wait"}.
- Otherwise reply {"status":"answer","question":"<what they wrote, word for word>","answer":"<your answer>"}.

Your answer is written back into the notebook by hand, so: answer in the language they wrote in; plain text, no markdown, no LaTeX (write formulas plainly, as x^2 + 1, sqrt(2)); short, a few sentences, unless they ask for more; new lines only between paragraphs. If you can't read a word, ask about it.`

// diaryReply is what the model says.
type diaryReply struct {
	Status   string `json:"status"`
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

// diaryAsk sends what's been written since the last answer.
func (w *noteWin) diaryAsk(a *bro) {
	d := w.diary
	if d.page >= len(w.nb.pages) {
		return
	}
	q := diaryQuestion(w.nb.pages[d.page])
	if len(q) == 0 {
		return
	}
	cfg, err := ai.Load(kobo.AppFile("ai.conf"))
	if err != nil {
		a.Notify("diary: ai.conf: " + diaryConfErr(err))
		return
	}
	pic, err := w.diaryPicture(d.page, q)
	if err != nil {
		log.Printf("diary: the picture: %v", err)
		return
	}
	cfg.Agent = "hellobro/" + version
	h := fnv.New64a()
	h.Write([]byte(w.nb.name))
	cfg.Session = fmt.Sprintf("hellobro-%x", h.Sum64())
	msgs := append(slices.Clone(d.history[max(0, len(d.history)-diaryHistory):]),
		ai.Msg{Role: "user", Parts: []ai.Part{{Text: "Written now:"}, {PNG: pic}}})
	ctx, cancel := context.WithCancel(context.Background())
	d.asked++
	n, page, below := d.asked, d.page, boxOf(q)
	d.cancel = cancel
	log.Printf("diary %s: asking %s (%d strokes, a %d byte picture)", w.nb.name, cfg.Model, len(q), len(pic))
	go func() {
		out, err := cfg.Ask(ctx, diarySystem, msgs)
		a.Do(func() {
			if d.asked != n || !slices.Contains(a.noteWins(), w) { // written since, or stopped: asked again; or closed
				return
			}
			d.asked, d.cancel = 0, nil
			cancel()
			w.diaryAnswer(a, page, below, out, err)
		})
	}()
}

// diaryAnswer takes the model's answer to what's in below on page.
func (w *noteWin) diaryAnswer(a *bro, page int, below image.Rectangle, out string, err error) {
	if err != nil {
		log.Printf("diary %s: %v", w.nb.name, err)
		a.Notify("diary: " + err.Error())
		return
	}
	var r diaryReply
	if err := ai.JSON(out, &r); err != nil {
		log.Printf("diary %s: %v", w.nb.name, err)
		r = diaryReply{Status: "answer", Answer: strings.TrimSpace(out)} // it answered, but not as asked
	}
	if r.Status != "answer" || strings.TrimSpace(r.Answer) == "" {
		log.Printf("diary %s: waits", w.nb.name)
		return
	}
	log.Printf("diary %s: answers %d characters", w.nb.name, len(r.Answer))
	said, _ := json.Marshal(r)
	q := r.Question
	if q == "" {
		q = "(not read)"
	}
	w.diary.history = append(w.diary.history, ai.Msg{Role: "user", Parts: []ai.Part{{Text: "Written now (read): " + q}}},
		ai.Msg{Role: "assistant", Parts: []ai.Part{{Text: string(said)}}})
	w.diaryLay(page, below, r.Answer, time.Now().UnixNano())
	w.diary.nextAt = time.Now()
}

// diaryLay lays the answer out under below on page, to be written: on
// the next page where it's full (a page put in after it, unless the
// next is empty).
func (w *noteWin) diaryLay(page int, below image.Rectangle, text string, seed int64) {
	d := w.diary
	h, err := hand.Script()
	if err != nil {
		log.Printf("diary: %v", err)
		return
	}
	size := w.pageSize(page)
	left := max(diaryMargin, min(below.Min.X, size.X/4))
	wr := h.Write(text, diarySize, float64(size.X-diaryMargin-left), seed)
	lineH := hand.LineHeight(diarySize)
	dx, dy := float64(left), float64(below.Max.Y)+lineH*0.5 // the first line's top
	if below.Empty() {
		dy = diaryMargin
	}
	base := math.Inf(-1)
	for _, wd := range wr.Words {
		if wd.Base != base { // a line begins: is there room for it?
			base = wd.Base
			if dy+base+diarySize*0.5 > float64(size.Y-diaryMargin/2) {
				page = w.diaryPage(page)
				size = w.pageSize(page)
				dy = diaryMargin - (base - diarySize)
			}
		}
		dw := diaryWord{page: page}
		for _, l := range wd.Strokes {
			s := &noteStroke{gray: kobo.Black, attrs: []xml.Attr{
				{Name: xml.Name{Local: "tool"}, Value: "pen"}, {Name: xml.Name{Local: "color"}, Value: "#000000ff"},
				{Name: xml.Name{Local: "capStyle"}, Value: "round"}, {Name: xml.Name{Local: diaryAttr}, Value: "diary"}}}
			for _, p := range l {
				s.pts = append(s.pts, notePt{int16(math.Round(p.X + dx)), int16(math.Round(p.Y + dy)), diaryRadius})
			}
			s.bound()
			dw.strokes = append(dw.strokes, s)
		}
		d.writing = append(d.writing, dw)
	}
}

// diaryPage is the page after page to go on writing on: the next, if
// it's empty, or a new one put in.
func (w *noteWin) diaryPage(page int) int {
	next := page + 1
	if next < len(w.nb.pages) && len(w.nb.pages[next]) == 0 {
		return next
	}
	m := defaultMeta()
	if page < len(w.nb.meta) && w.nb.pdfPath() == "" {
		m = w.nb.meta[page]
	}
	if next > len(w.nb.pages) {
		next = len(w.nb.pages)
	}
	op := noteOp{kind: pageIns, page: next, meta: m}
	w.nb.apply(op, false)
	w.diary.wrote = append(w.diary.wrote, op)
	return next
}

// diaryWriteWord writes the answer's next word: on its page, and on
// the screen if it shows. The pages turn with it, unless the pen's at
// work.
func (w *noteWin) diaryWriteWord(a *bro) {
	d := w.diary
	wd := d.writing[0]
	d.writing = d.writing[1:]
	for len(w.nb.pages) <= wd.page {
		w.nb.pages = append(w.nb.pages, nil)
	}
	w.nb.pages[wd.page] = append(w.nb.pages[wd.page], wd.strokes...)
	if n := len(d.wrote); n > 0 && d.wrote[n-1].kind == pageStrokes && d.wrote[n-1].page == wd.page {
		d.wrote[n-1].add = append(d.wrote[n-1].add, wd.strokes...)
	} else {
		d.wrote = append(d.wrote, noteOp{page: wd.page, add: slices.Clone(wd.strokes)})
	}
	d.nextAt = time.Now().Add(diaryWordTime + time.Duration(len(wd.strokes))*diaryStrokeTime)
	w.changed = time.Now() // kept, should the window close before it's done
	if !w.p.Outer.Empty() && a.shows(w) && !w.overview && !w.settings {
		if wd.page == w.cur+w.shown() && a.nt.ink == nil && a.nt.drag == nil && w.typing == nil {
			w.cur = wd.page
			w.show(a)
			a.MarkFlash(w.pages())
		}
		if i := wd.page - w.cur; i >= 0 && i < len(w.slots) {
			at, clip := w.slots[i], w.paperArea()
			var box image.Rectangle
			for _, s := range wd.strokes {
				w.drawStroke(a, s, at, clip)
				box = box.Union(w.toScreenRect(s.box, at))
			}
			a.Mark(box.Intersect(clip), false)
			d.box = d.box.Union(box.Intersect(clip))
		}
	}
	if len(d.writing) == 0 {
		w.diaryDone(a)
	}
}

// diaryDone ends an answer: what it wrote is a step to undo, and gets
// its cleanup on screen.
func (w *noteWin) diaryDone(a *bro) {
	d := w.diary
	if len(d.wrote) > 0 {
		w.edited(d.wrote...)
	}
	d.wrote = nil
	if a != nil && !d.box.Empty() && a.shows(w) {
		a.Mark(d.box, true)
		w.drawBar(a)
		a.Mark(w.bar, true)
	}
	d.box = image.Rectangle{}
}
