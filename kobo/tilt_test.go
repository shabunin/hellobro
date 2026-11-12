package kobo

import "testing"

func TestTiltOf(t *testing.T) {
	for _, c := range []struct {
		v    byte
		want int
	}{
		{1 << 2, 2}, {1 << 4, 3}, {1 << 3, 0}, {1 << 5, 1}, // the Elipsa's board is upside down
		{1 << 0, -1}, {1 << 1, -1}, {0, -1}, // face up, face down, nothing
	} {
		if got := tiltOf(c.v); got != c.want {
			t.Errorf("tiltOf(%#x) = %d, want %d", c.v, got, c.want)
		}
	}
}
