package tex

// The atoms' classes, as TeX has them: they set the spaces between.
const (
	ord = iota
	op
	bin
	rel
	open
	clo
	punct
	inner
)

// spacing is the space between a left atom's class and a right one's:
// 1 thin, 2 medium, 3 thick, 4 thin; 2-4 only in display and text style
// (TeXbook, chapter 18's table).
var spacing = [8][8]int8{
	ord:   {0, 1, 2, 3, 0, 0, 0, 4},
	op:    {1, 1, 0, 3, 0, 0, 0, 4},
	bin:   {2, 2, 0, 0, 2, 0, 0, 2},
	rel:   {3, 3, 0, 0, 3, 0, 0, 3},
	open:  {0, 0, 0, 0, 0, 0, 0, 0},
	clo:   {0, 1, 2, 3, 0, 0, 0, 4},
	punct: {4, 4, 0, 4, 4, 4, 4, 4},
	inner: {4, 1, 2, 3, 4, 0, 4, 4},
}

type sym struct {
	r     rune
	class int
}

// greek are the Greek letters: letters, italic as Latin ones are (the
// capitals upright, as TeX has them).
var greek = map[string]rune{
	"alpha": 'α', "beta": 'β', "gamma": 'γ', "delta": 'δ', "epsilon": 'ϵ', "varepsilon": 'ε',
	"zeta": 'ζ', "eta": 'η', "theta": 'θ', "vartheta": 'ϑ', "iota": 'ι', "kappa": 'κ', "varkappa": 'ϰ',
	"lambda": 'λ', "mu": 'μ', "nu": 'ν', "xi": 'ξ', "omicron": 'ο', "pi": 'π', "varpi": 'ϖ',
	"rho": 'ρ', "varrho": 'ϱ', "sigma": 'σ', "varsigma": 'ς', "tau": 'τ', "upsilon": 'υ',
	"phi": 'ϕ', "varphi": 'φ', "chi": 'χ', "psi": 'ψ', "omega": 'ω', "digamma": 'ϝ',
	"Gamma": 'Γ', "Delta": 'Δ', "Theta": 'Θ', "Lambda": 'Λ', "Xi": 'Ξ', "Pi": 'Π', "Sigma": 'Σ',
	"Upsilon": 'Υ', "Phi": 'Φ', "Psi": 'Ψ', "Omega": 'Ω',
	"Alpha": 'Α', "Beta": 'Β', "Epsilon": 'Ε', "Zeta": 'Ζ', "Eta": 'Η', "Iota": 'Ι', "Kappa": 'Κ',
	"Mu": 'Μ', "Nu": 'Ν', "Omicron": 'Ο', "Rho": 'Ρ', "Tau": 'Τ', "Chi": 'Χ',
}

