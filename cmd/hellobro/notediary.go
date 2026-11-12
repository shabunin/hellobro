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
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/image/font"

	"hellobro/ai"
	"hellobro/hand"
	"hellobro/kobo"
)

const (
	diaryAttr       = "hellobro" // a stroke's attribute in the file, "diary" on its own
	diaryOnAttr     = "hellobro-diary"
	diaryRadius     = 22  // its pen, tenths of a unit
	diaryMargin     = 90  // from the page's edges
	diaryDPI        = 150 // the picture sent
	diaryHistory    = 12  // messages of the talk before, sent with a question
	diaryWordTime   = 60 * time.Millisecond
	diaryStrokeTime = 22 * time.Millisecond
	diaryTapGap     = 450 * time.Millisecond // between them, at most
	diaryMaxOut     = 4000                   // tokens: a model that thinks first needs room
)

// The models to choose from are the API's own list (ai.Models), those
// that read pictures, kept in ai-models.json and fetched again when it's
// a day old and the models are looked at (the notebook's settings, the
// Settings). What we know of a few besides: the Go plan's monthly limit
// on each, in dollars (a fifth of it in 5 hours, half in a week), and
// what a question cost and took when tried (2026-11-11).
var (
	diaryLimits = map[string]float64{
		"claude-haiku-5-5": 15, "glm-5.3-flash": 60, "minimax-m3": 60, "qwen3.8-flash": 30, "mimo-v2.6-flash": 60,
		"qwen3.7-plus": 60, "kimi-k2.7-code": 60, "mimo-v2.6-pro": 15, "qwen3.8-max": 15, "kimi-k3": 15,
	}
	diaryTried = map[string]string{
		"claude-haiku-5-5": "reads well", "minimax-m3": "quick", "kimi-k2.7-code": "for code",
	}
)

const (
	modelsFile  = "ai-models.json"
	modelsStale = 24 * time.Hour
	modelsV     = 2 // the cache's: an older one is fetched again (v2: the models' features)
)

// modelsState is the models' list, as last fetched.
type modelsState struct {
	List     []ai.Model `json:"models"`
	At       time.Time  `json:"at"`
	Base     string     `json:"base"` // the API it's of
	V        int        `json:"v"`
	loaded   bool
	fetching bool
	err      string
	errAt    time.Time
}

// models is the list, read from its file the first time.
func (a *bro) models() *modelsState {
	ms := &a.nt.models
	if !ms.loaded {
		ms.loaded = true
		if b, err := os.ReadFile(kobo.AppFile(modelsFile)); err == nil {
			json.Unmarshal(b, ms)
		}
	}
	return ms
}

// modelsWant fetches the list again if it's old, or of another API than
// ai.conf's now, off the loop; the setting's choices are its then.
func (a *bro) modelsWant() { a.modelsFetch(false) }

// modelsFetch fetches the list: now if force, else if it's old (and not
// failed a moment ago).
func (a *bro) modelsFetch(force bool) {
	ms := a.models()
	c, err := ai.Load(kobo.AppFile("ai.conf"))
	if err != nil {
		ms.err = diaryConfErr(err)
		return
	}
	fresh := ms.Base == c.Base && ms.V == modelsV && time.Since(ms.At) < modelsStale
	if ms.fetching || !force && (fresh || time.Since(ms.errAt) < 5*time.Minute) {
		return
	}
	ms.fetching = true
	c.Agent = "hellobro/" + version
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		list, err := c.Models(ctx)
		a.Do(func() {
			ms.fetching = false
			if err != nil {
				log.Printf("ai: the models' list: %v", err)
				ms.err, ms.errAt = err.Error(), time.Now()
				a.inkRedraw(modelsKey)
				return
			}
			ms.List, ms.At, ms.Base, ms.V, ms.err = list, time.Now(), c.Base, modelsV, ""
			if b, err := json.MarshalIndent(ms, "", " "); err == nil {
				writeAtomic(kobo.AppFile(modelsFile), b)
			}
			log.Printf("ai: %d models listed", len(list))
			a.applyModels()
			a.noteSettingsRedraw()
			a.inkRedraw("settings")
			a.inkRedraw(modelsKey)
		})
	}()
	a.inkRedraw(modelsKey) // "fetching"
}

