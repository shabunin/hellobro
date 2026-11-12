package gallery

import (
	"fmt"
	"image"
	"strings"
	"time"

	"hellobro/ink"
	"hellobro/ink/layout"
	"hellobro/ink/text"
	"hellobro/ink/theme"
	"hellobro/ink/widget"
)

// Mail is a demo made of ink's parts, after a desktop mail client:
// mailboxes, a list of messages, the one read; wide, the three side by
// side; narrower, the mailboxes in a dropdown; narrow, the list, then the
// message over it.
type Mail struct {
	msgs    []message
	rowBtn  []widget.Clickable
	box     int // the mailbox shown
	boxBtn  []widget.Clickable
	boxSel  widget.Select
	search  widget.Editor
	clr     widget.Clickable
	list    layout.Scroll
	read    layout.Scroll
	cur     int  // the message read; -1: none
	reading bool // narrow: the message shows, not the list
	narrow  bool // as last laid out

	back, reply, flag, archive, del, compose widget.Clickable

	composing    bool
	to, subject  widget.Editor
	body         widget.TextArea
	send, cancel widget.Clickable
}

type message struct {
	Box, From, To, Subject, Body string
	At                           time.Time
	Unread, Flagged              bool
}

var mailboxes = []string{"Inbox", "Flagged", "Sent", "Drafts", "Archive", "Bin"}

// The mail, made up: from, subject, the body, how long ago, read.
var letters = []struct {
	from, subject, body string
	ago                 time.Duration
	read, flag          bool
}{
	{"Mira Holt", "The book club, Thursday", "Hi,\n\nWe're on chapter nine this week, the one with the lighthouse. Bring the questions you had last time; I've got a few of my own.\n\nThe bookshop said we can stay till nine.\n\nMira", 40 * time.Minute, false, true},
	{"The library", "Your loan is due in three days", "The Long Way Round is due back on Monday. You can renew it once more online, or at the desk.\n\nThe library", 3 * time.Hour, false, false},
	{"Jonas Brandt", "Re: the climbing wall", "Tuesday works. The new routes on the north side are up, a couple of them look hard.\n\n> Shall we go on Tuesday, after work?\n> The wall's open till ten.\n\nJonas", 5 * time.Hour, true, false},
	{"Ana Ferreira", "Photos from the walk", "Here they are, the good ones at least. The one of the river at the bend came out best, I think.\n\nAna", 26 * time.Hour, true, false},
	{"Farmers' market", "This Saturday: apples, and a band", "The orchards are in this week with the first apples. A band plays from eleven by the fountain.", 30 * time.Hour, true, false},
	{"Theo Lind", "Notes for the review", "I put the notes in the shared folder. The second half needs a look; the first is fine as it is.\n\nTheo", 2 * 24 * time.Hour, false, false},
	{"The cinema", "Your tickets for Saturday", "Two seats, row F, 20:00. Show this at the door, or the code in the app.", 3 * 24 * time.Hour, true, true},
	{"Mira Holt", "Re: a lamp for the reading corner", "The brass one, I'd say. It throws the light lower, which is what you want for a page.\n\n> Which one, the brass or the white?\n\nMira", 4 * 24 * time.Hour, true, false},
	{"Dentist's", "A reminder: Tuesday 11:00", "This is to remind you of your appointment on Tuesday at 11:00, 14 Elm street. To move it, call us or reply to this.", 5 * 24 * time.Hour, true, false},
	{"Jonas Brandt", "A long ride in the spring?", "Thinking of riding along the coast in April, three or four days. Interested? No hurry to say.\n\nJonas", 8 * 24 * time.Hour, true, false},
	{"The bookshop", "Ordered in: The Map of Small Things", "The book you asked for is in. We'll keep it behind the counter for two weeks.", 10 * 24 * time.Hour, true, false},
	{"Ana Ferreira", "Recipe, as promised", "Soak the beans overnight. Onion, garlic, a bay leaf, then two hours on a low heat. Salt only at the end.\n\nAna", 12 * 24 * time.Hour, true, true},
}

