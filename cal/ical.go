// Package cal is calendars: iCalendar's events and tasks (RFC 5545),
// read and written so what we don't know of them is kept as it was, and
// kept in step with a CalDAV server (RFC 4791).
package cal

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Prop is a property: NAME;PARAM=value:VALUE.
type Prop struct {
	Name   string
	Params [][2]string // in order, names upper case
	Value  string      // as written, escapes and all
}

// Param is the value of the parameter name, "" if it has none.
func (p *Prop) Param(name string) string {
	for _, kv := range p.Params {
		if kv[0] == name {
			return kv[1]
		}
	}
	return ""
}

// Comp is a component: VCALENDAR, VEVENT, VTODO, VALARM, VTIMEZONE…
type Comp struct {
	Name  string
	Props []*Prop
	Comps []*Comp
}

// Get is c's first property name, nil if it has none.
func (c *Comp) Get(name string) *Prop {
	for _, p := range c.Props {
		if p.Name == name {
			return p
		}
	}
	return nil
}

// All are c's properties named name.
func (c *Comp) All(name string) []*Prop {
	var ps []*Prop
	for _, p := range c.Props {
		if p.Name == name {
			ps = append(ps, p)
		}
	}
	return ps
}

// Text is property name's value as text, unescaped; "" if none.
func (c *Comp) Text(name string) string {
	if p := c.Get(name); p != nil {
		return unescape(p.Value)
	}
	return ""
}

// Set puts property name as value (already in iCalendar's form), with
// params, in place of what it had; "" takes it away.
func (c *Comp) Set(name, value string, params ...[2]string) {
	if value == "" {
		c.Del(name)
		return
	}
	if p := c.Get(name); p != nil {
		p.Value, p.Params = value, params
		c.Props = append(c.Props[:0], deleteFrom(c.Props, name, p)...)
		return
	}
	c.Props = append(c.Props, &Prop{Name: name, Params: params, Value: value})
}

// deleteFrom is ps less the properties name, but keep.
func deleteFrom(ps []*Prop, name string, keep *Prop) []*Prop {
	out := make([]*Prop, 0, len(ps))
	for _, p := range ps {
		if p.Name != name || p == keep {
			out = append(out, p)
		}
	}
	return out
}

// SetText puts property name as text, escaped.
func (c *Comp) SetText(name, s string) { c.Set(name, escape(s)) }

// Del takes c's properties name away.
func (c *Comp) Del(name string) { c.Props = deleteFrom(c.Props, name, nil) }

// Sub is c's first component name, nil if none.
func (c *Comp) Sub(name string) *Comp {
	for _, s := range c.Comps {
		if s.Name == name {
			return s
		}
	}
	return nil
}

