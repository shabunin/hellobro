// Package gentium is Gentium Book, unchanged (its license is in
// OFL.txt): what the reader falls back on for the letters its fonts lack
// (the Kobo's have no polytonic Greek), and a font for notebooks' text,
// with Latin, Greek and Cyrillic whole, in its four styles.
package gentium

import _ "embed"

var (
	//go:embed GentiumBook-Regular.ttf
	TTF []byte
	//go:embed GentiumBook-Bold.ttf
	Bold []byte
	//go:embed GentiumBook-Italic.ttf
	Italic []byte
	//go:embed GentiumBook-BoldItalic.ttf
	BoldItalic []byte
)