func (m *Mail) seed(now time.Time) {
	for _, l := range letters {
		m.msgs = append(m.msgs, message{Box: "Inbox", From: l.from, To: "me", Subject: l.subject, Body: l.body,
			At: now.Add(-l.ago), Unread: !l.read, Flagged: l.flag})
	}
	m.msgs = append(m.msgs,
		message{Box: "Sent", From: "me", To: "Jonas Brandt", Subject: "the climbing wall", Body: "Shall we go on Tuesday, after work?\nThe wall's open till ten.", At: now.Add(-7 * time.Hour)},
		message{Box: "Drafts", From: "me", To: "Mira Holt", Subject: "Questions for chapter nine", Body: "1. Why does she go back to the lighthouse?\n2.", At: now.Add(-50 * time.Minute)},
		message{Box: "Archive", From: "Theo Lind", To: "me", Subject: "The plan for November", Body: "Here's the plan, as we said.", At: now.Add(-20 * 24 * time.Hour)},
	)
	m.rowBtn = make([]widget.Clickable, len(m.msgs))
	m.boxBtn = make([]widget.Clickable, len(mailboxes))
	m.list.Axis, m.read.Axis = layout.Vertical, layout.Vertical
	m.cur = -1
}

// shown is the messages the mailbox and the search leave, newest first.
func (m *Mail) shown() []int {
	q := strings.ToLower(strings.TrimSpace(m.search.Text()))
	var is []int
	for i, msg := range m.msgs {
		in := msg.Box == mailboxes[m.box] || mailboxes[m.box] == "Flagged" && msg.Flagged && msg.Box != "Bin"
		if !in {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(msg.From+" "+msg.Subject+" "+msg.Body), q) {
			continue
		}
		is = append(is, i)
	}
	// Newest first: they're mostly in that order already.
	for a := 1; a < len(is); a++ {
		for b := a; b > 0 && m.msgs[is[b]].At.After(m.msgs[is[b-1]].At); b-- {
			is[b], is[b-1] = is[b-1], is[b]
		}
	}
	return is
}

func (m *Mail) unread(box string) int {
	n := 0
	for _, msg := range m.msgs {
		if msg.Box == box && msg.Unread {
			n++
		}
	}
	return n
}

// Esc closes what's open: true if something was.
func (m *Mail) Esc() bool {
	switch {
	case m.composing:
		m.composing = false
	case m.reading && m.narrow:
		m.reading = false
	default:
		return false
	}
	return true
}

func (m *Mail) scroller() layout.Scroller {
	if m.reading && m.narrow {
		return &m.read
	}
	return &m.list
}

// when is a message's time as a list says it.
func when(t, now time.Time) string {
	switch d := dayOf(now).Sub(dayOf(t)); {
	case d <= 0:
		return t.Format("15:04")
	case d <= 24*time.Hour:
		return "yesterday"
	case d < 7*24*time.Hour:
		return t.Format("Monday")
	}
	return t.Format("2 Jan")
}

func (m *Mail) open(i int) {
	m.cur, m.reading = i, true
	m.msgs[i].Unread = false
	m.read = layout.Scroll{Axis: layout.Vertical}
}

// move puts the message read in box, and reads the next one.
func (m *Mail) move(gtx ink.Ctx, g *Gallery, box, said string) {
	if m.cur < 0 {
		return
	}
	is := m.shown()
	next := -1
	for k, i := range is {
		if i == m.cur && k+1 < len(is) {
			next = is[k+1]
		}
	}
	m.msgs[m.cur].Box = box
	g.toast.Show(gtx, said)
	g.Said = "mail: " + said
	m.cur, m.reading = -1, false
	if next >= 0 {
		m.open(next)
		m.reading = false
	}
}

func (m *Mail) startCompose(gtx ink.Ctx, to, subject, body string) {
	m.composing = true
	m.to.SetText(to)
	m.subject.SetText(subject)
	m.body.SetText(body)
	m.body.SetCaret(0) // a reply's written over what it quotes
	if to == "" {
		gtx.Focus(&m.to)
	} else {
		gtx.Focus(&m.body)
	}
}

