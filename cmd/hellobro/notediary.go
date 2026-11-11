package main

// The diary: a notebook that answers what's written in it, as Tom
// Riddle's did. Turned on in its settings (each notebook its own, kept
// in its file), once the pen has rested a few seconds what's been
// written since its last answer goes to a language model as a picture
// (ai.conf in the app's folder: the key, OpenCode Go's, and the model).
// If it reads as finished, a question asked or a sentence ended, the
// answer is written under it by hand (package hand), a word at a time;
// if not, it waits for more. Three taps of a finger on the page ask at
// once, finished or not. Its strokes are marked as the diary's in the
// file: under the last of them on a page is what's asked next. Each
// answer's cost is kept in ai-usage.tsv, for the settings to sum.

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
	"os"
	"slices"
	"strconv"
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
	diaryRadius     = 22  // its pen, tenths of a unit
	diaryMargin     = 90  // from the page's edges
	diaryDPI        = 150 // the picture sent
	diaryHistory    = 12  // messages of the talk before, sent with a question
	diaryWordTime   = 60 * time.Millisecond
	diaryStrokeTime = 22 * time.Millisecond
	diaryTaps       = 3                      // taps of a finger, quick, to ask now
	diaryTapGap     = 450 * time.Millisecond // between them, at most
	diaryMaxOut     = 4000                   // tokens: a model that thinks first needs room
)

// diaryModel is a model the diary can ask: one that reads a picture
// (tried, 2026-11-11), and its monthly limit on OpenCode Go's Go plan,
// in dollars (a fifth of it in 5 hours, half in a week).
type diaryModel struct {
	id, note string
	limit    float64
}

var diaryModels = []diaryModel{
	{"conf", "as ai.conf says", 0},
	{"claude-haiku-5-5", "reads well, quick", 15},
	{"glm-5.3-flash", "cheapest, quick", 60},
	{"minimax-m3", "quick, cheap", 60},
	{"qwen3.8-flash", "cheap", 30},
	{"mimo-v2.6-flash", "cheap, slower", 60},
	{"qwen3.7-plus", "slower", 60},
	{"longcat-2.0", "slower", 60},
	{"kimi-k2.7-code", "for code", 60},
	{"mimo-v2.6-pro", "", 15},
	{"qwen3.8-max", "strong, slow", 15},
	{"kimi-k3", "strong, dear", 15},
}

func diaryModelIDs() []string {
	var ids []string
	for _, m := range diaryModels {
		ids = append(ids, m.id)
	}
	return ids
}

func diaryModelOf(id string) diaryModel {
	for _, m := range diaryModels {
		if m.id == id {
			return m
		}
	}
	return diaryModel{id: id}
}

// diaryConf is ai.conf, with the model the settings choose.
func (a *bro) diaryConf() (ai.Config, error) {
	c, err := ai.Load(kobo.AppFile("ai.conf"))
	if m := a.setting("ai-model"); m != "conf" && m != "" {
		c.Model, c.Proto = m, ""
	}
	c.MaxOut = max(c.MaxOut, diaryMaxOut)
	return c, err
}

// diarySizes are the answer's capitals, page units, as "ai-size" has
// them (medium: about 5 mm).
var diarySizes = map[string]float64{"s": 34, "m": 46, "l": 60, "xl": 76}

// noteDiary is a notebook's diary at work.
type noteDiary struct {
	page    int       // where the last stroke was written
	wroteAt time.Time // when: zero, nothing new to send
	asked   int       // the question out, numbered (0 none): a stroke since and its answer's not wanted
	cancel  context.CancelFunc
	writing []diaryWord // the answer, still to write
	wrote   []noteOp    // what it wrote: one step to undo
	box     image.Rectangle
	nextAt  time.Time
}

// diaryTaps are a finger's taps on a page, the last few: three quick
// ones ask the diary at once.
type diaryTapper struct {
	at   []time.Time
	last image.Point
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
	if _, err := a.diaryConf(); err != nil {
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
			w.diaryAsk(a, false)
		}
	}
}

// diaryQuestion is what's been written on page since the diary's last
// answer there: the strokes under it (all of the page's, if it hasn't
// answered on it), and what's pasted there, typed or drawn.
func diaryQuestion(ss []*noteStroke) []*noteStroke {
	top := math.MinInt
	for _, s := range ss {
		if isDiary(s) {
			top = max(top, s.box.Max.Y)
		}
	}
	var q []*noteStroke
	for _, s := range ss {
		if !isDiary(s) && (s.box.Min.Y+s.box.Max.Y)/2 > top {
			q = append(q, s)
		}
	}
	return q
}

