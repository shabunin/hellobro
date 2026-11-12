package main

// The AI models' window: the API's list as a table, to search, sort by
// a column and filter by what they can do; a model tapped is told of
// below it (what it reads and can do, its limits and prices), and
// "use it" makes it the AI notebooks' model.

import (
	"fmt"
	"image"
	"log"
	"slices"
	"strings"
	"time"

	"hellobro/ai"
	"hellobro/ink"
	"hellobro/ink/layout"
	"hellobro/ink/theme"
	"hellobro/ink/widget"
)

const modelsKey = "models"

// modelFilters are the chips over the table: on, only the models that
// have it show.
var modelFilters = []struct {
	name string
	has  func(m ai.Model) bool
}{
	{"pictures", func(m ai.Model) bool { return m.Images || !m.Known }},
	{"PDF", func(m ai.Model) bool { return m.PDF }},
	{"audio", func(m ai.Model) bool { return m.Audio }},
	{"reasoning", func(m ai.Model) bool { return m.Reasoning }},
	{"tools", func(m ai.Model) bool { return m.Tools }},
	{"JSON", func(m ai.Model) bool { return m.JSON }},
	{"open weights", func(m ai.Model) bool { return m.OpenWeights }},
	{"free", func(m ai.Model) bool { return m.In == 0 && m.Out == 0 }},
}

var modelCols = []theme.Column{
	{Title: "model", Weight: 2.2},
	{Title: "$ in", Weight: 0.7, Align: theme.Right},
	{Title: "$ out", Weight: 0.7, Align: theme.Right},
	{Title: "context", Weight: 0.8, Align: theme.Right},
	{Title: "features", Weight: 2.9},
	{Title: "released", Weight: 1.1, Align: theme.Right},
}

type modelsUI struct {
	a             *bro
	find          widget.Editor
	findClr       widget.Clickable
	chips         []widget.Clickable
	on            []bool
	table         widget.Table
	shown         []ai.Model // as filtered and sorted
	at            time.Time  // the list's, as shown
	sel           string     // the model told of below
	use, conf     widget.Clickable
	refresh, back widget.Clickable

	typing bool
	vk     int
}

func newModelsUI(a *bro) inkUI {
	u := &modelsUI{a: a, chips: make([]widget.Clickable, len(modelFilters)), on: make([]bool, len(modelFilters))}
	u.on[0] = true // the diary sends pictures
	u.table.Sort = 1
	u.sel = a.diaryModelNow()
	return u
}

func (u *modelsUI) Status() string {
	ms := u.a.models()
	switch {
	case ms.fetching:
		return "fetching the list…"
	case ms.err != "":
		return "not fetched: " + ms.err
	case !ms.At.IsZero():
		return "fetched " + ago(time.Since(ms.At))
	}
	return ""
}

// ago is d, said shortly.
func ago(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d min ago", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%d h ago", int(d.Hours()))
	}
	return fmt.Sprintf("%d days ago", int(d.Hours()/24))
}

func (u *modelsUI) take() {
	if u.vk != 0 {
		u.a.vkAsk(u.vk > 0)
		u.vk = 0
	}
}

// filter makes shown of the list: searched, filtered, sorted.
func (u *modelsUI) filter() {
	q := strings.ToLower(strings.TrimSpace(u.find.Text()))
	u.shown = u.shown[:0]
	for _, m := range u.a.models().List {
		ok := q == "" || strings.Contains(strings.ToLower(m.ID+" "+m.Name+" "+m.Family+" "+m.About), q)
		for i, f := range modelFilters {
			ok = ok && (!u.on[i] || f.has(m))
		}
		if ok {
			u.shown = append(u.shown, m)
		}
	}
	price := func(f float64) float64 { // not said: dearest
		if f < 0 {
			return 1e9
		}
		return f
	}
	slices.SortStableFunc(u.shown, func(x, y ai.Model) int {
		c := 0
		switch u.table.Sort {
		case 0:
			c = strings.Compare(x.ID, y.ID)
		case 1:
			c = cmpF(price(x.In), price(y.In))
			if c == 0 {
				c = cmpF(price(x.Out), price(y.Out))
			}
		case 2:
			c = cmpF(price(x.Out), price(y.Out))
		case 3:
			c = x.Context - y.Context
		case 4:
			c = len(x.Features()) - len(y.Features())
		case 5:
			c = strings.Compare(x.Released, y.Released)
		}
		if u.table.Desc {
			c = -c
		}
		if c == 0 {
			c = strings.Compare(x.ID, y.ID)
		}
		return c
	})
}