// Layout lays it out in what it's given; the new message is Over's.
func (m *Mail) Layout(gtx ink.Ctx, g *Gallery) ink.Dims {
	th := g.Th
	if m.msgs == nil {
		m.seed(gtx.Now)
	}
	for i := range m.boxBtn {
		if m.boxBtn[i].Clicked(gtx) {
			m.box, m.boxSel.Value, m.cur, m.reading = i, i, -1, false
			m.list = layout.Scroll{Axis: layout.Vertical}
			g.Said = "mail: " + mailboxes[i]
		}
	}
	if m.boxSel.Changed(gtx, len(mailboxes)) {
		m.box, m.cur, m.reading = m.boxSel.Value, -1, false
		m.list = layout.Scroll{Axis: layout.Vertical}
		g.Said = "mail: " + mailboxes[m.box]
	}
	if m.search.Changed(gtx) {
		m.list = layout.Scroll{Axis: layout.Vertical}
	}
	for i := range m.rowBtn {
		if m.rowBtn[i].Clicked(gtx) {
			m.open(i)
			g.Said = "mail: " + m.msgs[i].Subject
		}
	}
	if m.back.Clicked(gtx) {
		m.reading = false
	}
	if m.compose.Clicked(gtx) {
		m.startCompose(gtx, "", "", "")
	}
	if m.cur >= 0 {
		msg := &m.msgs[m.cur]
		if m.reply.Clicked(gtx) {
			var q []string
			for _, l := range strings.Split(msg.Body, "\n") {
				if l = strings.TrimSpace(l); l != "" {
					l = "> " + l
				}
				q = append(q, l)
			}
			subj := msg.Subject
			if !strings.HasPrefix(subj, "Re: ") {
				subj = "Re: " + subj
			}
			m.startCompose(gtx, msg.From, subj, "\n\n"+strings.Join(q, "\n"))
		}
		if m.flag.Clicked(gtx) {
			msg.Flagged = !msg.Flagged
			g.Said = "mail: flagged " + onOff(msg.Flagged)
		}
		if m.archive.Clicked(gtx) {
			m.move(gtx, g, "Archive", "archived")
		}
		if m.del.Clicked(gtx) {
			m.move(gtx, g, "Bin", "moved to the bin")
		}
	}
	w := gtx.Constraints.Max.X
	wide, narrow := w >= gtx.Dp(1100), w < gtx.Dp(700)
	m.narrow = narrow
	rule := func(gtx ink.Ctx) ink.Dims {
		h := gtx.Constraints.Max.Y
		ink.Fill(gtx.Ops, image.Rect(0, 0, max(gtx.Dp(th.Border), 1), h), th.Fg)
		return ink.Dims{Size: image.Pt(max(gtx.Dp(th.Border), 1), h)}
	}
	listCol := func(gtx ink.Ctx) ink.Dims { return m.listColumn(gtx, g, !wide) }
	switch {
	case wide:
		return layout.Flex{}.Layout(gtx,
			layout.Rigid(func(gtx ink.Ctx) ink.Dims {
				gtx.Constraints = ink.Exact(image.Pt(gtx.Dp(220), gtx.Constraints.Max.Y))
				return m.boxColumn(gtx, g)
			}),
			layout.Rigid(rule),
			layout.Rigid(func(gtx ink.Ctx) ink.Dims {
				gtx.Constraints = ink.Exact(image.Pt(gtx.Dp(400), gtx.Constraints.Max.Y))
				return listCol(gtx)
			}),
			layout.Rigid(rule),
			layout.Flexed(1, func(gtx ink.Ctx) ink.Dims { return m.message(gtx, g, false) }))
	case narrow:
		if m.reading && m.cur >= 0 {
			return m.message(gtx, g, true)
		}
		return listCol(gtx)
	}
	return layout.Flex{}.Layout(gtx,
		layout.Flexed(0.42, listCol),
		layout.Rigid(rule),
		layout.Flexed(0.58, func(gtx ink.Ctx) ink.Dims { return m.message(gtx, g, false) }))
}