// diaryCan says the diary can ask m: it reads pictures (if the list
// says), by a protocol we speak.
func diaryCan(m ai.Model) bool { return (m.Images || !m.Known) && m.Proto != "-" }

// applyModels makes the list the "ai-model" setting's choices: as
// ai.conf says first, then those we can ask, cheapest first; the one
// chosen stays, listed or not.
func (a *bro) applyModels() {
	i := settingIndex("ai-model")
	st := &settingList[i]
	cur := st.values[a.set["ai-model"]]
	values, shown := []string{"conf"}, []string{"as ai.conf says"}
	for _, m := range a.models().List {
		if m.Proto != "-" {
			values, shown = append(values, m.ID), append(shown, modelLabel(m))
		}
	}
	if !slices.Contains(values, cur) {
		values, shown = append(values, cur), append(shown, cur+", not listed now")
	}
	st.values, st.shown = values, shown
	a.set["ai-model"] = slices.Index(values, cur)
}

// modelLabel is how a model's listed: its id, what it's good for, its
// price in and out, a million tokens.
func modelLabel(m ai.Model) string {
	l := m.ID
	if n := diaryTried[m.ID]; n != "" {
		l += ", " + n
	}
	switch {
	case m.In == 0 && m.Out == 0:
		l += " · free"
	case m.In >= 0:
		l += fmt.Sprintf(" · $%s / $%s", price(m.In), price(m.Out))
	}
	return l
}

// price is dollars, as few figures as say it.
func price(f float64) string {
	s := strconv.FormatFloat(f, 'f', 2, 64)
	if f >= 10 {
		s = strconv.FormatFloat(f, 'f', 0, 64)
	}
	if strings.Contains(s, ".") {
		s = strings.TrimSuffix(strings.TrimSuffix(s, "0"), ".0")
	}
	return s
}

// modelProto is how the list says model is reached; "" if it doesn't.
func (a *bro) modelProto(model string) string {
	for _, m := range a.models().List {
		if m.ID == model && m.Proto != "-" {
			return m.Proto
		}
	}
	return ""
}

// diaryConfModel is the model ai.conf names (or the default).
func (a *bro) diaryConfModel() string {
	c, _ := ai.Load(kobo.AppFile("ai.conf"))
	if c.Model == "" {
		return ai.DefaultModel
	}
	return c.Model
}

// noteSettingsRedraw draws the notebooks' settings again where they
// show: a choice's list may have changed under them.
func (a *bro) noteSettingsRedraw() {
	for _, w := range a.noteWins() {
		if w.setBody(a) != nil {
			w.showSettings(a)
		}
	}
}

// diaryConf is ai.conf, with the model the settings choose.
func (a *bro) diaryConf() (ai.Config, error) {
	c, err := ai.Load(kobo.AppFile("ai.conf"))
	if m := a.setting("ai-model"); m != "conf" && m != "" {
		c.Model, c.Proto = m, ""
	}
	if c.Proto == "" {
		c.Proto = a.modelProto(c.Model)
	}
	c.MaxOut = max(c.MaxOut, diaryMaxOut)
	c.Agent = "hellobro/" + version // as OpenCode asks of the apps calling it
	return c, err
}

// The answer's letters, points: "ai-size" is one of these, 15 at first.
var (
	diarySizeValues = []string{"10", "11", "12", "13", "14", "15", "16", "17", "19", "21", "24", "28", "32"}
	diarySizeShown  = func() []string {
		var ss []string
		for _, v := range diarySizeValues {
			ss = append(ss, v+" pt")
		}
		return ss
	}()
)

const diaryPt = 15 // the answer's letters, if nothing says

// diarySize is the answers' letters, points, as the setting says.
func (a *bro) diarySize() float64 {
	if f, err := strconv.ParseFloat(a.setting("ai-size"), 64); err == nil && f > 0 {
		return f
	}
	return diaryPt
}

