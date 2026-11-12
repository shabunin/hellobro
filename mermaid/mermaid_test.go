package mermaid

import (
	"fmt"
	"image/png"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var samples = []struct{ name, src string }{
	{"flow", `graph TD
    A[Start] --> B{Is it working?}
    B -->|Yes| C[Great!]
    B -->|No| D[Debug]
    D --> E((Fix))
    E --> B
    C --> F([Done])`},
	{"flowlr", `flowchart LR
    client[(Client DB)] -- request --> api[[API server]]
    api -.-> cache{{Cache}}
    api ==> db[(Postgres)]
    cache --o api
    db --x log>Log]
    api --> q[/Queue/] & w[\Worker\]`},
	{"subgraph", `flowchart TB
    c1-->a2
    subgraph one
    a1-->a2
    end
    subgraph two [Second one]
    b1-->b2
    end
    subgraph three
    c1-->c2
    end
    one --> two
    three --> two
    two --> c2`},
	{"math", "graph TD\n  A[\"$$x^2 + y^2 = r^2$$\"] --> B[\"**Площадь**: $$\\pi r^2$$\"]\n  B --> C[\"Ελληνικά<br>second line\"]"},
	{"state", `stateDiagram-v2
    [*] --> Still
    Still --> [*]
    Still --> Moving : push
    Moving --> Still : stop
    Moving --> Crash
    Crash --> [*]
    state Moving {
        [*] --> Slow
        Slow --> Fast : faster
        Fast --> Slow
    }
    state choose <<choice>>
    Crash --> choose
    choose --> Still : fixed`},
	{"seq", `sequenceDiagram
    autonumber
    participant A as Alice
    actor B as Bob
    participant C as Server
    A->>B: Hello Bob, how are you?
    loop Every minute
        B-->>C: ping
        C--)B: pong
    end
    alt is sick
        B->>A: Not so good :(
    else is well
        B->>A: Feeling fresh like a daisy
    end
    Note right of C: thinks a while
    Note over A,B: A typical interaction
    A->>A: think
    A-xC: bye`},
	{"pie", `pie title Pets adopted
    "Dogs" : 386
    "Cats" : 85
    "Rats" : 15`},
	{"cycle", "graph LR\nA-->B-->C-->A\nC-->D\nD-->D"},
}

func TestRender(t *testing.T) {
	out := os.Getenv("MERMAID_OUT")
	for _, s := range samples {
		r, err := Render(s.src, Options{Em: 22})
		if err != nil {
			t.Errorf("%s: %v", s.name, err)
			continue
		}
		b := r.Img.Bounds()
		if b.Dx() < 50 || b.Dy() < 50 || b.Dx() >= maxSide || b.Dy() >= maxSide {
			t.Errorf("%s: %v", s.name, b)
		}
		dark := 0
		for _, g := range r.Img.Pix {
			if g < 128 {
				dark++
			}
		}
		if dark < 100 {
			t.Errorf("%s: nearly blank (%d dark)", s.name, dark)
		}
		if out != "" {
			f, _ := os.Create(filepath.Join(out, s.name+".png"))
			png.Encode(f, r.Img)
			f.Close()
		}
	}
}

func TestUnknown(t *testing.T) {
	for _, src := range []string{"", "gantt\n title x", "classDiagram\n A <|-- B", "graph TD\n"} {
		if _, err := Render(src, Options{Em: 20}); err == nil {
			t.Errorf("%q drawn", src)
		}
	}
}

func TestFlowParse(t *testing.T) {
	g := parseFlow(prepare(`graph TD
  A["a (b)"] -- yes --> B(round) -.-> C{q?}
  C ==>|label| D & E
  my-node --> F;F---G
  X -->|"quoted|text"| Y`))
	want := map[string]string{"A": "rect a (b)", "B": "round round", "C": "rhombus q?", "my-node": "rect my-node"}
	for id, w := range want {
		i, ok := g.ids[id]
		if !ok {
			t.Errorf("no %s", id)
			continue
		}
		if got := g.nodes[i].shape + " " + g.nodes[i].text; got != w {
			t.Errorf("%s: %q, want %q", id, got, w)
		}
	}
	var es []string
	for _, e := range g.edges {
		h := e.head
		if h == 0 {
			h = '_'
		}
		es = append(es, fmt.Sprintf("%s%c%c%s:%s", g.nodes[e.from].id, e.line, h, g.nodes[e.to].id, e.text))
	}
	got := strings.Join(es, " ")
	want2 := "A->B:yes B.>C: C=>D:label C=>E:label my-node->F: F-_G: X->Y:quoted|text"
	if got != want2 {
		t.Errorf("edges %s\nwant %s", got, want2)
	}
}

// TestJunk: whatever's written, it doesn't panic or hang.
func TestJunk(t *testing.T) {
	rnd := rand.New(rand.NewSource(1))
	bits := []string{"graph TD\n", "flowchart LR\n", "sequenceDiagram\n", "stateDiagram-v2\n", "pie\n", "A", "B", "-->", "---", "-.->", "==>", "|x|",
		"[", "]", "(", ")", "{", "}", "((", "))", "\"", "subgraph s", "end", "\n", ";", "&", " ", "-x", "->>", ":", "Note over A,B: n",
		"loop", "alt", "else", "[*]", "state X {", "}", "$$x^", "<br>", "**", "\"a\" : 3", "~~~", "o", "x", ">"}
	for range 400 {
		var b strings.Builder
		for range rnd.Intn(40) {
			b.WriteString(bits[rnd.Intn(len(bits))])
		}
		Render(b.String(), Options{Em: 12})
	}
}