// symbols are the commands that are a sign.
var symbols = map[string]sym{
	// binary operations
	"pm": {'±', bin}, "mp": {'∓', bin}, "times": {'×', bin}, "div": {'÷', bin}, "cdot": {'⋅', bin},
	"ast": {'∗', bin}, "star": {'⋆', bin}, "circ": {'∘', bin}, "bullet": {'∙', bin},
	"oplus": {'⊕', bin}, "ominus": {'⊖', bin}, "otimes": {'⊗', bin}, "odot": {'⊙', bin}, "oslash": {'⊘', bin},
	"cup": {'∪', bin}, "cap": {'∩', bin}, "sqcup": {'⊔', bin}, "sqcap": {'⊓', bin}, "uplus": {'⊎', bin},
	"wedge": {'∧', bin}, "land": {'∧', bin}, "vee": {'∨', bin}, "lor": {'∨', bin},
	"setminus": {'∖', bin}, "smallsetminus": {'∖', bin}, "wr": {'≀', bin}, "dagger": {'†', bin}, "ddagger": {'‡', bin},
	"amalg": {'⨿', bin}, "triangleleft": {'◁', bin}, "triangleright": {'▷', bin}, "diamond": {'⋄', bin},
	"bigtriangleup": {'△', bin}, "bigtriangledown": {'▽', bin}, "divideontimes": {'⋇', bin},
	// relations
	"leq": {'≤', rel}, "le": {'≤', rel}, "geq": {'≥', rel}, "ge": {'≥', rel}, "neq": {'≠', rel}, "ne": {'≠', rel},
	"equiv": {'≡', rel}, "approx": {'≈', rel}, "approxeq": {'≊', rel}, "sim": {'∼', rel}, "simeq": {'≃', rel},
	"cong": {'≅', rel}, "propto": {'∝', rel}, "asymp": {'≍', rel}, "doteq": {'≐', rel},
	"ll": {'≪', rel}, "gg": {'≫', rel}, "lll": {'⋘', rel}, "ggg": {'⋙', rel},
	"leqslant": {'⩽', rel}, "geqslant": {'⩾', rel}, "lesssim": {'≲', rel}, "gtrsim": {'≳', rel},
	"prec": {'≺', rel}, "succ": {'≻', rel}, "preceq": {'⪯', rel}, "succeq": {'⪰', rel},
	"subset": {'⊂', rel}, "supset": {'⊃', rel}, "subseteq": {'⊆', rel}, "supseteq": {'⊇', rel},
	"subsetneq": {'⊊', rel}, "supsetneq": {'⊋', rel}, "nsubseteq": {'⊈', rel}, "nsupseteq": {'⊉', rel},
	"sqsubset": {'⊏', rel}, "sqsupset": {'⊐', rel}, "sqsubseteq": {'⊑', rel}, "sqsupseteq": {'⊒', rel},
	"in": {'∈', rel}, "notin": {'∉', rel}, "ni": {'∋', rel}, "owns": {'∋', rel},
	"perp": {'⊥', rel}, "parallel": {'∥', rel}, "nparallel": {'∦', rel}, "mid": {'∣', rel}, "nmid": {'∤', rel},
	"models": {'⊨', rel}, "vDash": {'⊨', rel}, "Vdash": {'⊩', rel}, "vdash": {'⊢', rel}, "dashv": {'⊣', rel},
	"smile": {'⌣', rel}, "frown": {'⌢', rel}, "bowtie": {'⋈', rel},
	"to": {'→', rel}, "rightarrow": {'→', rel}, "leftarrow": {'←', rel}, "gets": {'←', rel},
	"Rightarrow": {'⇒', rel}, "Leftarrow": {'⇐', rel}, "leftrightarrow": {'↔', rel}, "Leftrightarrow": {'⇔', rel},
	"longrightarrow": {'⟶', rel}, "longleftarrow": {'⟵', rel}, "Longrightarrow": {'⟹', rel}, "Longleftarrow": {'⟸', rel},
	"longleftrightarrow": {'⟷', rel}, "Longleftrightarrow": {'⟺', rel}, "iff": {'⟺', rel}, "implies": {'⟹', rel},
	"impliedby": {'⟸', rel}, "mapsto": {'↦', rel}, "longmapsto": {'⟼', rel},
	"hookrightarrow": {'↪', rel}, "hookleftarrow": {'↩', rel},
	"uparrow": {'↑', rel}, "downarrow": {'↓', rel}, "updownarrow": {'↕', rel}, "Uparrow": {'⇑', rel},
	"Downarrow": {'⇓', rel}, "Updownarrow": {'⇕', rel}, "nearrow": {'↗', rel}, "searrow": {'↘', rel},
	"swarrow": {'↙', rel}, "nwarrow": {'↖', rel}, "rightleftharpoons": {'⇌', rel}, "leftrightarrows": {'⇆', rel},
	"rightleftarrows": {'⇄', rel}, "rightharpoonup": {'⇀', rel}, "leftharpoonup": {'↼', rel}, "leadsto": {'⇝', rel},
	"coloneqq": {'≔', rel}, "eqqcolon": {'≕', rel}, "triangleq": {'≜', rel}, "ncong": {'≇', rel},
	"nsim": {'≁', rel}, "nleq": {'≰', rel}, "ngeq": {'≱', rel}, "nless": {'≮', rel}, "ngtr": {'≯', rel},
	"because": {'∵', rel}, "therefore": {'∴', rel}, "colon": {':', punct},
	// the rest
	"infty": {'∞', ord}, "partial": {'∂', ord}, "nabla": {'∇', ord}, "forall": {'∀', ord}, "exists": {'∃', ord},
	"nexists": {'∄', ord}, "emptyset": {'∅', ord}, "varnothing": {'∅', ord}, "neg": {'¬', ord}, "lnot": {'¬', ord},
	"angle": {'∠', ord}, "measuredangle": {'∡', ord}, "triangle": {'△', ord}, "square": {'□', ord},
	"Box": {'□', ord}, "blacksquare": {'■', ord}, "Diamond": {'◇', ord}, "lozenge": {'◊', ord},
	"hbar": {'ℏ', ord}, "hslash": {'ℏ', ord}, "ell": {'ℓ', ord}, "wp": {'℘', ord}, "Re": {'ℜ', ord}, "Im": {'ℑ', ord},
	"aleph": {'ℵ', ord}, "beth": {'ℶ', ord}, "prime": {'′', ord}, "top": {'⊤', ord}, "bot": {'⊥', ord},
	"flat": {'♭', ord}, "natural": {'♮', ord}, "sharp": {'♯', ord}, "clubsuit": {'♣', ord},
	"diamondsuit": {'♢', ord}, "heartsuit": {'♡', ord}, "spadesuit": {'♠', ord}, "surd": {'√', ord},
	"degree": {'°', ord}, "checkmark": {'✓', ord}, "dag": {'†', ord}, "ddag": {'‡', ord}, "S": {'§', ord},
	"P": {'¶', ord}, "copyright": {'©', ord}, "pounds": {'£', ord}, "euro": {'€', ord}, "imath": {'ı', ord},
	"jmath": {'ȷ', ord}, "mho": {'℧', ord}, "complement": {'∁', ord}, "backslash": {'\\', ord},
	"vert": {'|', ord}, "Vert": {'‖', ord}, "|": {'‖', ord}, "bigstar": {'★', ord}, "circledR": {'®', ord},
	"dots": {'…', inner}, "ldots": {'…', inner}, "cdots": {'⋯', inner}, "dotsc": {'…', inner}, "dotsb": {'⋯', inner},
	"dotsm": {'⋯', inner}, "dotsi": {'⋯', inner}, "dotso": {'…', inner}, "vdots": {'⋮', ord}, "ddots": {'⋱', inner},
	"ldotp": {'.', punct}, "cdotp": {'⋅', punct}, "lbrace": {'{', open}, "rbrace": {'}', clo}, "{": {'{', open},
	"}": {'}', clo}, "lbrack": {'[', open}, "rbrack": {']', clo}, "langle": {'⟨', open}, "rangle": {'⟩', clo},
	"lfloor": {'⌊', open}, "rfloor": {'⌋', clo}, "lceil": {'⌈', open}, "rceil": {'⌉', clo},
	"lvert": {'|', open}, "rvert": {'|', clo}, "lVert": {'‖', open}, "rVert": {'‖', clo},
	"lgroup": {'⟮', open}, "rgroup": {'⟯', clo},
	"%": {'%', ord}, "$": {'$', ord}, "&": {'&', ord}, "#": {'#', ord}, "_": {'_', ord},
}

