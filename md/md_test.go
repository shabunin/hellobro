package md

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	src := "# Title\n\nSome **bold** text,\nnext line.\n\n- one\n- two\n  - deeper\n    goes on\n1. first\n2) second\n\n```go\nfunc f() {\n\treturn\n}\n```\n\n$$\n\\frac{a}{b}\n$$\n\n> quoted\n> more\n\n| a | b |\n|:--|--:|\n| 1 | `x|y` |\n\n---\n\\begin{align}\nx &= 1\n\\end{align}\nafter $$y$$ here\n\nSub\n---\n- [ ] todo"
	got := Parse(src)
	want := []Block{
		{Kind: Heading, Level: 1, Text: "Title"},
		{Kind: Para, Text: "Some **bold** text,\nnext line."},
		{Kind: Item, Marker: "•", Text: "one"},
		{Kind: Item, Marker: "•", Text: "two"},
		{Kind: Item, Level: 1, Marker: "◦", Text: "deeper\ngoes on"},
		{Kind: Item, Marker: "1.", Text: "first"},
		{Kind: Item, Marker: "2.", Text: "second"},
		{Kind: Code, Lang: "go", Text: "func f() {\n    return\n}"},
		{Kind: Math, Text: `\frac{a}{b}`},
		{Kind: Para, Quote: 1, Text: "quoted\nmore"},
		{Kind: Table, Rows: [][]string{{"a", "b"}, {"1", "`x|y`"}}, Align: []byte("lr")},
		{Kind: Rule},
		{Kind: Math, Text: "\\begin{align}\nx &= 1\n\\end{align}"},
		{Kind: Para, Text: "after $$y$$ here"},
		{Kind: Heading, Level: 2, Text: "Sub"},
		{Kind: Item, Marker: "☐", Text: "todo"},
	}
	if len(got) != len(want) {
		t.Fatalf("%d blocks: %+v", len(got), got)
	}
	for i := range want {
		if !reflect.DeepEqual(got[i], want[i]) {
			t.Errorf("%d: %+v\nwant %+v", i, got[i], want[i])
		}
	}
	if b := Parse("text\n$$x^2$$ and on"); len(b) != 3 || b[1].Kind != Math || b[1].Text != "x^2" || b[2].Text != "and on" {
		t.Errorf("a formula on its line: %+v", b)
	}
}

func TestInline(t *testing.T) {
	for in, want := range map[string][]Span{
		"a **b** *c* ***d***":                  {{Text: "a "}, {Text: "b", Style: Bold}, {Text: " "}, {Text: "c", Style: Italic}, {Text: " "}, {Text: "d", Style: Bold | Italic}},
		"**bold *both* bold**":                 {{Text: "bold ", Style: Bold}, {Text: "both", Style: Bold | Italic}, {Text: " bold", Style: Bold}},
		"x `a*b` $x^2$ \\(y\\)":                {{Text: "x "}, {Text: "a*b", Style: Mono}, {Text: " "}, {Text: "x^2", Math: true}, {Text: " "}, {Text: "y", Math: true}},
		"costs $5 and $6":                      {{Text: "costs $5 and $6"}},
		"2 * 3 * 4, snake_case_name":           {{Text: "2 * 3 * 4, snake_case_name"}},
		"[a link](http://x) and ![pic](p.png)": {{Text: "a link"}, {Text: " and "}, {Text: "pic"}},
		"one<br>two\nthree":                    {{Text: "one"}, {Break: true}, {Text: "two"}, {Break: true}, {Text: "three"}},
		"\\*not\\* ~~gone~~":                   {{Text: "*not* "}, {Text: "gone", Style: Strike}},
		"see $$\\sum x$$ there":                {{Text: "see "}, {Text: `\sum x`, Math: true, Display: true}, {Text: " there"}},
		"**$x$ is** _it_":                      {{Text: "x", Math: true}, {Text: " is", Style: Bold}, {Text: " "}, {Text: "it", Style: Italic}},
	} {
		if got := Inline(in); !reflect.DeepEqual(got, want) {
			t.Errorf("%q:\n got %+v\nwant %+v", in, got, want)
		}
	}
}
