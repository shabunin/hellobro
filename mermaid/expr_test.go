package mermaid

import (
	"math"
	"testing"
)

func TestFormula(t *testing.T) {
	for _, tc := range []struct {
		s    string
		x, y float64
	}{
		{"2x+1", 3, 7},
		{"x^2", -3, 9},
		{"-x^2", 3, -9},
		{"2^-1", 0, 0.5},
		{"2^3^2", 0, 512},
		{"(x+1)(x-1)", 3, 8},
		{"3(x+1)", 1, 6},
		{"sin(pi/2)", 0, 1},
		{"sin x", math.Pi / 2, 1},
		{"sin 2x", math.Pi / 4, 1},
		{"sin(x)^2 + cos(x)^2", 0.7, 1},
		{"Math.sqrt(x) * 2", 4, 4},
		{"x²−1", 3, 8},
		{"exp(-x/5)*cos(x)", 0, 1},
		{"ln(e)", 0, 1},
		{"log10(1000)", 0, 3},
		{"abs(x) + max(x, 2)", -3, 5},
		{"2πx", 0.5, math.Pi},
		{"1e-3*x", 1000, 1},
		{"x/2/2", 8, 2},
		{"2*-x", 3, -6},
		{"xsin(x)", math.Pi / 2, math.Pi / 2},
		{"1/(1+e^(-x))", 0, 0.5},
	} {
		f, err := parseFormula(tc.s)
		if err != nil {
			t.Errorf("%s: %v", tc.s, err)
			continue
		}
		if y := f(tc.x); math.Abs(y-tc.y) > 1e-9 {
			t.Errorf("%s at %v: %v, want %v", tc.s, tc.x, y, tc.y)
		}
	}
	for _, s := range []string{"", "2+", "foo(x)", "(x", "x)", "sin()", "1,2", "y"} {
		if _, err := parseFormula(s); err == nil {
			t.Errorf("%q read", s)
		}
	}
}