func cmpF(a, b float64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// dollars is a price a million tokens, in a cell.
func dollars(f float64) string {
	switch {
	case f < 0:
		return "—"
	case f == 0:
		return "free"
	}
	return "$" + price(f)
}

// tokens is a count of them, short: 200k, 1M.
func tokens(n int) string {
	switch {
	case n <= 0:
		return ""
	case n >= 10e6:
		return fmt.Sprintf("%dM", n/1e6)
	case n >= 1e6:
		return strings.TrimSuffix(fmt.Sprintf("%.1f", float64(n)/1e6), ".0") + "M"
	case n >= 1000:
		return fmt.Sprintf("%dk", n/1000)
	}
	return fmt.Sprint(n)
}

// short are a model's features as the table has room for.
func short(m ai.Model) string {
	r := strings.NewReplacer("pictures", "pics", "reasoning", "thinks", "open weights", "open")
	return r.Replace(strings.Join(m.Features(), " "))
}

func (u *modelsUI) Layout(gtx ink.Ctx) ink.Dims {
	a, th := u.a, u.a.theme()
	ms := a.models()
	a.modelsWant()
	ink.Fill(gtx.Ops, image.Rectangle{Max: gtx.Constraints.Max}, th.Bg)

	again := !ms.At.Equal(u.at)
	u.at = ms.At
	if u.find.Changed(gtx) || u.findClr.Clicked(gtx) {
		again = true
		u.table.Body.ScrollTo(0)
	}
	for i := range u.chips {
		if u.chips[i].Clicked(gtx) {
			u.on[i], again = !u.on[i], true
			u.table.Body.ScrollTo(0)
		}
	}
	if u.table.Sorted(gtx) || again {
		u.filter()
	}
	if i, ok := u.table.Clicked(gtx); ok && i < len(u.shown) {
		u.sel = u.shown[i].ID
	}
	if u.refresh.Clicked(gtx) {
		a.modelsFetch(true)
	}
	if u.use.Clicked(gtx) {
		a.useModel(u.sel)
	}
	if u.conf.Clicked(gtx) {
		a.useModel("conf")
	}
	if u.back.Clicked(gtx) {
		if g := a.gui(modelsKey); g != nil {
			a.closeWin(g)
			return ink.Dims{Size: gtx.Constraints.Max}
		}
	}
	if typing := u.find.Focused(); typing != u.typing {
		u.typing = typing
		u.vk = map[bool]int{true: 1, false: -1}[typing]
	}

	now := a.diaryModelNow() // as the setting says, or ai.conf
	sp := layout.Rigid(layout.Spacer{Width: th.Pad}.Layout)
	search := theme.Editor(th, &u.find, "search the models: a name, a maker, a word of what they're for")
	search.Clear = &u.findClr
	t := theme.Table(th, &u.table, modelCols...)
	t.Mark = strings.TrimSpace(u.find.Text())
	t.Empty = "none has all that"
	if len(ms.List) == 0 {
		t.Empty = "no list yet: " + map[bool]string{true: "fetching it…", false: "refresh, with Wi-Fi on"}[ms.fetching]
	}
	for i, m := range u.shown {
		if m.ID == u.sel {
			t.Sel = i
		}
	}
	inset := func(w ink.Widget) ink.Widget {
		return func(gtx ink.Ctx) ink.Dims {
			return layout.Inset{Left: th.Pad * 1.5, Right: th.Pad * 1.5, Top: th.Pad / 2, Bottom: th.Pad / 2}.Layout(gtx, w)
		}
	}
	kids := []layout.FlexChild{
		layout.Rigid(inset(func(gtx ink.Ctx) ink.Dims {
			nowS := now
			if a.setting("ai-model") == "conf" {
				nowS = "as ai.conf says: " + now
			}
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
				layout.Rigid(theme.Button(th, &u.back, "‹ back").Layout),
				layout.Flexed(1, func(gtx ink.Ctx) ink.Dims {
					return layout.Inset{Left: th.Pad * 2}.Layout(gtx, theme.Body(th, "the AI notebooks ask: "+nowS).Layout)
				}),
				layout.Rigid(theme.Button(th, &u.refresh, map[bool]string{true: "fetching…", false: "refresh"}[ms.fetching]).Layout))
		})),
		layout.Rigid(inset(search.Layout)),
		layout.Rigid(inset(func(gtx ink.Ctx) ink.Dims {
			var cs []layout.FlexChild
			for i, f := range modelFilters {
				if i > 0 {
					cs = append(cs, sp)
				}
				b := theme.Button(th, &u.chips[i], f.name)
				b.Strong = u.on[i]
				cs = append(cs, layout.Rigid(b.Layout))
			}
			return layout.Flex{Alignment: layout.Middle}.Layout(gtx, cs...)
		})),
		layout.Rigid(inset(theme.Small(th, fmt.Sprintf("%d of %d models · the filled ones only those that have it · a head sorts, again the other way · prices a million tokens", len(u.shown), len(ms.List))).Layout)),
		layout.Flexed(1, func(gtx ink.Ctx) ink.Dims {
			return layout.Inset{Left: th.Pad, Right: th.Pad}.Layout(gtx, func(gtx ink.Ctx) ink.Dims {
				return t.Layout(gtx, len(u.shown), func(i, c int) string {
					m := u.shown[i]
					switch c {
					case 0:
						if m.ID == now {
							return "• " + m.ID
						}
						return m.ID
					case 1:
						return dollars(m.In)
					case 2:
						return dollars(m.Out)
					case 3:
						return tokens(m.Context)
					case 4:
						return short(m)
					}
					return m.Released
				})
			})
		}),
	}
	if m, ok := a.modelOf(u.sel); ok {
		kids = append(kids, layout.Rigid(func(gtx ink.Ctx) ink.Dims {
			gtx.Constraints.Max.Y = gtx.Constraints.Max.Y / 2
			return u.details(gtx, th, m, now)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, kids...)
}

// details tell of m: what it reads and does, its limits, its prices, how
// it's asked; and the buttons to use it.
func (u *modelsUI) details(gtx ink.Ctx, th *theme.Theme, m ai.Model, now string) ink.Dims {
	var lines []string
	reads := []string{"text"}
	for _, r := range []struct {
		on bool
		s  string
	}{{m.Images, "pictures"}, {m.PDF, "PDFs"}, {m.Audio, "audio"}, {m.Video, "video"}} {
		if r.on {
			reads = append(reads, r.s)
		}
	}
	var can []string
	for _, r := range []struct {
		on bool
		s  string
	}{{m.Reasoning, "thinks first (reasoning)"}, {m.Tools, "calls tools"}, {m.JSON, "answers in a JSON schema"}} {
		if r.on {
			can = append(can, r.s)
		}
	}
	l := "reads " + strings.Join(reads, ", ")
	if !m.Known {
		l = "the list doesn't say what it reads"
	}
	if len(can) > 0 {
		l += " · " + strings.Join(can, ", ")
	}
	if m.OpenWeights {
		l += " · open weights"
	}
	lines = append(lines, l)
	var lim []string
	if m.Context > 0 {
		lim = append(lim, "takes "+tokens(m.Context)+" tokens")
	}
	if m.MaxOut > 0 {
		lim = append(lim, "writes "+tokens(m.MaxOut)+" at most")
	}
	if m.Knowledge != "" {
		lim = append(lim, "knows up to "+m.Knowledge)
	}
	if m.Released != "" {
		lim = append(lim, "released "+m.Released)
	}
	if len(lim) > 0 {
		lines = append(lines, strings.Join(lim, " · "))
	}
	p := "price not said"
	if m.In >= 0 {
		p = dollars(m.In) + " in, " + dollars(m.Out) + " out, a million tokens"
		if m.Cached > 0 {
			p += "; " + dollars(m.Cached) + " read from the cache"
		}
		if m.Peak {
			p += "; dearer at some hours"
		}
	}
	how := map[string]string{"anthropic": "Anthropic's messages", "openai": "OpenAI's chat completions", "responses": "OpenAI's Responses API"}[m.Proto]
	if how != "" {
		p += " · asked by " + how
	}
	lines = append(lines, p)
	if m.Proto == "-" {
		lines = append(lines, "hellobro can't ask it: its API is one hellobro doesn't speak")
	} else if m.Known && !m.Images {
		lines = append(lines, "it doesn't read pictures: the notebook's handwriting is sent as one, so it would see nothing of it")
	}

	title := m.ID
	if m.Name != "" && m.Name != m.ID {
		title = m.Name + " (" + m.ID + ")"
	}
	use := theme.Button(th, &u.use, "use it")
	use.Strong = true
	if m.ID == now {
		use.Text = "in use"
	}
	conf := theme.Button(th, &u.conf, "as ai.conf says")
	body := func(gtx ink.Ctx) ink.Dims {
		var ks []layout.FlexChild
		if m.About != "" {
			ks = append(ks, layout.Rigid(theme.Body(th, m.About).Layout), layout.Rigid(layout.Spacer{Height: th.Pad / 2}.Layout))
		}
		for _, l := range lines {
			ks = append(ks, layout.Rigid(theme.Small(th, l).Layout))
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, ks...)
	}
	return layout.Inset{Left: th.Pad * 1.5, Right: th.Pad * 1.5, Top: th.Pad, Bottom: th.Pad}.Layout(gtx, func(gtx ink.Ctx) ink.Dims {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx ink.Ctx) ink.Dims {
				return layout.Flex{Alignment: layout.Middle}.Layout(gtx,
					layout.Flexed(1, theme.Title(th, title).Layout),
					layout.Rigid(conf.Layout), layout.Rigid(layout.Spacer{Width: th.Pad}.Layout),
					layout.Rigid(use.Layout))
			}),
			layout.Rigid(layout.Spacer{Height: th.Pad / 2}.Layout),
			layout.Rigid(body))
	})
}

// useModel makes id the AI notebooks' model ("conf": as ai.conf says).
func (a *bro) useModel(id string) {
	i := settingIndex("ai-model")
	st := settingList[i]
	k := slices.Index(st.values, id)
	if k < 0 {
		if m, ok := a.modelOf(id); !ok || m.Proto == "-" {
			return
		}
		a.applyModels()
		if k = slices.Index(settingList[i].values, id); k < 0 {
			return
		}
	}
	a.set["ai-model"] = k
	a.set.save()
	log.Printf("settings: ai-model = %s", id)
	a.noteSettingsRedraw()
	a.inkRedraw("settings")
	a.inkRedraw(modelsKey)
}

// modelOf is the listed model id.
func (a *bro) modelOf(id string) (ai.Model, bool) {
	for _, m := range a.models().List {
		if m.ID == id {
			return m, true
		}
	}
	return ai.Model{}, false
}