// nots are \not and a relation: their own signs, where there's one.
var nots = map[rune]rune{'=': '≠', '∈': '∉', '≡': '≢', '⊂': '⊄', '⊃': '⊅', '<': '≮', '>': '≯', '≤': '≰',
	'≥': '≱', '∼': '≁', '≈': '≉', '⊆': '⊈', '⊇': '⊉', '∃': '∄', '∣': '∤', '∥': '∦', '≅': '≇', '≃': '≄'}

// bigOps are the large operators: their limits over and under in
// display style, but the integrals' at their side.
var bigOps = map[string]rune{
	"sum": '∑', "prod": '∏', "coprod": '∐', "bigcup": '⋃', "bigcap": '⋂', "bigsqcup": '⨆', "bigvee": '⋁',
	"bigwedge": '⋀', "bigoplus": '⨁', "bigotimes": '⨂', "bigodot": '⨀', "biguplus": '⨄',
	"int": '∫', "iint": '∬', "iiint": '∭', "oint": '∮', "oiint": '∯', "intop": '∫', "smallint": '∫',
}

var integrals = map[rune]bool{'∫': true, '∬': true, '∭': true, '∮': true, '∯': true}

// funcs are the functions written upright; true: limits over and under
// in display style.
var funcs = map[string]bool{
	"arccos": false, "arcsin": false, "arctan": false, "arg": false, "cos": false, "cosh": false,
	"cot": false, "coth": false, "csc": false, "deg": false, "dim": false, "exp": false, "hom": false,
	"ker": false, "lg": false, "ln": false, "log": false, "sec": false, "sin": false, "sinh": false,
	"tan": false, "tanh": false, "sgn": false, "tr": false, "rank": false, "Tr": false,
	"det": true, "gcd": true, "inf": true, "lim": true, "max": true, "min": true, "Pr": true, "sup": true,
	"liminf": true, "limsup": true, "argmax": true, "argmin": true, "lcm": true,
}

