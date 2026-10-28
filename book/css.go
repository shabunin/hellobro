package book

// A little of CSS: rules whose selector is a tag, a class or a tag with
// classes (no descendants, ids or pseudo-classes), and of their
// properties those that change how a reader lays text out.

import (
	"strconv"
	"strings"
)

type decl struct{ prop, val string }

// sheet is a stylesheet's rules, by selector: "p", ".c", "p.c".
type sheet struct{ rules map[string][]decl }

func parseCSS(src string) *sheet {
	s := &sheet{rules: map[string][]decl{}}
	for {
		i := strings.Index(src, "/*")
		if i < 0 {
			break
		}
		j := strings.Index(src[i+2:], "*/")
		if j < 0 {
			src = src[:i]
			break
		}
		src = src[:i] + " " + src[i+2+j+2:]
	}
	for {
		src = strings.TrimSpace(src)
		if src == "" {
			return s
		}
		open := strings.IndexByte(src, '{')
		if open < 0 {
			return s
		}
		sel := strings.TrimSpace(src[:open])
		if semi := strings.IndexByte(sel, ';'); semi >= 0 && strings.HasPrefix(sel, "@") {
			src = src[semi+1:] // @import, @charset
			continue
		}
		// the block, to its closing brace (an @media's has rules in it)
		depth, end := 0, -1
		for k := open; k < len(src); k++ {
			if src[k] == '{' {
				depth++
			} else if src[k] == '}' {
				if depth--; depth == 0 {
					end = k
					break
				}
			}
		}
		if end < 0 {
			return s
		}
		body := src[open+1 : end]
		src = src[end+1:]
		if strings.HasPrefix(sel, "@") {
			continue
		}
		ds := parseDecls(body)
		for _, one := range strings.Split(sel, ",") {
			if k, ok := simpleSelector(strings.TrimSpace(one)); ok {
				s.rules[k] = append(s.rules[k], ds...)
			}
		}
	}
}

// simpleSelector is sel as a key, if it's one we follow: "p", ".c",
// "p.c", "p.a.b" (classes sorted as they come).
func simpleSelector(sel string) (string, bool) {
	if sel == "" || strings.ContainsAny(sel, " >+~:[#*") {
		return "", false
	}
	return strings.ToLower(sel), true
}

func parseDecls(body string) []decl {
	var ds []decl
	for _, d := range strings.Split(body, ";") {
		p, v, ok := strings.Cut(d, ":")
		if !ok {
			continue
		}
		v = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(v), "!important"))
		ds = append(ds, decl{strings.ToLower(strings.TrimSpace(p)), strings.ToLower(v)})
	}
	return ds
}

// matching are the declarations for an element tag with classes, in
// the order they apply: the tag's, each class's, the tag with them.
func (s *sheet) matching(tag string, classes []string, out []decl) []decl {
	out = append(out, s.rules[tag]...)
	for _, c := range classes {
		out = append(out, s.rules["."+c]...)
	}
	for _, c := range classes {
		out = append(out, s.rules[tag+"."+c]...)
	}
	if len(classes) > 1 {
		out = append(out, s.rules["."+strings.Join(classes, ".")]...)
		out = append(out, s.rules[tag+"."+strings.Join(classes, ".")]...)
	}
	return out
}

// length is v in ems; ok false if it's not a length.
func length(v string) (float32, bool) {
	v = strings.TrimSpace(v)
	unit := strings.TrimLeft(v, "-+.0123456789")
	n, err := strconv.ParseFloat(strings.TrimSuffix(v, unit), 32)
	if err != nil {
		return 0, v == "auto" || v == "0"
	}
	f := float32(n)
	switch unit {
	case "em", "rem":
		return f, true
	case "ex", "ch":
		return f / 2, true
	case "px":
		return f / 16, true
	case "pt":
		return f / 12, true
	case "%":
		return f / 100 * 30, true // of a line about 30 ems wide
	case "": // only 0 is right, but px is what's meant
		return f / 16, true
	}
	return 0, false
}

// fontScale is font-size's v as a factor of the parent's.
func fontScale(v string) (float32, bool) {
	switch v {
	case "xx-small":
		return 0.6, true
	case "x-small":
		return 0.75, true
	case "small", "smaller":
		return 0.85, true
	case "medium", "inherit", "initial":
		return 1, true
	case "large", "larger":
		return 1.15, true
	case "x-large":
		return 1.4, true
	case "xx-large":
		return 1.7, true
	}
	if strings.HasSuffix(v, "%") {
		n, err := strconv.ParseFloat(strings.TrimSuffix(v, "%"), 32)
		return float32(n) / 100, err == nil && n > 0
	}
	if f, ok := length(v); ok && f > 0 {
		return f, true
	}
	return 1, false
}