// boxColumn is the mailboxes, each with its unread count.
func (m *Mail) boxColumn(gtx ink.Ctx, g *Gallery) ink.Dims {
	th := g.Th
	kids := []layout.FlexChild{
		layout.Rigid(func(gtx ink.Ctx) ink.Dims {
			return layout.UniformInset(th.Pad).Layout(gtx, func(gtx ink.Ctx) ink.Dims {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				b := theme.Button(th, &m.compose, "new message")
				b.Strong = true
				return b.Layout(gtx)
			})
		}),
		layout.Rigid(theme.Section(th, "Mailboxes")),
	}
	for i, name := range mailboxes {
		kids = append(kids, layout.Rigid(func(gtx ink.Ctx) ink.Dims {
			r := theme.Row(th, &m.boxBtn[i], name)
			if n := m.unread(name); n > 0 {
				r.Value = fmt.Sprint(n)
			}
			d := r.Layout(gtx)
			if i == m.box {
				ink.Fill(gtx.Ops, image.Rect(0, 0, gtx.Dp(5), d.Size.Y), th.Fg)
			}
			return d
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
}

// listColumn is the search, the messages; with the mailboxes in a
// dropdown over them if pick.
func (m *Mail) listColumn(gtx ink.Ctx, g *Gallery, pick bool) ink.Dims {
	th := g.Th
	search := theme.Editor(th, &m.search, "search")
	search.Clear = &m.clr
	var kids []layout.FlexChild
	if pick {
		kids = append(kids, layout.Rigid(func(gtx ink.Ctx) ink.Dims {
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, theme.Select(th, &m.boxSel, "Mailbox", mailboxes...).Layout),
				layout.Rigid(func(gtx ink.Ctx) ink.Dims {
					return layout.Inset{Right: th.Pad}.Layout(gtx, theme.Button(th, &m.compose, "new").Layout)
				}))
		}))
	}
	is := m.shown()
	kids = append(kids,
		layout.Rigid(func(gtx ink.Ctx) ink.Dims {
			return layout.UniformInset(th.Pad).Layout(gtx, search.Layout)
		}),
		layout.Rigid(theme.Divider(th)),
		layout.Flexed(1, func(gtx ink.Ctx) ink.Dims {
			if len(is) == 0 {
				return layout.UniformInset(th.Pad*2).Layout(gtx, theme.Small(th, "nothing here").Layout)
			}
			return theme.List(th, &m.list).Layout(gtx, len(is), func(gtx ink.Ctx, k int) ink.Dims {
				return m.row(gtx, th, is[k])
			})
		}))
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
}

// row is a message in the list: unread's dot, who from, when, the
// subject, the start of it.
func (m *Mail) row(gtx ink.Ctx, th *theme.Theme, i int) ink.Dims {
	msg := m.msgs[i]
	clk := &m.rowBtn[i]
	return clk.Layout(gtx, func(gtx ink.Ctx) ink.Dims {
		w := gtx.Constraints.Max.X
		reg := th.Face(gtx, th.TextSize, text.Regular)
		bold := th.Face(gtx, th.TextSize, text.Bold)
		small := th.Face(gtx, th.TextSize*0.85, text.Regular)
		pad, left := gtx.Dp(th.Pad), gtx.Dp(26)
		lh, sh := text.Height(reg), text.Height(small)
		h := pad*2 + lh + 2*sh + gtx.Dp(4)
		r := image.Rect(0, 0, w, h)
		ink.Fill(gtx.Ops, r, th.Bg)
		if i == m.cur {
			ink.Fill(gtx.Ops, r, ink.Light)
		}
		y := pad + text.Ascent(reg)
		if msg.Unread {
			ink.Disc(gtx.Ops, image.Pt(left/2+gtx.Dp(2), pad+lh/2), gtx.Dp(5), th.Fg)
		}
		who := msg.From
		if m.msgs[i].Box == "Sent" || m.msgs[i].Box == "Drafts" {
			who = "to " + msg.To
		}
		at := when(msg.At, gtx.Now)
		aw := text.Width(small, at)
		f := reg
		if msg.Unread {
			f = bold
		}
		ink.TextAt(gtx.Ops, f, left, y, text.Fit(f, who, w-left-aw-3*pad), th.Fg)
		ink.TextAt(gtx.Ops, small, w-pad-aw, y, at, th.Quiet)
		y += sh + gtx.Dp(2)
		subj := msg.Subject
		room := w - left - pad
		if msg.Flagged {
			fx := w - pad - gtx.Dp(10)
			ink.Polygon(gtx.Ops, th.Fg, image.Pt(fx, y-gtx.Dp(12)), image.Pt(fx+gtx.Dp(10), y-gtx.Dp(8)), image.Pt(fx, y-gtx.Dp(4)))
			ink.Fill(gtx.Ops, image.Rect(fx, y-gtx.Dp(12), fx+gtx.Dp(2), y+gtx.Dp(2)), th.Fg)
			room -= gtx.Dp(16)
		}
		ink.TextAt(gtx.Ops, small, left, y, text.Fit(small, subj, room), th.Fg)
		y += sh
		prev := strings.Join(strings.Fields(msg.Body), " ")
		ink.TextAt(gtx.Ops, small, left, y, text.Fit(small, prev, w-left-pad), th.Quiet)
		ink.Fill(gtx.Ops, image.Rect(left, h-1, w, h), ink.Light)
		if clk.Pressed() {
			ink.Invert(gtx.Ops, r)
		}
		return ink.Dims{Size: r.Size()}
	})
}

// message is the one read: its buttons, who it's from, what it says.
func (m *Mail) message(gtx ink.Ctx, g *Gallery, back bool) ink.Dims {
	th := g.Th
	if m.cur < 0 {
		return layout.Center.Layout(gtx, theme.Small(th, "no message picked").Layout)
	}
	msg := m.msgs[m.cur]
	sp := layout.Rigid(layout.Spacer{Width: th.Pad}.Layout)
	flag := "flag"
	if msg.Flagged {
		flag = "unflag"
	}
	tools := []layout.FlexChild{}
	if back {
		tools = append(tools, layout.Rigid(theme.Button(th, &m.back, "‹ "+mailboxes[m.box]).Layout), layout.Flexed(1, layout.Spacer{}.Layout))
	}
	tools = append(tools,
		layout.Rigid(theme.Button(th, &m.reply, "reply").Layout), sp,
		layout.Rigid(theme.Button(th, &m.flag, flag).Layout), sp,
		layout.Rigid(theme.Button(th, &m.archive, "archive").Layout), sp,
		layout.Rigid(theme.Button(th, &m.del, "delete").Layout))
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx ink.Ctx) ink.Dims {
			return layout.UniformInset(th.Pad).Layout(gtx, func(gtx ink.Ctx) ink.Dims {
				return layout.Flex{Alignment: layout.Middle, Spacing: layout.SpaceStart}.Layout(gtx, tools...)
			})
		}),
		layout.Rigid(theme.Divider(th)),
		layout.Flexed(1, func(gtx ink.Ctx) ink.Dims {
			return layout.Inset{Left: th.Pad * 2, Top: th.Pad}.Layout(gtx, theme.ScrollBox(th, &m.read, func(gtx ink.Ctx) ink.Dims {
				return layout.Inset{Right: th.Pad * 2, Bottom: th.Pad * 4}.Layout(gtx, func(gtx ink.Ctx) ink.Dims {
					kids := []layout.FlexChild{
						layout.Rigid(theme.Title(th, msg.Subject).Layout),
						layout.Rigid(layout.Spacer{Height: th.Pad}.Layout),
						layout.Rigid(theme.Label(th, th.TextSize, msg.From).Layout),
						layout.Rigid(theme.Small(th, "to "+msg.To+", "+msg.At.Format("Monday 2 January 2006, 15:04")).Layout),
						layout.Rigid(layout.Spacer{Height: th.Pad * 2}.Layout),
					}
					for _, p := range strings.Split(msg.Body, "\n\n") {
						kids = append(kids, layout.Rigid(func(gtx ink.Ctx) ink.Dims {
							if !strings.HasPrefix(p, ">") {
								return theme.Body(th, p).Layout(gtx)
							}
							// Quoted: aside, a bar down its left.
							var ls []string
							for _, l := range strings.Split(p, "\n") {
								ls = append(ls, strings.TrimSpace(strings.TrimPrefix(l, ">")))
							}
							d := layout.Inset{Left: th.Pad * 2}.Layout(gtx, func(gtx ink.Ctx) ink.Dims {
								l := theme.Body(th, strings.Join(ls, "\n"))
								l.Gray = th.Quiet
								return l.Layout(gtx)
							})
							ink.Fill(gtx.Ops, image.Rect(0, 0, gtx.Dp(3), d.Size.Y), th.Quiet)
							return d
						}), layout.Rigid(layout.Spacer{Height: th.Pad * 1.5}.Layout))
					}
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
				})
			}))
		}),
	)
}

