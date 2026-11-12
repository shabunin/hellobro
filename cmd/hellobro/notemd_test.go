package main

import (
	"bytes"
	"image"
	"strings"
	"testing"

	"hellobro/md"
)

const mdAnswer = "## Solving\n\nThe roots of $ax^2+bx+c=0$ are **two**:\n\n$$x = \\frac{-b \\pm \\sqrt{b^2-4ac}}{2a}$$\n\n" +
	"- one *item*\n- two `code`\n  1. deeper\n\n```go\nfunc f(x float64) float64 {\n\treturn x*x // a long comment that goes on and on past the frame's edge for sure\n}\n```\n\n" +
	"| a | b |\n|---|--:|\n| Привет | αβγ |\n\n> a quote\n\n```mermaid\ngraph LR\n  A[Начало] -->|да| B{$x > 0$?}\n```\n\n" +
	"```mermaid\ngantt\n  title not drawn: code\n```\n\n---\nThe end."

func TestDiaryLayMD(t *testing.T) {
	q := lineAt(300, false)
	w := &noteWin{nb: &notebook{name: "t", pages: [][]*noteStroke{{q}}}, diary: &noteDiary{}}
	w.nb.padMeta()
	w.diaryLayMD(0, q.box, diaryReply{Status: "answer", Question: "roots?", Answer: mdAnswer}, famGentium, 15)
	if len(w.diary.writing) < 20 {
		t.Fatalf("%d words", len(w.diary.writing))
	}
	a := &bro{}
	for len(w.diary.writing) > 0 {
		w.diaryWriteWord(a)
	}
	var texts []string
	pics, lines, diagrams := 0, 0, 0
	for _, pg := range w.nb.pages {
		for _, s := range pg[:] {
			if s == q {
				continue
			}
			if !isDiary(s) || !s.box.In(image.Rect(0, 0, notePageW, notePageH)) {
				t.Errorf("not the diary's, or off the page: %v %v", s.box, strokeAttrs(s))
			}
			switch {
			case s.txt != nil:
				texts = append(texts, s.txt.s)
			case s.pic != nil:
				pics++
				if attr(s.attrs, mermaidAttr) != "" {
					diagrams++
				} else if attr(s.attrs, texAttr) == "" {
					t.Error("a formula without its source")
				}
			default:
				lines++
			}
		}
	}
	all := strings.Join(texts, "|")
	for _, want := range []string{"Solving", "two", "•", "item", "code", "1.", "func f(x float64) float64 {", "Привет", "αβγ", "a quote", "gantt", "The end."} {
		if !strings.Contains(all, want) {
			t.Errorf("no %q in %s", want, all)
		}
	}
	if pics != 3 || diagrams != 1 || lines < 4 {
		t.Errorf("%d pictures, %d diagrams, %d lines (frame, rules, bar)", pics, diagrams, lines)
	}
	if talk, _ := diaryTalk(w.nb); len(talk) != 2 {
		t.Errorf("the talk: %+v", talk)
	}
	nb, err := parseXopp("t", bytes.NewReader(w.nb.xoppText()))
	if err != nil {
		t.Fatal(err)
	}
	if talk, _ := diaryTalk(nb); len(talk) != 2 {
		t.Errorf("the talk read back: %+v", talk)
	}
	n := 0
	for _, pg := range nb.pages {
		for _, s := range pg {
			if s.pic != nil && isDiary(s) && attr(s.attrs, texAttr) != "" {
				n++
				if !s.pic.ink || s.pic.gray.Pix[0] != 0xff {
					t.Errorf("a formula read back: ink %v, its corner %d", s.pic.ink, s.pic.gray.Pix[0])
				}
			}
		}
	}
	if n != 2 {
		t.Errorf("%d formulas read back", n)
	}
}

// TestMDSpaces: a formula after a word has a space before it, and the
// word after it one too.
func TestMDSpaces(t *testing.T) {
	l := &mdLay{fam: famGentium, pt: 15}
	lines := l.wrap(l.atoms([]md.Span{{Text: "a "}, {Text: "x", Math: true}, {Text: " b"}}, 0, 15), 1000)
	if len(lines) != 1 || len(lines[0]) != 3 {
		t.Fatalf("%+v", lines)
	}
	ln := lines[0]
	if ln[1].x <= ln[0].a.w || ln[2].x <= ln[1].x+ln[1].a.w {
		t.Errorf("no spaces: %d %d+%d %d", ln[0].a.w, ln[1].x, ln[1].a.w, ln[2].x)
	}
}

func TestShareOut(t *testing.T) {
	if w := shareOut([]int{100, 200}, 500, 10); w[0] != 100 || w[1] != 200 {
		t.Errorf("fits: %v", w)
	}
	if w := shareOut([]int{50, 900, 900}, 650, 10); w[0] != 50 || w[1] != 300 || w[2] != 300 {
		t.Errorf("shared: %v", w)
	}
}