// funcNames are the functions written otherwise than as named.
var funcNames = map[string]string{"liminf": "lim inf", "limsup": "lim sup", "argmax": "arg max", "argmin": "arg min"}

// accents are the accents drawn as a sign over what they're on.
var accents = map[string]rune{
	"hat": 'ˆ', "check": 'ˇ', "tilde": '˜', "acute": '´', "grave": '`', "dot": '˙', "ddot": '¨',
	"breve": '˘', "mathring": '˚', "bar": '¯',
}

// unicodeClass is the class of a sign written as it is ("≤", not
// "\le"): as its command's.
var unicodeClass = func() map[rune]int {
	m := map[rune]int{'+': bin, '−': bin, '-': bin, '*': bin, '=': rel, '<': rel, '>': rel, ':': rel,
		',': punct, ';': punct, '(': open, '[': open, ')': clo, ']': clo, '!': clo, '?': clo}
	for _, s := range symbols {
		if _, ok := m[s.r]; !ok && s.r > 127 {
			m[s.r] = s.class
		}
	}
	return m
}()

// letter maps a letter to the font's: italic (as math is), upright,
// bold..., as it is where the font hasn't it.
func letter(r rune, font string) rune {
	isUp, isLow := r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z'
	digit := r >= '0' && r <= '9'
	switch font {
	case "it":
		switch {
		case r == 'h':
			return 'ℎ'
		case isUp:
			return 0x1D434 + r - 'A'
		case isLow:
			return 0x1D44E + r - 'a'
		case r >= 'α' && r <= 'ω':
			return 0x1D6FC + r - 'α'
		}
		switch r {
		case 'ϵ':
			return 0x1D716
		case 'ϑ':
			return 0x1D717
		case 'ϰ':
			return 0x1D718
		case 'ϕ':
			return 0x1D719
		case 'ϱ':
			return 0x1D71A
		case 'ϖ':
			return 0x1D71B
		case 'ı':
			return 0x1D6A4
		case 'ȷ':
			return 0x1D6A5
		case '∂':
			return 0x1D715
		}
	case "bf":
		switch {
		case isUp:
			return 0x1D400 + r - 'A'
		case isLow:
			return 0x1D41A + r - 'a'
		case digit:
			return 0x1D7CE + r - '0'
		}
	case "bfit":
		switch {
		case isUp:
			return 0x1D468 + r - 'A'
		case isLow:
			return 0x1D482 + r - 'a'
		case digit:
			return 0x1D7CE + r - '0'
		case r >= 'α' && r <= 'ω':
			return 0x1D736 + r - 'α'
		}
	case "bb":
		if x, ok := map[rune]rune{'C': 'ℂ', 'H': 'ℍ', 'N': 'ℕ', 'P': 'ℙ', 'Q': 'ℚ', 'R': 'ℝ', 'Z': 'ℤ'}[r]; ok {
			return x
		}
		switch {
		case isUp:
			return 0x1D538 + r - 'A'
		case isLow:
			return 0x1D552 + r - 'a'
		case digit:
			return 0x1D7D8 + r - '0'
		}
	case "cal":
		if x, ok := map[rune]rune{'B': 'ℬ', 'E': 'ℰ', 'F': 'ℱ', 'H': 'ℋ', 'I': 'ℐ', 'L': 'ℒ', 'M': 'ℳ', 'R': 'ℛ',
			'e': 'ℯ', 'g': 'ℊ', 'o': 'ℴ'}[r]; ok {
			return x
		}
		switch {
		case isUp:
			return 0x1D49C + r - 'A'
		case isLow:
			return 0x1D4B6 + r - 'a'
		}
	case "frak":
		if x, ok := map[rune]rune{'C': 'ℭ', 'H': 'ℌ', 'I': 'ℑ', 'R': 'ℜ', 'Z': 'ℨ'}[r]; ok {
			return x
		}
		switch {
		case isUp:
			return 0x1D504 + r - 'A'
		case isLow:
			return 0x1D51E + r - 'a'
		}
	case "sf":
		switch {
		case isUp:
			return 0x1D5A0 + r - 'A'
		case isLow:
			return 0x1D5BA + r - 'a'
		case digit:
			return 0x1D7E2 + r - '0'
		}
	case "tt":
		switch {
		case isUp:
			return 0x1D670 + r - 'A'
		case isLow:
			return 0x1D68A + r - 'a'
		case digit:
			return 0x1D7F6 + r - '0'
		}
	}
	return r
}
