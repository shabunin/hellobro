// Package stix is STIX Two Math's regular, unchanged (its license is in
// OFL.txt): the formulas' font, every sign of them in one.
package stix

import _ "embed"

//go:embed STIXTwoMath-Regular.otf
var OTF []byte