// The answer's font is the notebook's own: Gentium Book (Latin, Greek
// and Cyrillic whole), Go's, one of the reader's; or the Hershey
// script's hand.
const (
	diaryFontAttr = "hellobro-diary-font"
	diaryHand     = "handwriting"
)

// diaryFonts are the fonts an answer can be in.
func diaryFonts() []string {
	fs := []string{famGentium, famGo}
	for _, f := range noteFamilies() {
		if f.files[0] != "" && !slices.Contains(fs, f.name) {
			fs = append(fs, f.name)
		}
	}
	return append(fs, diaryHand)
}

// diaryFont is nb's answers' font.
func (nb *notebook) diaryFont() string {
	if f := attr(nb.attrs, diaryFontAttr); f != "" {
		return f
	}
	return famGentium
}

// setDiaryFont makes f this notebook's answers' font.
func (w *noteWin) setDiaryFont(f string) {
	w.nb.attrs = slices.DeleteFunc(w.nb.attrs, func(a xml.Attr) bool { return a.Name.Local == diaryFontAttr })
	w.nb.attrs = append(w.nb.attrs, xml.Attr{Name: xml.Name{Local: diaryFontAttr}, Value: f})
	w.changed = time.Now()
}

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

	// While it's asked, a badge where the answer will go: black, a
	// spinner turning, "thinking", and how long it's been at it. On
	// screen only.
	think     image.Rectangle // page units: its top left; empty: none
	thinkPage int
	dots      int // steps so far: the spinner's, the dots'
	dotAt     time.Time
	thinkAt   time.Time
}

const diaryDotEvery = 700 * time.Millisecond

// diaryTapper has a finger's taps on a page, the last few: quick ones
// ask the diary at once ("ai-taps").
type diaryTapper struct {
	at   []time.Time
	last image.Point
}

// diaryWord is a word of an answer: its strokes, on its page.
type diaryWord struct {
	page    int
	strokes []*noteStroke // the hand's
	line    *noteStroke   // or a font's: the line it's on, its text that far
	text    string
	first   bool // the line's first word: it goes on the page
}

func (nb *notebook) diaryOn() bool { return attr(nb.attrs, diaryOnAttr) == "on" }

func (nb *notebook) setDiary(on bool) {
	nb.attrs = slices.DeleteFunc(nb.attrs, func(a xml.Attr) bool { return a.Name.Local == diaryOnAttr })
	if on {
		nb.attrs = append(nb.attrs, xml.Attr{Name: xml.Name{Local: diaryOnAttr}, Value: "on"})
	}
}

func onOff(on bool) string { return map[bool]string{true: "on", false: "off"}[on] }

func isDiary(s *noteStroke) bool { return attr(strokeAttrs(s), diaryAttr) == "diary" }

// strokeAttrs are s's attributes in the file: a text's are its own.
func strokeAttrs(s *noteStroke) []xml.Attr {
	if s.txt != nil {
		return s.txt.attrs
	}
	return s.attrs
}

