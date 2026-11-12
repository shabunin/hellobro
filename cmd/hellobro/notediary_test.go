package main

import (
	"bytes"
	"encoding/xml"
	"errors"
	"image"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

func lineAt(y int16, diary bool) *noteStroke {
	s := &noteStroke{pts: []notePt{{100, y, 20}, {500, y, 20}}}
	if diary {
		s.attrs = []xml.Attr{{Name: xml.Name{Local: diaryAttr}, Value: "diary"}}
	}
	s.bound()
	return s
}

func TestDiaryQuestion(t *testing.T) {
	q1, ans, q2, q3 := lineAt(100, false), lineAt(200, true), lineAt(300, false), lineAt(400, false)
	if got := diaryQuestion([]*noteStroke{q1, ans, q2, q3}); !slices.Equal(got, []*noteStroke{q2, q3}) {
		t.Errorf("under the answer: %v", got)
	}
	if got := diaryQuestion([]*noteStroke{q1, q2}); len(got) != 2 {
		t.Errorf("no answer yet: %v", got)
	}
	if got := diaryQuestion([]*noteStroke{q1, ans}); len(got) != 0 {
		t.Errorf("all answered: %v", got)
	}
}

// TestDiaryLay: an answer under a question low on the page goes on on a
// page put in after it, and is one step to undo.
func TestDiaryLay(t *testing.T) {
	q := lineAt(notePageH-300, false)
	after := lineAt(200, false)
	w := &noteWin{nb: &notebook{pages: [][]*noteStroke{{q}, {after}}}, diary: &noteDiary{}}
	w.nb.padMeta()
	w.diaryLay(0, q.box, diaryReply{Status: "answer", Question: "a story?", Answer: strings.Repeat("It was a dark and stormy night, and the diary answered. ", 6), Prompt: "call me Harry"}, 1, "Go", 15)
	pages := map[int]bool{}
	for _, wd := range w.diary.writing {
		pages[wd.page] = true
		for _, s := range wd.strokes {
			if !isDiary(s) || !s.box.In(image.Rect(0, 0, notePageW, notePageH)) {
				t.Fatalf("a stroke off the page, or not the diary's: %v", s.box)
			}
			if wd.page == 0 && s.box.Min.Y < q.box.Max.Y {
				t.Errorf("a stroke over the question: %v", s.box)
			}
		}
	}
	if !pages[0] || !pages[1] || len(w.nb.pages) != 3 || len(w.nb.pages[1]) != 0 || w.nb.pages[2][0] != after {
		t.Fatalf("not on a page put in: on %v, %d pages", pages, len(w.nb.pages))
	}
	a := &bro{}
	for len(w.diary.writing) > 0 {
		w.diaryWriteWord(a)
	}
	if len(w.undo) != 1 || len(w.nb.pages[1]) == 0 {
		t.Fatalf("%d steps, %d strokes on the new page", len(w.undo), len(w.nb.pages[1]))
	}
	talk, prompts := diaryTalk(w.nb)
	if len(talk) != 2 || !strings.Contains(talk[0].Parts[0].Text, "a story?") || !strings.Contains(talk[1].Parts[0].Text, "stormy") || !slices.Equal(prompts, []string{"call me Harry"}) {
		t.Errorf("the talk: %+v, %q", talk, prompts)
	}
	if sys := diarySystem(w.nb, prompts, "md"); !strings.Contains(sys, "Tom Riddle") || !strings.Contains(sys, "- call me Harry") {
		t.Errorf("the system prompt: %s", sys)
	}
	w.doUndoOps()
	if talk, prompts := diaryTalk(w.nb); len(talk) != 0 || len(prompts) != 0 {
		t.Errorf("undone, but still in the talk: %+v", talk)
	}
	if len(w.nb.pages) != 2 || !slices.Equal(w.nb.pages[0], []*noteStroke{q}) || w.nb.pages[1][0] != after {
		t.Errorf("undone: %d pages, %v", len(w.nb.pages), w.nb.pages[0])
	}
}

// doUndoOps undoes the last step, as doUndo does, without a screen.
func (w *noteWin) doUndoOps() {
	ops := w.undo[len(w.undo)-1]
	w.undo = w.undo[:len(w.undo)-1]
	for i := len(ops) - 1; i >= 0; i-- {
		w.nb.apply(ops[i], true)
	}
}

func TestDiaryUsage(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	at := func(ago time.Duration) string { return strconv.FormatInt(now.Add(-ago).Unix(), 10) }
	data := at(time.Hour) + "\tm\t0.5\t10\t20\n" +
		at(2*24*time.Hour) + "\tm\t0.25\t1\t2\n" +
		at(10*24*time.Hour) + "\tm\t1\t1\t2\n" +
		at(40*24*time.Hour) + "\tm\t8\t1\t2\n" +
		at(time.Hour) + "\tother\t3\t1\t2\n" +
		"garbage\n"
	h5, week, month := diaryUsage([]byte(data), "m", now)
	if h5 != 0.5 || week != 0.75 || month != 1.75 {
		t.Errorf("5 h %v, week %v, month %v", h5, week, month)
	}
}

func TestDiaryModels(t *testing.T) {
	ids := diaryModelIDs()
	if ids[0] != "conf" || !slices.Contains(ids, "claude-haiku-5-5") {
		t.Errorf("models %v", ids)
	}
	if diaryModelOf("glm-5.3-flash").limit != 60 || diaryModelOf("nonesuch").limit != 0 {
		t.Error("limits")
	}
}

// TestDiaryKept: the talk and the voice are in the file, as written.
func TestDiaryKept(t *testing.T) {
	s := lineAt(300, true)
	s.attrs = append(s.attrs, xml.Attr{Name: xml.Name{Local: diaryQAttr}, Value: `who "are" you?`},
		xml.Attr{Name: xml.Name{Local: diaryAAttr}, Value: "Tom.\n\nПривет & <bye>"})
	w := &noteWin{nb: &notebook{name: "t", pages: [][]*noteStroke{{s}}}}
	w.nb.padMeta()
	w.nb.setDiary(true)
	w.stepVoice(1)
	nb, err := parseXopp("t", bytes.NewReader(w.nb.xoppText()))
	if err != nil {
		t.Fatal(err)
	}
	talk, _ := diaryTalk(nb)
	if !nb.diaryOn() || nb.diaryVoice() != 1 || len(talk) != 2 || talk[0].Parts[0].Text != `Written (read): who "are" you?` || !strings.Contains(talk[1].Parts[0].Text, `Tom.\n\nПривет \u0026 \u003cbye\u003e`) {
		t.Errorf("read back: on %v, voice %d, %+v", nb.diaryOn(), nb.diaryVoice(), talk)
	}
}

// TestDiaryCut: undo while an answer's being written stops it, and takes
// back what's written of it.
func TestDiaryCut(t *testing.T) {
	q := lineAt(200, false)
	w := &noteWin{nb: &notebook{pages: [][]*noteStroke{{q}}}, diary: &noteDiary{}}
	w.nb.padMeta()
	w.diaryLay(0, q.box, diaryReply{Status: "answer", Answer: "one two three four five six"}, 1, "handwriting", 15)
	a := &bro{}
	for range 3 {
		w.diaryWriteWord(a)
	}
	w.diaryCut(a)
	if len(w.diary.writing) != 0 || len(w.undo) != 1 {
		t.Fatalf("%d words still to write, %d steps", len(w.diary.writing), len(w.undo))
	}
	w.doUndoOps()
	if !slices.Equal(w.nb.pages[0], []*noteStroke{q}) {
		t.Errorf("undone: %d strokes", len(w.nb.pages[0]))
	}
}

func TestSuperscripts(t *testing.T) {
	f := noteTextFace(40)
	for in, want := range map[string]string{
		"x^2 + y^2 = r^2": "x² + y² = r²",
		"2^{10}, e^(-n)":  "2¹⁰, e⁽⁻ⁿ⁾",
		"10^-3":           "10⁻³",
		"a^b stays":       "a^b stays",
		"caret ^ alone":   "caret ^ alone",
	} {
		if got := superscripts(in, f); got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
}

// TestDiaryLayText: an answer in a font is lines of text, on the page,
// under the question, each word a step longer.
func TestDiaryLayText(t *testing.T) {
	q := lineAt(300, false)
	w := &noteWin{nb: &notebook{pages: [][]*noteStroke{{q}}}, diary: &noteDiary{}}
	w.nb.padMeta()
	w.diaryLay(0, q.box, diaryReply{Status: "answer", Question: "q?", Answer: strings.Repeat("words and more words ", 30) + "\n\nx^2 the end."}, 1, "Go", 15)
	lines := map[*noteStroke]string{}
	for _, wd := range w.diary.writing {
		if wd.line == nil || wd.first != (lines[wd.line] == "") {
			t.Fatalf("a word not on a line, or its line's first not first: %+v", wd)
		}
		lines[wd.line] = wd.text
	}
	if len(lines) < 4 {
		t.Fatalf("%d lines", len(lines))
	}
	a := &bro{}
	for len(w.diary.writing) > 0 {
		w.diaryWriteWord(a)
	}
	for l, text := range lines {
		if l.txt.s != text || !isDiary(l) || !l.box.In(image.Rect(0, q.box.Max.Y, notePageW, notePageH)) {
			t.Errorf("a line %q at %v", l.txt.s, l.box)
		}
	}
	if talk, _ := diaryTalk(w.nb); len(talk) != 2 {
		t.Errorf("the talk: %+v", talk)
	}
	if !strings.Contains(w.nb.pages[0][len(w.nb.pages[0])-1].txt.s, "x² the end.") {
		t.Errorf("the last line: %q", w.nb.pages[0][len(w.nb.pages[0])-1].txt.s)
	}
}

// A notebook with its date for a name is named by its first question's
// title, one to three words a file's name can have; one named by hand
// isn't, and isn't asked for one.
func TestDiaryTitle(t *testing.T) {
	for name, want := range map[string]bool{"2026-11-12 09.21": true, "2026-11-12 09.21 (2)": true, "school/2026-11-12 09.21": true,
		"Physics": false, "2026-11-12 09.21 notes": false} {
		if autoNamed(name) != want {
			t.Errorf("%q: auto-named %v", name, !want)
		}
	}
	for in, want := range map[string]string{"Quadratic equations": "Quadratic equations", " «Tom: the diary» ": "Tom the diary",
		"one two three four": "one two three", "a/b?": "a b", "...": "", "Квадратные уравнения": "Квадратные уравнения"} {
		if got := titleName(in); got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
	nb := &notebook{name: "2026-11-12 09.21"}
	if !strings.Contains(diarySystem(nb, nil, "md"), `"title"`) {
		t.Error("a date for a name: no title asked for")
	}
	nb.name = "Physics"
	if strings.Contains(diarySystem(nb, nil, "md"), `"title"`) {
		t.Error("named by hand: a title asked for")
	}
}

// An answer that didn't come is said on the page, where it'd be, and
// isn't part of the talk.
func TestDiaryError(t *testing.T) {
	q := lineAt(300, false)
	w := &noteWin{nb: &notebook{pages: [][]*noteStroke{{q}}}, diary: &noteDiary{}}
	w.nb.padMeta()
	a := &bro{}
	w.diaryAnswer(a, 0, q.box, "", errors.New("ai: 401 Unauthorized: {\"error\": \"bad key\"}"), diaryHand, 15, true)
	for len(w.diary.writing) > 0 {
		w.diaryWriteWord(a)
	}
	pg := w.nb.pages[0]
	if last := pg[len(pg)-1]; last.txt == nil || !strings.Contains(last.txt.s, "401 Unauthorized") || !strings.HasPrefix(pg[1].txt.s, "(no answer:") {
		t.Fatalf("the page: %d strokes, the last %+v", len(pg), last.txt)
	}
	if talk, _ := diaryTalk(w.nb); len(talk) != 0 {
		t.Errorf("the error is in the talk: %+v", talk)
	}
}