func escape(s string) string {
	return strings.NewReplacer(`\`, `\\`, ";", `\;`, ",", `\,`, "\r\n", `\n`, "\n", `\n`).Replace(s)
}

func unescape(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 == len(s) {
			b.WriteByte(s[i])
			continue
		}
		i++
		switch s[i] {
		case 'n', 'N':
			b.WriteByte('\n')
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// Parse reads an iCalendar object: its VCALENDAR.
func Parse(r io.Reader) (*Comp, error) {
	lines, err := unfold(r)
	if err != nil {
		return nil, err
	}
	var stack []*Comp
	var root *Comp
	for n, l := range lines {
		if l == "" {
			continue
		}
		p, err := parseLine(l)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", n+1, err)
		}
		switch p.Name {
		case "BEGIN":
			c := &Comp{Name: strings.ToUpper(p.Value)}
			if len(stack) > 0 {
				top := stack[len(stack)-1]
				top.Comps = append(top.Comps, c)
			} else if root == nil {
				root = c
			}
			stack = append(stack, c)
		case "END":
			if len(stack) == 0 || stack[len(stack)-1].Name != strings.ToUpper(p.Value) {
				return nil, fmt.Errorf("line %d: END:%s out of place", n+1, p.Value)
			}
			stack = stack[:len(stack)-1]
		default:
			if len(stack) == 0 {
				return nil, fmt.Errorf("line %d: %s outside a component", n+1, p.Name)
			}
			top := stack[len(stack)-1]
			top.Props = append(top.Props, p)
		}
	}
	if root == nil {
		return nil, errors.New("no calendar in it")
	}
	if len(stack) > 0 {
		return nil, fmt.Errorf("%s not ended", stack[len(stack)-1].Name)
	}
	return root, nil
}

// ParseString is Parse of s.
func ParseString(s string) (*Comp, error) { return Parse(strings.NewReader(s)) }

// unfold reads the content lines, each one again as it was before it was
// folded (a line going on starts with a space or a tab).
func unfold(r io.Reader) ([]string, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 8<<20)
	var lines []string
	for sc.Scan() {
		l := strings.TrimRight(sc.Text(), "\r")
		if len(l) > 0 && (l[0] == ' ' || l[0] == '\t') && len(lines) > 0 {
			lines[len(lines)-1] += l[1:]
			continue
		}
		lines = append(lines, l)
	}
	return lines, sc.Err()
}

// parseLine reads NAME;P=V;Q="V,W":VALUE.
func parseLine(l string) (*Prop, error) {
	p := &Prop{}
	i := strings.IndexAny(l, ";:")
	if i < 0 {
		return nil, fmt.Errorf("no value: %q", l)
	}
	p.Name = strings.ToUpper(l[:i])
	for l[i] == ';' {
		l = l[i+1:]
		eq := strings.IndexByte(l, '=')
		if eq < 0 {
			return nil, fmt.Errorf("a parameter without =: %q", l)
		}
		name := strings.ToUpper(l[:eq])
		l = l[eq+1:]
		var val string
		if strings.HasPrefix(l, `"`) {
			end := strings.IndexByte(l[1:], '"')
			if end < 0 {
				return nil, fmt.Errorf("a quote not closed: %q", l)
			}
			val, l = l[1:end+1], l[end+2:]
		} else {
			end := strings.IndexAny(l, ";:")
			if end < 0 {
				return nil, fmt.Errorf("no value: %q", l)
			}
			val, l = l[:end], l[end:]
		}
		p.Params = append(p.Params, [2]string{name, val})
		i = 0
		if l == "" {
			return nil, errors.New("no value")
		}
	}
	p.Value = l[i+1:]
	return p, nil
}

// Encode writes c as iCalendar: CRLF line ends, lines folded at 75 bytes
// (not inside a letter).
func (c *Comp) Encode(w io.Writer) error {
	var b strings.Builder
	c.encode(&b)
	_, err := io.WriteString(w, b.String())
	return err
}

// String is c encoded.
func (c *Comp) String() string {
	var b strings.Builder
	c.encode(&b)
	return b.String()
}

func (c *Comp) encode(b *strings.Builder) {
	fold(b, "BEGIN:"+c.Name)
	for _, p := range c.Props {
		var l strings.Builder
		l.WriteString(p.Name)
		for _, kv := range p.Params {
			v := kv[1]
			if strings.ContainsAny(v, ";:,") {
				v = `"` + v + `"`
			}
			l.WriteString(";" + kv[0] + "=" + v)
		}
		l.WriteString(":" + p.Value)
		fold(b, l.String())
	}
	for _, s := range c.Comps {
		s.encode(b)
	}
	fold(b, "END:"+c.Name)
}

func fold(b *strings.Builder, l string) {
	for first := true; ; first = false {
		max := 75
		if !first {
			b.WriteByte(' ')
			max = 74
		}
		if len(l) <= max {
			b.WriteString(l + "\r\n")
			return
		}
		cut := max
		for cut > 0 && l[cut]&0xC0 == 0x80 { // not in the middle of a letter
			cut--
		}
		b.WriteString(l[:cut] + "\r\n")
		l = l[cut:]
	}
}