// flipDiary turns w's diary on or off.
func (w *noteWin) flipDiary(a *bro) {
	on := !w.nb.diaryOn()
	w.nb.setDiary(on)
	w.changed = time.Now()
	if !on {
		w.diaryStop(a)
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
func (w *noteWin) diaryStop(a *bro) {
	d := w.diary
	if d == nil {
		return
	}
	if d.cancel != nil {
		d.cancel()
	}
	w.diaryUnthink(a)
	d.asked, d.cancel, d.wroteAt = 0, nil, time.Time{}
	if len(d.wrote) > 0 {
		w.diaryDone(nil)
	}
	d.writing = nil
}

// diaryWrote notes a stroke written on page: once the pen rests, it's
// asked; a question out is asked again, with it.
func (w *noteWin) diaryWrote(a *bro, page int) {
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
		w.diaryUnthink(a)
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
		case d.asked != 0:
			if !d.think.Empty() && time.Since(d.dotAt) >= diaryDotEvery {
				d.dots, d.dotAt = d.dots+1, time.Now()
				w.diaryShowDots(a, false)
			}
		case d.asked == 0 && !d.wroteAt.IsZero() && a.diaryPause() > 0 && time.Since(d.wroteAt) >= a.diaryPause() &&
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

Your answer is written back into the notebook, so: answer in the language they wrote in, unless told otherwise; short, a few sentences, unless they ask for more; FORMAT. If you can't read a word, ask about it.`

// diaryFormats are how an answer's to be written: in the hand, in a
// font as plain text, in Markdown.
var diaryFormats = map[string]string{
	"hand":  "plain text, no Markdown, no LaTeX: write formulas plainly, as x^2 + 1, sqrt(2); new lines only between paragraphs",
	"plain": "plain text, no Markdown, no LaTeX: write formulas plainly, with Unicode's signs where they help: x² + 1, √2, π, ≤, →; new lines only between paragraphs",
	"md": "Markdown where it helps: **bold**, *italic*, lists, tables, `code` and fenced code blocks (```), a heading only in a long answer; " +
		"formulas in LaTeX, $...$ in a line and $$...$$ on a line of their own; a diagram, where one helps, as a ```mermaid block " +
		"(flowchart, sequenceDiagram, stateDiagram-v2, classDiagram, erDiagram, gantt, timeline, mindmap, pie or xychart-beta; labels with brackets or punctuation in quotes); " +
		"a graph of functions as a ```plot block, a line each: title Name, x: -5 .. 5, y: -2 .. 2 if it matters, then each function as y = x^2 - 1 : name " +
		"(+ - * / ^, sqrt, sin, cos, tan, exp, ln, log10, abs, pi), points: (1, 2) (3, 4) : name, x = 2 for an upright line, xlabel: t, s. In the JSON, escape each backslash (\\\\frac, \\\\alpha) and write new lines as \\n",
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
			as := strokeAttrs(s)
			ans := attr(as, diaryAAttr)
			if ans == "" {
				continue
			}
			q := attr(as, diaryQAttr)
			if q == "" {
				q = "(not read)"
			}
			p := attr(as, diaryPromptAttr)
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

// diaryFormat is how nb's answers are written: "hand", "plain" text,
// "md" (Markdown).
func (a *bro) diaryFormat(nb *notebook) string {
	switch {
	case nb.diaryFont() == diaryHand:
		return "hand"
	case a.setting("ai-format") == "plain":
		return "plain"
	}
	return "md"
}

// diarySystem is what the model's told for nb: the rules, its voice, the
// writer's instructions.
func diarySystem(nb *notebook, prompts []string, format string) string {
	var b strings.Builder
	b.WriteString(nb.voiceText() + "\n\n" + strings.Replace(diaryRules, "FORMAT", diaryFormats[format], 1))
	if autoNamed(nb.name) {
		b.WriteString("\n- The notebook has no name yet: with your answer, add \"title\":\"<one to three words naming what it's about, in the writer's language, no punctuation>\".")
	}
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
	Title    string `json:"title,omitempty"` // the notebook's name, asked for with its first question
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
	taps := a.diaryTaps()
	if taps == 0 || len(t.at) < taps {
		return
	}
	t.at = nil
	if !w.nb.diaryOn() {
		a.Notify(map[int]string{2: "two", 3: "three"}[taps] + " taps ask the AI diary: it's off (the notebook's settings)")
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
		w.diaryUnthink(a)
	}
	if page >= len(w.nb.pages) || len(diaryQuestion(w.nb.pages[page])) == 0 { // tapped by a page with nothing new: the one written on
		page = d.page
	}
	d.page, d.wroteAt = page, time.Time{}
	w.diaryAsk(a, true)
}

// diaryPause is how long the pen rests before what's written is asked
// ("ai-pause"); 0: it isn't, only taps ask.
func (a *bro) diaryPause() time.Duration {
	n, err := strconv.Atoi(a.setting("ai-pause"))
	if err != nil {
		return 0
	}
	return time.Duration(n) * time.Second
}

// diaryTaps is how many quick taps of a finger ask now ("ai-taps"); 0:
// taps don't.
func (a *bro) diaryTaps() int {
	n, _ := strconv.Atoi(a.setting("ai-taps"))
	return n
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
	h := fnv.New64a()
	h.Write([]byte(w.nb.name))
	cfg.Session = fmt.Sprintf("hellobro-%x", h.Sum64())
	say := "Written now:"
	if now {
		say = "Written now (they asked for an answer at once: answer it, finished or not):"
		a.Notify("diary: asking " + cfg.Model + "…")
	}
	talk, prompts := diaryTalk(w.nb)
	format := a.diaryFormat(w.nb)
	system := diarySystem(w.nb, prompts, format)
	msgs := append(talk[max(0, len(talk)-diaryHistory):],
		ai.Msg{Role: "user", Parts: []ai.Part{{Text: say}, {PNG: pic}}})
	style, size := w.nb.diaryFont(), a.diarySize()
	ctx, cancel := context.WithCancel(context.Background())
	d.asked++
	n, page, below := d.asked, d.page, boxOf(q)
	d.cancel = cancel
	log.Printf("diary %s: asking %s (%d strokes, a %d byte picture)", w.nb.name, cfg.Model, len(q), len(pic))
	w.diaryThink(a, page, below, size/notePt72*0.7)
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
			w.diaryUnthink(a)
			w.diaryAnswer(a, page, below, out.Text, err, style, size, format == "md")
		})
	}()
}

// diaryAnswer takes the model's answer to what's in below on page.
func (w *noteWin) diaryAnswer(a *bro, page int, below image.Rectangle, out string, err error, style string, size float64, md bool) {
	if err != nil {
		// Said on the page, where the answer would be: a notice goes by
		// unseen while the eyes are on the page. Not part of the talk.
		log.Printf("diary %s: %v", w.nb.name, err)
		if style == diaryHand {
			style = famGentium
		}
		w.diaryLay(page, below, diaryReply{Status: "error", Answer: "(no answer: " + diaryErrText(err) + ")"}, time.Now().UnixNano(), style, size)
		w.diary.nextAt = time.Now()
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
	w.diaryName(a, r.Title)
	if md {
		w.diaryLayMD(page, below, r, style, size)
	} else {
		w.diaryLay(page, below, r, time.Now().UnixNano(), style, size)
	}
	w.diary.nextAt = time.Now()
}

// diaryLay lays the answer out under below on page, to be written: on
// the next page where it's full (a page put in after it, unless the
// next is empty). In fam, a font (Go's, or the reader's), a line of
// text at a time; or the hand.
func (w *noteWin) diaryLay(page int, below image.Rectangle, r diaryReply, seed int64, fam string, pt float64) {
	if pt == 0 {
		pt = diaryPt
	}
	if fam == diaryHand {
		w.diaryLayHand(page, below, r, seed, pt/notePt72)
		return
	}
	d := w.diary
	f := noteFontFace(fam, int(pt/notePt72+0.5))
	lineH := f.Metrics().Height.Ceil()
	size := w.pageSize(page)
	left := diaryLeft(below, size)
	width := size.X - diaryMargin - left
	y := below.Max.Y + lineH/2
	if below.Empty() {
		y = diaryMargin
	}
	first := true
	for para := range strings.SplitSeq(strings.ReplaceAll(superscripts(r.Answer, f), "\r", ""), "\n") {
		words := strings.Fields(para)
		if len(words) == 0 {
			y += lineH / 2
			continue
		}
		for len(words) > 0 {
			n := 1 // the words on this line
			for n < len(words) && font.MeasureString(f, strings.Join(words[:n+1], " ")).Ceil() <= width {
				n++
			}
			if y+lineH > size.Y-diaryMargin/2 && !w.rollRoom(page, y+lineH) {
				page = w.diaryPage(page)
				size, y = w.pageSize(page), diaryMargin
			}
			as := []xml.Attr{{Name: xml.Name{Local: "font"}, Value: fam}, {Name: xml.Name{Local: "color"}, Value: "#000000ff"},
				{Name: xml.Name{Local: diaryAttr}, Value: "diary"}}
			if first && r.Status != "error" {
				as = append(as, talkAttrs(r)...)
			}
			first = false
			line := textStroke(&noteText{size: pt, attrs: as}, image.Pt(left, y))
			for j := 1; j <= n; j++ {
				d.writing = append(d.writing, diaryWord{page: page, line: line, text: strings.Join(words[:j], " "), first: j == 1})
			}
			words, y = words[n:], y+lineH
		}
	}
}

// diaryErrText is what went wrong asking, short enough for a page.
func diaryErrText(err error) string {
	s := strings.Join(strings.Fields(err.Error()), " ")
	if r := []rune(s); len(r) > 300 {
		s = string(r[:300]) + "…"
	}
	return s
}

// talkAttrs are what an answer's first stroke keeps of it, for the
// talk.
func talkAttrs(r diaryReply) []xml.Attr {
	as := []xml.Attr{{Name: xml.Name{Local: diaryQAttr}, Value: r.Question}, {Name: xml.Name{Local: diaryAAttr}, Value: r.Answer}}
	if r.Prompt != "" {
		as = append(as, xml.Attr{Name: xml.Name{Local: diaryPromptAttr}, Value: r.Prompt})
	}
	return as
}

// supers are the signs with a superscript of their own.
var supers = map[rune]rune{'0': '⁰', '1': '¹', '2': '²', '3': '³', '4': '⁴', '5': '⁵', '6': '⁶', '7': '⁷', '8': '⁸', '9': '⁹',
	'+': '⁺', '-': '⁻', '=': '⁼', '(': '⁽', ')': '⁾', 'n': 'ⁿ', 'i': 'ⁱ'}

var supRE = regexp.MustCompile(`\^(\{[^{}]*\}|\([^()]*\)|[-+]?[0-9]+|[ni])`)

// superscripts writes x^2, 2^{10}, e^(-n) as x², 2¹⁰, e⁽⁻ⁿ⁾, where f has
// the signs; what it can't stays as it is.
func superscripts(s string, f font.Face) string {
	return supRE.ReplaceAllStringFunc(s, func(m string) string {
		in := strings.TrimSuffix(strings.TrimPrefix(m[1:], "{"), "}")
		var b strings.Builder
		for _, r := range in {
			sr, ok := supers[r]
			if _, has := f.GlyphAdvance(sr); !ok || !has {
				return m
			}
			b.WriteRune(sr)
		}
		return b.String()
	})
}

// diaryLayHand lays the answer out in the hand, capitals size high
// (page units).
func (w *noteWin) diaryLayHand(page int, below image.Rectangle, r diaryReply, seed int64, diarySize float64) {
	text := r.Answer
	d := w.diary
	h, err := hand.Script()
	if err != nil {
		log.Printf("diary: %v", err)
		return
	}
	size := w.pageSize(page)
	left := diaryLeft(below, size)
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
			if foot := dy + base + diarySize*0.5; foot > float64(size.Y-diaryMargin/2) && !w.rollRoom(page, int(foot)) {
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
				s.attrs, first = append(s.attrs, talkAttrs(r)...), false
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
	m := w.nb.newMeta() // as new pages are (not a roll's grown page)
	if page < len(w.nb.meta) && w.nb.pdfPath() == "" {
		m.bg = w.nb.meta[page].bg
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
	add, long := wd.strokes, len(wd.strokes)
	var was image.Rectangle // a line's, before its word
	if wd.line != nil {
		was = wd.line.box
		wd.line.txt.s = wd.text
		wd.line.bound()
		add, long = nil, utf8.RuneCountInString(wd.text[strings.LastIndex(wd.text, " ")+1:])
		if wd.first {
			add = []*noteStroke{wd.line}
		}
	}
	w.nb.pages[wd.page] = append(w.nb.pages[wd.page], add...)
	if len(add) == 0 {
	} else if n := len(d.wrote); n > 0 && d.wrote[n-1].kind == pageStrokes && d.wrote[n-1].page == wd.page {
		d.wrote[n-1].add = append(d.wrote[n-1].add, add...)
	} else {
		d.wrote = append(d.wrote, noteOp{page: wd.page, add: slices.Clone(add)})
	}
	d.nextAt = time.Now().Add(diaryWordTime + time.Duration(min(long, 8))*diaryStrokeTime) // as long a word, whatever the hand
	w.changed = time.Now()                                                                 // kept, should the window close before it's done
	if !w.p.Outer.Empty() && a.shows(w) && !w.overview && !w.settings {
		idle := a.nt.ink == nil && a.nt.drag == nil && w.typing == nil
		if w.roll() && idle { // the roll follows it, a screenful at a time
			box := boxOf(wd.strokes)
			if wd.line != nil {
				box = wd.line.box
			}
			w.rollShow(a, wd.page, box.Min.Y, box.Max.Y)
		} else if wd.page == w.cur+w.shown() && idle {
			w.cur = wd.page
			w.show(a)
			a.MarkFlash(w.pages())
		}
		if i := wd.page - w.cur; i >= 0 && i < len(w.slots) {
			at, clip := w.slots[i], w.paperArea()
			var box image.Rectangle
			if wd.line != nil { // the line again, its word on
				box = w.toScreenRect(was.Union(wd.line.box), at)
				box.Max.X += box.Dy() // drawn smaller, the letters may come out a little wider than measured
				box = box.Intersect(clip)
				w.drawPages(a, box)
			}
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
		w.showBar(a)
	}
	d.box = image.Rectangle{}
}

// diaryLeft is where an answer's lines start on a page size wide,
// under what's in below.
func diaryLeft(below image.Rectangle, size image.Point) int {
	return max(diaryMargin, min(below.Min.X, size.X/4))
}

// diaryThink puts the dots up, under below on page, where the answer's
// to go, for letters size high.
func (w *noteWin) diaryThink(a *bro, page int, below image.Rectangle, size float64) {
	if size == 0 {
		size = diaryPt
	}
	d := w.diary
	x, y := float64(diaryLeft(below, w.pageSize(page))), float64(below.Max.Y)+hand.LineHeight(size)*0.5+size*0.75
	r, gap := size*0.09, size*0.55
	d.think = image.Rect(int(x-r), int(y-r), int(x+2*gap+r)+1, int(y+r)+1)
	d.thinkPage, d.dots, d.dotAt, d.thinkAt = page, 0, time.Now(), time.Now()
	w.diaryShowDots(a, false)
}

// diaryUnthink takes the dots down.
func (w *noteWin) diaryUnthink(a *bro) {
	if d := w.diary; d != nil && !d.think.Empty() {
		w.diaryShowDots(a, true)
		d.think = image.Rectangle{}
	}
}

// thinkRect is where the badge is on screen; empty if its page
// doesn't show.
func (w *noteWin) thinkRect() image.Rectangle {
	d := w.diary
	if d == nil || d.think.Empty() || w.overview || w.settings {
		return image.Rectangle{}
	}
	i := d.thinkPage - w.cur
	if i < 0 || i >= len(w.slots) {
		return image.Rectangle{}
	}
	at := w.toScreen(notePt{int16(d.think.Min.X), int16(d.think.Min.Y), 0}, w.slots[i])
	r := image.Rectangle{Max: thinkSize()}.Add(at)
	area := w.paperArea().Inset(8)
	return r.Add(into(r, area)).Intersect(area)
}

// thinkSize is the badge's: as wide as its longest words.
var thinkSize = sync.OnceValue(func() image.Point {
	f := kobo.LoadFonts().TabBold
	return image.Pt(thinkH+kobo.TextWidth(f, "thinking...  999 s")+thinkH/2, thinkH)
})

const thinkH = 60 // px, the badge's height

// diaryShowDots draws the badge again where it is (gone, if it's to
// go), quick: and with the page's grays when it goes.
func (w *noteWin) diaryShowDots(a *bro, gone bool) {
	r := w.thinkRect()
	if r.Empty() || a == nil || w.p.Outer.Empty() || !a.shows(w) {
		return
	}
	if gone {
		w.diary.think = image.Rectangle{}
	}
	w.drawPages(a, r)
	a.Mark(r, gone)
}

// drawThinking draws the badge, if it's up, where it's in clip: a
// spinner of eight dots, one big and turning; "thinking", a dot more
// each step; the seconds since it was asked.
func (w *noteWin) drawThinking(a *bro, clip image.Rectangle) {
	r := w.thinkRect()
	if r.Empty() || !r.Overlaps(clip) {
		return
	}
	d := w.diary
	clip = clip.Intersect(r)
	c := a.C
	c.Fill(clip, kobo.Black)
	c.Frame(r.Inset(4), 2, kobo.White)
	mid := image.Pt(r.Min.X+thinkH/2+4, r.Min.Y+thinkH/2)
	for k := range 8 {
		t := float64(k) * math.Pi / 4
		p := mid.Add(image.Pt(int(15*math.Cos(t)), int(15*math.Sin(t))))
		rad := 3
		if k == d.dots%8 {
			rad = 7
		}
		c.Disc(p, rad, kobo.White, clip)
	}
	words := "thinking" + strings.Repeat(".", d.dots%3+1)
	if secs := int(time.Since(d.thinkAt).Seconds()); secs >= 1 {
		words += fmt.Sprintf("  %d s", secs)
	}
	f := a.F.TabBold
	c.TextAt(f, r.Min.X+thinkH+8, r.Min.Y+(thinkH+f.Metrics().Ascent.Ceil()-f.Metrics().Descent.Ceil())/2, words, kobo.White, clip)
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
	lim := diaryLimits[m]
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
	if lim := diaryLimits[m]; lim > 0 {
		return fmt.Sprintf("AI usage: %s's, $%.3f this month, of the Go plan's $%.0f (a fifth in 5 h, half a week); this Kobo's only", m, month, lim)
	}
	return fmt.Sprintf("AI usage: %s's, in dollars, this Kobo's questions only", m)
}

// autoNamedRE is a new notebook's name, as newNoteName gives it.
var autoNamedRE = regexp.MustCompile(`^\d{4}-\d{2}-\d{2} \d{2}\.\d{2}( \(\d+\))?$`)

// autoNamed is whether a notebook still has the name it was made with
// (its date): the diary names it, then, by its first question.
func autoNamed(name string) bool { return autoNamedRE.MatchString(path.Base(name)) }

// titleName is a model's title as a notebook's name: one to three
// words, no signs a file's name can't have, not too long.
func titleName(t string) string {
	t = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`/\:*?"<>|`, r) || unicode.IsControl(r) {
			return ' '
		}
		return r
	}, t)
	f := strings.Fields(strings.Trim(t, " .'`«»"))
	if len(f) > 3 {
		f = f[:3]
	}
	t = strings.TrimLeft(strings.Join(f, " "), ".")
	for len([]rune(t)) > 40 {
		t = string([]rune(t)[:40])
	}
	return strings.TrimSpace(t)
}

// diaryName names the notebook by title, the model's for its first
// question, if it still has its date for a name: in its folder, a
// number after it if the name's taken.
func (w *noteWin) diaryName(a *bro, title string) {
	name := titleName(title)
	if name == "" || !autoNamed(w.nb.name) {
		return
	}
	if dir := path.Dir(w.nb.name); dir != "." {
		name = dir + "/" + name
	}
	to := name
	for i := 2; fileExists(notePath(to)) || a.noteWindow(to) != nil; i++ {
		to = fmt.Sprintf("%s (%d)", name, i)
	}
	from := w.nb.name
	if err := renameNotebook(a, from, to); err != nil {
		log.Printf("diary %s: naming it %q: %v", from, to, err)
		return
	}
	log.Printf("diary %s: named %q", from, to)
	a.Notify("named it: " + path.Base(to))
}
