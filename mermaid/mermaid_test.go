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
	{"class", `classDiagram
    direction TB
    class Animal {
        <<abstract>>
        +String name
        +int age
        +makeSound()* void
        +isMammal() bool
    }
    class Duck {
        +String beakColor
        +swim()
        +quack()
    }
    class List~T~
    class Fish
    Animal <|-- Duck
    Animal <|-- Fish : is a
    Animal "1" *-- "many" Leg : has
    Zoo o-- Animal
    Duck ..> List~T~ : uses
    Fish ..|> Swimmer
    <<interface>> Swimmer
    Leg : +int length`},
	{"plot", `plot
    title Damped wave
    x: -2pi .. 4pi
    y = sin(x)/x : sinc
    y = exp(-x/5) cos x : "damped"
    y = tan(x) / 4
    x = 3.5 : a line
    points: (0, 1) (pi, 0) : marks
    xlabel: t, s`},
	{"plotauto", "plot\nf(x) = x^2 - 3\ng(x) = 2x + 1"},
	{"xy", `xychart-beta
    title "Sales Revenue"
    x-axis [jan, feb, mar, apr, may, jun, jul, aug]
    y-axis "Revenue (in $)" 4000 --> 11000
    bar [5000, 6000, 7500, 8200, 9500, 10500, 11000, 10200]
    line [5000, 6000, 7500, 8200, 9500, 10500, 11000, 10200]`},
	{"gantt", `gantt
    title A Gantt Diagram
    dateFormat YYYY-MM-DD
    section Section
    A task           :a1, 2014-01-01, 30d
    Another task     :after a1, 20d
    section Another
    Task in sec      :2014-01-12, 12d
    another task     :24d
    Done thing       :done, des1, 2014-01-06, 2014-01-08
    Active one       :active, des2, 2014-01-09, 3d
    Critical         :crit, 2014-02-01, 5d
    Release          :milestone, m1, 2014-02-20, 0d`},
	{"timeline", `timeline
    title History of Social Media Platform
    section The beginning
    2002 : LinkedIn
    2004 : Facebook : Google
    section Video
    2005 : YouTube
         : and more
    2006 : Twitter`},
	{"mindmap", "mindmap\n  root((mindmap))\n    Origins\n      Long history\n      ::icon(fa fa-book)\n      Popularisation\n        British popular psychology author Tony Buzan\n    Research\n      On effectiveness<br/>and features\n      On Automatic creation\n        Uses\n            Creative techniques\n            Strategic planning\n            Argument mapping\n    Tools\n      Pen and paper\n      Mermaid\n    id)Cloud(\n    id2{{Hexagon}}"},
	{"er", `erDiagram
    CUSTOMER ||--o{ ORDER : places
    ORDER ||--|{ LINE-ITEM : contains
    CUSTOMER }|..|{ DELIVERY-ADDRESS : uses
    PRODUCT |o--o{ LINE-ITEM : "ordered in"
    CUSTOMER {
        string name
        string custNumber PK "the number"
        string email UK
    }
    ORDER {
        int orderNumber PK
        string customer FK
    }`},
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
	for _, src := range []string{"", "quadrantChart\n title x", "journey\n title x", "graph TD\n", "classDiagram\n", "erDiagram\n"} {
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
