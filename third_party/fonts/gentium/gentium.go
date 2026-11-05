// Package gentium is Gentium Book's regular, unchanged (its license is in
// OFL.txt): what the reader falls back on for the letters its fonts lack.
// The Kobo's have no polytonic Greek.
package gentium

import _ "embed"

//go:embed GentiumBook-Regular.ttf
var TTF []byte