// Over lays the new message out over all the gallery, if it's being
// written: a sheet with who to, the subject, the text.
func (m *Mail) Over(gtx ink.Ctx, g *Gallery) {
	th := g.Th
	if m.cancel.Clicked(gtx) {
		m.composing = false
	}
	if m.send.Clicked(gtx) && m.composing {
		to := strings.TrimSpace(m.to.Text())
		if to == "" {
			g.toast.Show(gtx, "who to?")
			gtx.Focus(&m.to)
		} else {
			m.msgs = append(m.msgs, message{Box: "Sent", From: "me", To: to, Subject: m.subject.Text(), Body: m.body.Text(), At: gtx.Now})
			m.rowBtn = append(m.rowBtn, widget.Clickable{})
			m.composing = false
			g.toast.Show(gtx, "sent")
			g.Said = "mail: sent to " + to
		}
	}
	if !m.composing {
		return
	}
	size := gtx.Constraints.Max
	ink.Input(gtx.Ops, ink.Area{Rect: image.Rectangle{Max: size}}) // the backdrop: taps stop here
	edge := gtx.Dp(th.Pad * 6)
	r := image.Rect(edge, edge, size.X-edge, size.Y-edge)
	if r.Dx() > gtx.Dp(900) {
		r.Min.X = (size.X - gtx.Dp(900)) / 2
		r.Max.X = r.Min.X + gtx.Dp(900)
	}
	b := max(gtx.Dp(th.Border), 1)
	ink.Fill(gtx.Ops, r.Inset(-b*3), th.Bg)
	ink.Fill(gtx.Ops, r.Add(image.Pt(b*4, b*4)), th.Quiet)
	ink.Fill(gtx.Ops, r, th.Bg)
	ink.Frame(gtx.Ops, r, b*2, th.Fg)
	st := ink.Offset(gtx.Ops, r.Min)
	cg := gtx
	cg.Constraints = ink.Exact(r.Size())
	field := func(label string, e *widget.Editor) layout.FlexChild {
		return layout.Rigid(func(gtx ink.Ctx) ink.Dims {
			return layout.Inset{Left: th.Pad * 2, Right: th.Pad * 2, Top: th.Pad}.Layout(gtx, func(gtx ink.Ctx) ink.Dims {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(func(gtx ink.Ctx) ink.Dims {
						gtx.Constraints.Min.X = gtx.Dp(90)
						return theme.Label(th, th.TextSize, label).Layout(gtx)
					}),
					layout.Flexed(1, theme.Editor(th, e, "").Layout))
			})
		})
	}
	send := theme.Button(th, &m.send, "send")
	send.Strong = true
	title := "New message"
	if strings.HasPrefix(m.subject.Text(), "Re: ") {
		title = "Reply"
	}
	layout.Flex{Axis: layout.Vertical}.Layout(cg,
		layout.Rigid(func(gtx ink.Ctx) ink.Dims {
			return layout.UniformInset(th.Pad).Layout(gtx, func(gtx ink.Ctx) ink.Dims {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Rigid(theme.Button(th, &m.cancel, "cancel").Layout),
					layout.Flexed(1, func(gtx ink.Ctx) ink.Dims {
						l := theme.Label(th, th.TextSize*1.2, title)
						l.Style, l.Align = text.Bold, theme.Center
						return l.Layout(gtx)
					}),
					layout.Rigid(send.Layout))
			})
		}),
		layout.Rigid(theme.Divider(th)),
		field("To", &m.to),
		field("Subject", &m.subject),
		layout.Flexed(1, func(gtx ink.Ctx) ink.Dims {
			return layout.UniformInset(th.Pad*2).Layout(gtx, func(gtx ink.Ctx) ink.Dims {
				a := theme.TextArea(th, &m.body)
				a.Tools = false
				a.Hint = "what to say"
				return a.Layout(gtx)
			})
		}))
	st.Pop()
}