// diaryPicture is the strokes drawn on white, around their box: a PNG.
// A picture among them (a book's page pasted) is drawn as sharp as the
// screen has it, for its small print.
func (w *noteWin) diaryPicture(page int, q []*noteStroke) ([]byte, error) {
	dpi := float64(diaryDPI)
	if slices.ContainsFunc(q, func(s *noteStroke) bool { return s.pic != nil }) {
		dpi = 227
	}
	m, err := renderNotePage(exportPage{n: page, size: w.pageSize(page), strokes: q, pdfPage: -1}, "plain", nil, dpi)
	if err != nil {
		return nil, err
	}
	k := dpi / 227.0
	b := boxOf(q).Inset(-30)
	crop := image.Rect(int(float64(b.Min.X)*k), int(float64(b.Min.Y)*k), int(float64(b.Max.X)*k)+1, int(float64(b.Max.Y)*k)+1).Intersect(m.Bounds())
	var out bytes.Buffer
	if err := png.Encode(&out, m.SubImage(crop)); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// diaryRules are what the model's told whatever its voice: how to
// answer, for the notebook.
const diaryRules = `The writer writes by hand in a notebook, and what they wrote since your last answer comes to you as a picture. The talk before comes as text.

Reply with a JSON object only, nothing around it.
- If the writing isn't finished yet: it doesn't end with a question mark, a full stop or an exclamation mark, or it reads as cut off: reply {"status":"wait"}. Unless they asked for an answer at once: then answer.
- Otherwise reply {"status":"answer","question":"<what they wrote, word for word>","answer":"<your answer>"}.
- If the writing tells you how to answer from now on (a role, a manner, a language, what to call them...), add "prompt":"<that, as a standing instruction to you>" to your answer, and say in it, briefly, that you will.

Your answer is written back into the notebook by hand, so: answer in the language they wrote in, unless told otherwise; plain text, no markdown, no LaTeX (write formulas plainly, as x^2 + 1, sqrt(2)); short, a few sentences, unless they ask for more; new lines only between paragraphs. If you can't read a word, ask about it.`

// diaryVoices are the manners it can answer in, by name: a notebook's
// own (its settings).
var diaryVoices = []struct{ id, label, text string }{
	{"diary", "a diary, as Tom Riddle's", "You are a diary that answers, like Tom Riddle's: personal, warm, a little mysterious; you remember what was written to you."},
	{"tutor", "a tutor: step by step", "You are a patient tutor. Given a problem, work it through step by step; given the writer's own working, check it and point out where it goes wrong; ask a guiding question when it helps them learn."},
	{"helper", "a helper, plain", "You are a helpful assistant: plain, accurate and to the point."},
	{"brief", "brief: a line at most", "Answer as briefly as you can: a word, a number, or one sentence."},
}

const diaryVoiceAttr = "hellobro-diary-voice"

// diaryVoice is w's notebook's voice (diaryVoices), as its file keeps it.
func (nb *notebook) diaryVoice() int {
	v := attr(nb.attrs, diaryVoiceAttr)
	for i, dv := range diaryVoices {
		if dv.id == v {
			return i
		}
	}
	return 0
}

// stepVoice turns to the notebook's next voice, d on.
func (w *noteWin) stepVoice(d int) {
	n := len(diaryVoices)
	i := (w.nb.diaryVoice() + d + n) % n
	w.nb.attrs = slices.DeleteFunc(w.nb.attrs, func(a xml.Attr) bool { return a.Name.Local == diaryVoiceAttr })
	w.nb.attrs = append(w.nb.attrs, xml.Attr{Name: xml.Name{Local: diaryVoiceAttr}, Value: diaryVoices[i].id})
	w.changed = time.Now()
}

// An answer's first stroke keeps what was asked, what was answered and
// an instruction it was given: the talk is read back from the pages, so
// an answer undone or erased is gone from it too.
const (
	diaryQAttr      = "hellobro-q"
	diaryAAttr      = "hellobro-a"
	diaryPromptAttr = "hellobro-prompt"
)

// diaryTalk is the talk on nb's pages, in order: what was asked and
// answered; and the instructions it was given, in the order given.
func diaryTalk(nb *notebook) (msgs []ai.Msg, prompts []string) {
	for _, pg := range nb.pages {
		for _, s := range pg {
			ans := attr(s.attrs, diaryAAttr)
			if ans == "" {
				continue
			}
			q := attr(s.attrs, diaryQAttr)
			if q == "" {
				q = "(not read)"
			}
			p := attr(s.attrs, diaryPromptAttr)
			said, _ := json.Marshal(diaryReply{Status: "answer", Question: q, Answer: ans, Prompt: p})
			msgs = append(msgs, ai.Msg{Role: "user", Parts: []ai.Part{{Text: "Written (read): " + q}}},
				ai.Msg{Role: "assistant", Parts: []ai.Part{{Text: string(said)}}})
			if p != "" {
				prompts = append(prompts, p)
			}
		}
	}
	return
}

// diarySystem is what the model's told for nb: the rules, its voice, the
// writer's instructions.
func diarySystem(nb *notebook, prompts []string) string {
	var b strings.Builder
	b.WriteString(diaryVoices[nb.diaryVoice()].text + "\n\n" + diaryRules)
	if len(prompts) > 0 {
		b.WriteString("\n\nThe writer's standing instructions (follow them; a later one wins over an earlier):")
		for _, p := range prompts {
			b.WriteString("\n- " + p)
		}
	}
	return b.String()
}

// diaryReply is what the model says.
type diaryReply struct {
	Status   string `json:"status"`
	Question string `json:"question"`
	Answer   string `json:"answer"`
	Prompt   string `json:"prompt,omitempty"`
}

// diaryTap takes a finger's tap at p on page: the third quick one asks
// the diary now.
func (a *bro) diaryTap(w *noteWin, page int, p image.Point) {
	t := &a.nt.taps
	now := time.Now()
	if n := len(t.at); n > 0 && (now.Sub(t.at[n-1]) > diaryTapGap || kobo.Abs(p.X-t.last.X)+kobo.Abs(p.Y-t.last.Y) > 150) {
		t.at = nil
	}
	t.at, t.last = append(t.at, now), p
	if len(t.at) < diaryTaps {
		return
	}
	t.at = nil
	if !w.nb.diaryOn() {
		a.Notify("three taps ask the AI diary: it's off (the notebook's settings)")
		return
	}
	if w.diary == nil {
		w.diary = &noteDiary{}
	}
	d := w.diary
	if len(d.writing) > 0 {
		return
	}
	if d.cancel != nil {
		d.cancel()
		d.asked, d.cancel = 0, nil
	}
	if page >= len(w.nb.pages) || len(diaryQuestion(w.nb.pages[page])) == 0 { // tapped by a page with nothing new: the one written on
		page = d.page
	}
	d.page, d.wroteAt = page, time.Time{}
	w.diaryAsk(a, true)
}

// diaryAsk sends what's been written since the last answer; now, it's
// to be answered, finished or not.
func (w *noteWin) diaryAsk(a *bro, now bool) {
	d := w.diary
	var q []*noteStroke
	if d.page < len(w.nb.pages) {
		q = diaryQuestion(w.nb.pages[d.page])
	}
	if len(q) == 0 {
		if now {
			a.Notify("diary: nothing new written on the page")
		}
		return
	}
	cfg, err := a.diaryConf()
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
	say := "Written now:"
	if now {
		say = "Written now (they asked for an answer at once: answer it, finished or not):"
		a.Notify("diary: asking " + cfg.Model + "…")
	}
	talk, prompts := diaryTalk(w.nb)
	system := diarySystem(w.nb, prompts)
	msgs := append(talk[max(0, len(talk)-diaryHistory):],
		ai.Msg{Role: "user", Parts: []ai.Part{{Text: say}, {PNG: pic}}})
	style, size := a.setting("ai-hand"), diarySizes[a.setting("ai-size")]
	ctx, cancel := context.WithCancel(context.Background())
	d.asked++
	n, page, below := d.asked, d.page, boxOf(q)
	d.cancel = cancel
	log.Printf("diary %s: asking %s (%d strokes, a %d byte picture)", w.nb.name, cfg.Model, len(q), len(pic))
	go func() {
		out, err := cfg.Ask(ctx, system, msgs)
		if err == nil {
			diaryCount(cfg.Model, out)
		}
		a.Do(func() {
			if d.asked != n || !slices.Contains(a.noteWins(), w) { // written since, or stopped: asked again; or closed
				return
			}
			d.asked, d.cancel = 0, nil
			cancel()
			w.diaryAnswer(a, page, below, out.Text, err, style, size)
		})
	}()
}

// diaryAnswer takes the model's answer to what's in below on page.
func (w *noteWin) diaryAnswer(a *bro, page int, below image.Rectangle, out string, err error, style string, size float64) {
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
	if r.Prompt != "" {
		log.Printf("diary %s: an instruction: %q", w.nb.name, r.Prompt)
	}
	w.diaryLay(page, below, r, time.Now().UnixNano(), style, size)
	w.diary.nextAt = time.Now()
}

// diaryLay lays the answer out under below on page, to be written: on
// the next page where it's full (a page put in after it, unless the
// next is empty).
func (w *noteWin) diaryLay(page int, below image.Rectangle, r diaryReply, seed int64, style string, diarySize float64) {
	text := r.Answer
	d := w.diary
	if diarySize == 0 {
		diarySize = diarySizes["m"]
	}
	h, err := hand.Style(style)
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
	base, first := math.Inf(-1), true
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
			if first { // what it said, for the talk
				first = false
				s.attrs = append(s.attrs, xml.Attr{Name: xml.Name{Local: diaryQAttr}, Value: r.Question},
					xml.Attr{Name: xml.Name{Local: diaryAAttr}, Value: r.Answer})
				if r.Prompt != "" {
					s.attrs = append(s.attrs, xml.Attr{Name: xml.Name{Local: diaryPromptAttr}, Value: r.Prompt})
				}
			}
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
	d.nextAt = time.Now().Add(diaryWordTime + time.Duration(min(len(wd.strokes), 8))*diaryStrokeTime) // as long a word, whatever the hand
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

// diaryCut stops an answer being written: what's written of it is a
// step, to undo.
func (w *noteWin) diaryCut(a *bro) {
	if d := w.diary; d != nil && len(d.writing) > 0 {
		d.writing = nil
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

// --- what it's cost ---

func diaryUsagePath() string { return kobo.AppFile("ai-usage.tsv") }

// diaryCount notes an answer's cost: a line of the time, the model, the
// dollars counted against OpenCode Go's limits, the tokens in and out.
func diaryCount(model string, out ai.Answer) {
	f, err := os.OpenFile(diaryUsagePath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		log.Printf("diary: usage: %v", err)
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%d\t%s\t%.7f\t%d\t%d\n", time.Now().Unix(), model, out.Cost, out.In, out.Out)
}

// diaryUsage sums what model's answers cost over the 5 hours, the week
// and the 30 days to now, as OpenCode Go's limits are counted.
func diaryUsage(data []byte, model string, now time.Time) (h5, week, month float64) {
	for l := range strings.Lines(string(data)) {
		f := strings.Split(strings.TrimSpace(l), "\t")
		if len(f) < 3 || f[1] != model {
			continue
		}
		sec, err1 := strconv.ParseInt(f[0], 10, 64)
		cost, err2 := strconv.ParseFloat(f[2], 64)
		if err1 != nil || err2 != nil {
			continue
		}
		switch ago := now.Sub(time.Unix(sec, 0)); {
		case ago < 5*time.Hour:
			h5 += cost
			fallthrough
		case ago < 7*24*time.Hour:
			week += cost
			fallthrough
		case ago < 30*24*time.Hour:
			month += cost
		}
	}
	return
}

// diaryModelNow is the model the diary asks.
func (a *bro) diaryModelNow() string {
	c, _ := a.diaryConf()
	return c.Model
}

// usageShown is what the settings say of the model's usage: dollars,
// and of its limit.
func (a *bro) usageShown() string {
	data, _ := os.ReadFile(diaryUsagePath())
	m := a.diaryModelNow()
	h5, week, month := diaryUsage(data, m, time.Now())
	lim := diaryModelOf(m).limit
	if lim == 0 {
		return fmt.Sprintf("5 h $%.3f · week $%.3f · month $%.3f", h5, week, month)
	}
	pc := func(v, of float64) string {
		p, prec := 100*v/of, 1
		if p > 0 && p < 1 {
			prec = 2
		}
		return strconv.FormatFloat(p, 'f', prec, 64) + "%"
	}
	return fmt.Sprintf("5 h %s · week %s · month %s", pc(h5, lim/5), pc(week, lim/2), pc(month, lim))
}

// usageNote is the line under the settings that says what that is.
func (a *bro) usageNote() string {
	m := a.diaryModelNow()
	data, _ := os.ReadFile(diaryUsagePath())
	_, _, month := diaryUsage(data, m, time.Now())
	if lim := diaryModelOf(m).limit; lim > 0 {
		return fmt.Sprintf("AI usage: %s's, $%.3f this month, of the Go plan's $%.0f (a fifth in 5 h, half a week); this Kobo's only", m, month, lim)
	}
	return fmt.Sprintf("AI usage: %s's, in dollars, this Kobo's questions only", m)
}
