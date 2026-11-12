package mermaid

// Pies: slices in grays, from the top clockwise, and a legend with each
// one's share.

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

type pieChart struct {
	title  string
	show   bool // showData: the values in the legend too
	slices []pieSlice
}

type pieSlice struct {
	name string
	v    float64
}

var pieRE = regexp.MustCompile(`^"([^"]*)"\s*:\s*([0-9.eE+-]+)`)

func parsePie(lines []string) *pieChart {
	pc := &pieChart{}
	head := strings.Fields(lines[0])
	for i := 1; i < len(head); i++ {
		switch head[i] {
		case "showData":
			pc.show = true
		case "title":
			pc.title = strings.Join(head[i+1:], " ")
			i = len(head)
		}
	}
	for _, l := range lines[1:] {
		if strings.HasPrefix(l, "title ") {
			pc.title = strings.TrimSpace(l[6:])
			continue
		}
		if m := pieRE.FindStringSubmatch(l); m != nil {
			if v, err := strconv.ParseFloat(m[2], 64); err == nil && v > 0 && !math.IsInf(v, 0) {
				pc.slices = append(pc.slices, pieSlice{m[1], v})
			}
		}
	}
	return pc
}

// pieGrays are the slices' grays, in turn: far apart, for e-ink.
var pieGrays = []uint8{0x30, 0xa8, 0x68, 0xd8, 0x50, 0xc0, 0x88, 0xec}

func (pc *pieChart) draw(d *drawer) *picture {
	em := d.em
	lw := max(1.2, em*0.07)
	p := &picture{}
	R := em * 5.5
	total := 0.0
	for _, s := range pc.slices {
		total += s.v
	}
	a := -math.Pi / 2
	var cuts []float64
	for i, s := range pc.slices {
		da := 2 * math.Pi * s.v / total
		n := max(2, int(da/(2*math.Pi)*96))
		ps := append([]pt{{0, 0}}, arc(pt{}, R, R, a, a+da, n)...)
		p.fill(ps, pieGrays[i%len(pieGrays)])
		cuts = append(cuts, a)
		a += da
	}
	if len(pc.slices) > 1 {
		for _, c := range cuts {
			p.stroke([]pt{{0, 0}, {R * math.Cos(c), R * math.Sin(c)}}, lw*1.5, 0, 0xff)
		}
	}
	p.stroke(closed(circle(pt{}, R, 96)), lw*1.2, 0, 0)
	y := -R + em*0.2
	x := R + em*1.5
	for i, s := range pc.slices {
		pct := 100 * s.v / total
		t := fmt.Sprintf("%.0f%%", pct)
		if pct < 10 {
			t = fmt.Sprintf("%.1f%%", pct)
		}
		name := s.name
		if pc.show {
			name += fmt.Sprintf(" [%s]", strconv.FormatFloat(s.v, 'f', -1, 64))
		}
		lab := d.label(name+" — "+t, em*14)
		sq := em * 0.9
		r := []pt{{x, y + (lab.h-sq)/2}, {x + sq, y + (lab.h-sq)/2}, {x + sq, y + (lab.h+sq)/2}, {x, y + (lab.h+sq)/2}}
		p.fill(r, pieGrays[i%len(pieGrays)])
		p.stroke(closed(r), lw, 0, 0)
		p.label(lab, pt{x + sq + em*0.4 + lab.w/2, y + lab.h/2})
		y += lab.h + em*0.4
	}
	if pc.title != "" {
		t := d.label("**"+pc.title+"**", em*24)
		p.label(t, pt{0, -R - em*0.6 - t.h/2})
	}
	return p
}
