// hellobro-server is an SSH server (charmbracelet/wish) serving a Bubble
// Tea TUI, to give hellobro's terminal something rich to show: live stats
// with bars and a sparkline, a todo list, a chat shared by everyone
// connected, and a showcase of borders, colors and text attributes.
//
// Anyone who can reach it gets in (any key, any password): keep it on the LAN.
//
//	go run . [-listen :2222]
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/ssh"
	"charm.land/wish/v2"
	"charm.land/wish/v2/activeterm"
	"charm.land/wish/v2/bubbletea"
	"charm.land/wish/v2/logging"
)

func main() {
	listen := flag.String("listen", ":2222", "address to listen on")
	flag.Parse()

	s, err := wish.NewServer(
		wish.WithAddress(*listen),
		wish.WithHostKeyPath(".ssh/hellobro_server_ed25519"),
		wish.WithPublicKeyAuth(func(ssh.Context, ssh.PublicKey) bool { return true }),
		wish.WithPasswordAuth(func(ssh.Context, string) bool { return true }),
		wish.WithMiddleware(
			bubbletea.MiddlewareWithProgramHandler(newProgram),
			activeterm.Middleware(), // a PTY is required
			logging.Middleware(),
		),
	)
	if err != nil {
		log.Fatal(err)
	}
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)
	log.Printf("hellobro-server on %s: ssh -p PORT anyone@this-host", *listen)
	go func() {
		if err := s.ListenAndServe(); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
			log.Fatal(err)
		}
	}()
	<-done
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.Shutdown(ctx)
}

// --- the chat, shared by all sessions ---

type chatLine struct {
	at        time.Time
	who, text string
}

type chatMsg struct{}   // "the chat changed"
type peopleMsg struct{} // someone came or went

var hub = struct {
	sync.Mutex
	lines    []chatLine
	programs map[*tea.Program]string
}{programs: map[*tea.Program]string{}}

func broadcast(msg tea.Msg) {
	hub.Lock()
	ps := make([]*tea.Program, 0, len(hub.programs))
	for p := range hub.programs {
		ps = append(ps, p)
	}
	hub.Unlock()
	for _, p := range ps {
		go p.Send(msg)
	}
}

func say(who, text string) {
	hub.Lock()
	hub.lines = append(hub.lines, chatLine{time.Now(), who, text})
	if len(hub.lines) > 200 {
		hub.lines = hub.lines[len(hub.lines)-200:]
	}
	hub.Unlock()
	broadcast(chatMsg{})
}

func people() []string {
	hub.Lock()
	defer hub.Unlock()
	var names []string
	for _, n := range hub.programs {
		names = append(names, n)
	}
	return names
}

func newProgram(sess ssh.Session) *tea.Program {
	pty, _, _ := sess.Pty()
	who := fmt.Sprintf("%s@%s", sess.User(), strings.Split(sess.RemoteAddr().String(), ":")[0])
	m := newModel(who, pty.Term)
	p := tea.NewProgram(m, bubbletea.MakeOptions(sess)...)
	hub.Lock()
	hub.programs[p] = who
	hub.Unlock()
	say("server", who+" joined ("+pty.Term+")")
	broadcast(peopleMsg{})
	go func() {
		<-sess.Context().Done()
		hub.Lock()
		delete(hub.programs, p)
		hub.Unlock()
		say("server", who+" left")
		broadcast(peopleMsg{})
	}()
	return p
}

// --- the model ---

var pages = []string{"Dashboard", "Todo", "Chat", "Showcase", "About"}

type tickMsg time.Time

type todo struct {
	text string
	done bool
}

type model struct {
	who, term string
	w, h      int
	page      int
	focusMenu bool
	dark      bool
	st        styles
	stats     stats
	cpuHist   []float64
	todos     []todo
	todoCur   int
	input     string // the todo / chat input line
	typing    bool   // the input line has the keys
	started   time.Time
	clicks    int
	lastMouse string
}

func newModel(who, term string) model {
	m := model{who: who, term: term, focusMenu: true, started: time.Now(),
		todos: []todo{{"plug in the Kobo", true}, {"ssh into this from hellobro", true}, {"run something with ncurses", false}, {"say hi in the chat", false}}}
	m.st = newStyles(false)
	m.stats = readStats(stats{})
	return m
}

func (m model) Init() tea.Cmd {
	return tea.Batch(tea.RequestBackgroundColor, tick())
}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.w, m.h = msg.Width, msg.Height
	case tea.BackgroundColorMsg:
		m.dark = msg.IsDark()
		m.st = newStyles(m.dark)
	case tickMsg:
		m.stats = readStats(m.stats)
		m.cpuHist = append(m.cpuHist, m.stats.cpu)
		if len(m.cpuHist) > 200 {
			m.cpuHist = m.cpuHist[1:]
		}
		return m, tick()
	case chatMsg, peopleMsg:
	case tea.MouseClickMsg:
		return m.click(msg.Mouse())
	case tea.MouseWheelMsg:
		mm := msg.Mouse()
		m.lastMouse = fmt.Sprintf("wheel %v at %d,%d", mm.Button, mm.X, mm.Y)
		if pages[m.page] == "Todo" && len(m.todos) > 0 {
			if mm.Button == tea.MouseWheelDown {
				m.todoCur = min(len(m.todos)-1, m.todoCur+1)
			} else {
				m.todoCur = max(0, m.todoCur-1)
			}
		}
	case tea.KeyPressMsg:
		return m.key(msg)
	}
	return m, nil
}

func (m model) key(k tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	s := k.String()
	if s == "ctrl+c" {
		return m, tea.Quit
	}
	if m.typing {
		switch s {
		case "esc":
			m.typing, m.input = false, ""
		case "enter":
			if text := strings.TrimSpace(m.input); text != "" {
				if pages[m.page] == "Chat" {
					say(m.who, text)
				} else {
					m.todos = append(m.todos, todo{text: text})
					m.todoCur = len(m.todos) - 1
				}
			}
			m.input = ""
			if pages[m.page] == "Todo" {
				m.typing = false
			}
		case "backspace":
			if r := []rune(m.input); len(r) > 0 {
				m.input = string(r[:len(r)-1])
			}
		default:
			if k.Text != "" {
				m.input += k.Text
			}
		}
		return m, nil
	}
	switch s {
	case "q":
		return m, tea.Quit
	case "tab", "right", "l":
		if m.focusMenu {
			m.focusMenu = false
			if pages[m.page] == "Chat" {
				m.typing = true
			}
		}
	case "shift+tab", "left", "h", "esc":
		m.focusMenu = true
	case "up", "k":
		if m.focusMenu {
			m.page = (m.page + len(pages) - 1) % len(pages)
		} else if pages[m.page] == "Todo" {
			m.todoCur = max(0, m.todoCur-1)
		}
	case "down", "j":
		if m.focusMenu {
			m.page = (m.page + 1) % len(pages)
		} else if pages[m.page] == "Todo" {
			m.todoCur = min(len(m.todos)-1, m.todoCur+1)
		}
	case "enter":
		if m.focusMenu {
			m.focusMenu = false
			m.typing = pages[m.page] == "Chat"
		} else if pages[m.page] == "Todo" && len(m.todos) > 0 {
			m.todos[m.todoCur].done = !m.todos[m.todoCur].done
		}
	case "space":
		if pages[m.page] == "Todo" && !m.focusMenu && len(m.todos) > 0 {
			m.todos[m.todoCur].done = !m.todos[m.todoCur].done
		}
	case "a":
		if pages[m.page] == "Todo" {
			m.focusMenu, m.typing = false, true
		}
	case "d":
		if pages[m.page] == "Todo" && !m.focusMenu && len(m.todos) > 0 {
			m.todos = append(m.todos[:m.todoCur], m.todos[m.todoCur+1:]...)
			m.todoCur = max(0, min(m.todoCur, len(m.todos)-1))
		}
	default:
		if n, err := strconv.Atoi(s); err == nil && n >= 1 && n <= len(pages) {
			m.page, m.focusMenu = n-1, true
		}
	}
	return m, nil
}

// click picks menu entries and todo items; the layout math mirrors View.
func (m model) click(mm tea.Mouse) (tea.Model, tea.Cmd) {
	m.clicks++
	m.lastMouse = fmt.Sprintf("click %v at %d,%d", mm.Button, mm.X, mm.Y)
	const menuTop = 3 // header (1) + menu border (1) + menu title (1)
	if mm.X < menuWidth+2 {
		if i := mm.Y - menuTop; i >= 0 && i < len(pages) {
			m.page, m.focusMenu = i, false
			m.typing = pages[i] == "Chat"
		}
		return m, nil
	}
	m.focusMenu = false
	if pages[m.page] == "Todo" {
		if i := mm.Y - 4; i >= 0 && i < len(m.todos) { // header, border, title, blank line
			if m.todoCur == i {
				m.todos[i].done = !m.todos[i].done
			}
			m.todoCur = i
		}
	}
	return m, nil
}

// --- view ---

const menuWidth = 16

type styles struct {
	header, menu, menuItem, menuSel, menuSelDim, box, title, dim, accent, ok, warn, input, footer lipgloss.Style
}

func newStyles(dark bool) styles {
	ld := lipgloss.LightDark(dark)
	accent := ld(lipgloss.Color("#5f00af"), lipgloss.Color("#d7afff"))
	dim := ld(lipgloss.Color("#666666"), lipgloss.Color("#999999"))
	return styles{
		header:     lipgloss.NewStyle().Bold(true).Reverse(true).Padding(0, 1),
		menu:       lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Width(menuWidth + 2),
		menuItem:   lipgloss.NewStyle().Padding(0, 1).Width(menuWidth),
		menuSel:    lipgloss.NewStyle().Padding(0, 1).Width(menuWidth).Reverse(true).Bold(true),
		menuSelDim: lipgloss.NewStyle().Padding(0, 1).Width(menuWidth).Bold(true).Underline(true),
		box:        lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1),
		title:      lipgloss.NewStyle().Bold(true).Foreground(accent),
		dim:        lipgloss.NewStyle().Foreground(dim),
		accent:     lipgloss.NewStyle().Foreground(accent),
		ok:         lipgloss.NewStyle().Foreground(ld(lipgloss.Color("#005f00"), lipgloss.Color("#87ff87"))),
		warn:       lipgloss.NewStyle().Foreground(ld(lipgloss.Color("#af0000"), lipgloss.Color("#ff8787"))).Bold(true),
		input:      lipgloss.NewStyle().Border(lipgloss.NormalBorder(), true, false, false, false),
		footer:     lipgloss.NewStyle().Foreground(dim),
	}
}

func (m model) View() tea.View {
	v := tea.NewView(m.render())
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.WindowTitle = "hellobro-server · " + pages[m.page]
	return v
}

func (m model) render() string {
	if m.w == 0 {
		return "…"
	}
	st := m.st
	host, _ := os.Hostname()
	right := fmt.Sprintf("%s  %s", m.who, time.Now().Format("15:04:05"))
	left := fmt.Sprintf("bro control center · %s", host)
	gap := max(1, m.w-lipgloss.Width(left)-lipgloss.Width(right)-2)
	header := st.header.Width(m.w).Render(left + strings.Repeat(" ", gap) + right)

	var items []string
	items = append(items, st.title.Render(" menu"))
	for i, p := range pages {
		label := fmt.Sprintf("%d %s", i+1, p)
		switch {
		case i == m.page && m.focusMenu:
			items = append(items, st.menuSel.Render(label))
		case i == m.page:
			items = append(items, st.menuSelDim.Render(label))
		default:
			items = append(items, st.menuItem.Render(label))
		}
	}
	// lipgloss sizes include borders and padding; every frame is exactly
	// m.h rows, since rows left over from a taller frame aren't cleared.
	bodyH := max(5, m.h-2) // header, footer
	menu := st.menu.Height(bodyH).MaxHeight(bodyH).Render(strings.Join(items, "\n"))

	boxW := max(24, m.w-menuWidth-2)
	contentW := boxW - 4 // border, padding
	var content string
	switch pages[m.page] {
	case "Dashboard":
		content = m.dashboard(contentW)
	case "Todo":
		content = m.todoView(contentW)
	case "Chat":
		content = m.chatView(contentW, bodyH-2)
	case "Showcase":
		content = m.showcase(contentW)
	case "About":
		content = m.about(contentW)
	}
	content = lipgloss.NewStyle().MaxWidth(contentW).MaxHeight(bodyH - 2).Render(content) // clip, don't stretch the box
	box := st.box.Width(boxW).Height(bodyH).Render(content)
	body := lipgloss.JoinHorizontal(lipgloss.Top, menu, box)

	help := "↑↓/jk menu · enter/tab open · esc back · 1-5 jump · mouse works · q quit"
	switch {
	case m.typing:
		help = "typing · enter send · esc stop"
	case pages[m.page] == "Todo" && !m.focusMenu:
		help = "↑↓ move · space/enter/tap toggle · a add · d delete · esc menu"
	}
	return lipgloss.JoinVertical(lipgloss.Left, header, body, st.footer.MaxWidth(m.w).Render(" "+help))
}

func (m model) dashboard(w int) string {
	st, s := m.st, m.stats
	barW := max(10, min(50, w-26))
	lines := []string{
		st.title.Render("This PC, live"), "",
		fmt.Sprintf("%-10s %s", "uptime", (time.Duration(s.uptime) * time.Second).Round(time.Second)),
		fmt.Sprintf("%-10s %.2f %.2f %.2f", "load", s.load[0], s.load[1], s.load[2]),
		fmt.Sprintf("%-10s %s %5.1f%%", "cpu", bar(s.cpu/100, barW), s.cpu),
		fmt.Sprintf("%-10s %s %5.1f%%  %s / %s", "memory", bar(s.memUsed/max(1, s.memTotal), barW),
			100*s.memUsed/max(1, s.memTotal), human(s.memUsed), human(s.memTotal)),
		fmt.Sprintf("%-10s %s", "processes", strconv.Itoa(s.procs)), "",
		st.title.Render("cpu history") + st.dim.Render(fmt.Sprintf("  (last %ds)", len(m.cpuHist))),
		sparkline(m.cpuHist, w-2, 4), "",
		st.title.Render("who's here"),
	}
	for _, p := range people() {
		mark := "  "
		if p == m.who {
			mark = st.accent.Render("▶ ")
		}
		lines = append(lines, mark+p)
	}
	return strings.Join(lines, "\n")
}

func bar(frac float64, w int) string {
	frac = max(0, min(1, frac))
	full := frac * float64(w)
	n := int(full)
	eighths := []string{"", "▏", "▎", "▍", "▌", "▋", "▊", "▉"}
	s := strings.Repeat("█", n)
	if n < w {
		s += eighths[int((full-float64(n))*8)]
	}
	return "▕" + s + strings.Repeat(" ", max(0, w-lipgloss.Width(s))) + "▏"
}

// sparkline draws values (0-100) as h rows of block eighths, newest right.
func sparkline(vals []float64, w, h int) string {
	if len(vals) > w {
		vals = vals[len(vals)-w:]
	}
	levels := []rune(" ▁▂▃▄▅▆▇█")
	rows := make([][]rune, h)
	for r := range rows {
		rows[r] = []rune(strings.Repeat(" ", w))
	}
	off := w - len(vals)
	for i, v := range vals {
		units := int(max(0, min(100, v)) / 100 * float64(h*8))
		for r := 0; r < h; r++ { // r = 0 is the bottom row
			u := max(0, min(8, units-r*8))
			rows[h-1-r][off+i] = levels[u]
		}
	}
	out := make([]string, h)
	for i, r := range rows {
		out[i] = string(r)
	}
	return strings.Join(out, "\n")
}

func (m model) todoView(w int) string {
	st := m.st
	done := 0
	for _, t := range m.todos {
		if t.done {
			done++
		}
	}
	lines := []string{st.title.Render(fmt.Sprintf("Todo  %d/%d done", done, len(m.todos))), ""}
	for i, t := range m.todos {
		box := "[ ]"
		text := t.text
		if t.done {
			box = st.ok.Render("[✓]")
			text = lipgloss.NewStyle().Strikethrough(true).Faint(true).Render(text)
		}
		line := box + " " + text
		if i == m.todoCur && !m.focusMenu {
			line = st.accent.Render("▶ ") + line
		} else {
			line = "  " + line
		}
		lines = append(lines, line)
	}
	if m.typing {
		lines = append(lines, "", st.input.Width(w).Render("new: "+m.input+"█"))
	}
	return strings.Join(lines, "\n")
}

func (m model) chatView(w, h int) string {
	st := m.st
	hub.Lock()
	all := append([]chatLine(nil), hub.lines...)
	hub.Unlock()
	var lines []string
	for _, l := range all {
		who := st.accent.Bold(true).Render(l.who)
		if l.who == "server" {
			who = st.dim.Render("server")
		}
		lines = append(lines, st.dim.Render(l.at.Format("15:04"))+" "+who+": "+l.text)
	}
	room := max(1, h-4) // title, blank, input's rule and line
	if len(lines) > room {
		lines = lines[len(lines)-room:]
	}
	for len(lines) < room {
		lines = append(lines, "")
	}
	cursor := ""
	if m.typing {
		cursor = "█"
	}
	head := st.title.Render(fmt.Sprintf("Chat · %d here", len(people()))) + "\n"
	return head + "\n" + strings.Join(lines, "\n") + "\n" + st.input.Width(w).Render("> "+m.input+cursor)
}

func (m model) showcase(w int) string {
	st := m.st
	borders := []struct {
		name string
		b    lipgloss.Border
	}{{"rounded", lipgloss.RoundedBorder()}, {"normal", lipgloss.NormalBorder()}, {"thick", lipgloss.ThickBorder()}, {"double", lipgloss.DoubleBorder()}}
	var boxes []string
	for _, b := range borders {
		boxes = append(boxes, lipgloss.NewStyle().Border(b.b).Padding(0, 1).Render(b.name))
	}
	attrs := strings.Join([]string{
		lipgloss.NewStyle().Bold(true).Render("bold"),
		lipgloss.NewStyle().Faint(true).Render("faint"),
		lipgloss.NewStyle().Italic(true).Render("italic"),
		lipgloss.NewStyle().Underline(true).Render("underline"),
		lipgloss.NewStyle().Strikethrough(true).Render("strike"),
		lipgloss.NewStyle().Reverse(true).Render("reverse"),
	}, "  ")
	var ansi16, ramp strings.Builder
	for i := 0; i < 16; i++ {
		ansi16.WriteString(lipgloss.NewStyle().Background(lipgloss.Color(strconv.Itoa(i))).Render("  "))
	}
	for i := 232; i <= 255; i++ {
		ramp.WriteString(lipgloss.NewStyle().Background(lipgloss.Color(strconv.Itoa(i))).Render(" "))
	}
	blocks := "█▉▊▋▌▍▎▏ ▁▂▃▄▅▆▇█  ░▒▓  ▖▗▘▙▚▛▜▝▞▟  ⣿⡇⠿⢸⣀⠉"
	table := strings.Join([]string{
		"┌──────┬──────┐  ╔══════╦══════╗  ┏━━━━━━┳━━━━━━┓",
		"│ cell │ cell │  ║ cell ║ cell ║  ┃ cell ┃ cell ┃",
		"├──────┼──────┤  ╠══════╬══════╣  ┣━━━━━━╋━━━━━━┫",
		"│ cell │ cell │  ║ cell ║ cell ║  ┃ cell ┃ cell ┃",
		"└──────┴──────┘  ╚══════╩══════╝  ┗━━━━━━┻━━━━━━┛",
	}, "\n")
	return strings.Join([]string{
		st.title.Render("Borders"), lipgloss.JoinHorizontal(lipgloss.Top, boxes...), "",
		table, "",
		st.title.Render("Attributes"), attrs, "",
		st.title.Render("16 colors, then the 24-step gray ramp"), ansi16.String(), ramp.String(), "",
		st.title.Render("Blocks, shades, quadrants, braille"), blocks, "",
		st.title.Render("Mouse"), fmt.Sprintf("%d clicks · last: %s", m.clicks, orNone(m.lastMouse)),
	}, "\n")
}

func (m model) about(w int) string {
	st := m.st
	theme := "light"
	if m.dark {
		theme = "dark"
	}
	return strings.Join([]string{
		st.title.Render("About"), "",
		"An SSH server made with charmbracelet/wish,",
		"serving a Bubble Tea app to every session.", "",
		fmt.Sprintf("%-14s %s", "you", m.who),
		fmt.Sprintf("%-14s %s", "TERM", m.term),
		fmt.Sprintf("%-14s %d×%d", "window", m.w, m.h),
		fmt.Sprintf("%-14s %s (from your terminal's background)", "theme", theme),
		fmt.Sprintf("%-14s %s", "connected", time.Since(m.started).Round(time.Second)), "",
		st.warn.Render("Anyone on the network can log in: LAN use only."),
	}, "\n")
}

func orNone(s string) string {
	if s == "" {
		return "none yet"
	}
	return s
}

func human(b float64) string {
	return fmt.Sprintf("%.1f GiB", b/(1<<30))
}

// --- /proc stats ---

type stats struct {
	uptime            float64
	load              [3]float64
	cpu               float64 // percent busy since the last reading
	idle, total       uint64
	memUsed, memTotal float64
	procs             int
}

func readStats(prev stats) stats {
	s := stats{}
	if b, err := os.ReadFile("/proc/uptime"); err == nil {
		fmt.Sscan(string(b), &s.uptime)
	}
	if b, err := os.ReadFile("/proc/loadavg"); err == nil {
		fmt.Sscan(string(b), &s.load[0], &s.load[1], &s.load[2])
	}
	if b, err := os.ReadFile("/proc/stat"); err == nil {
		f := strings.Fields(strings.SplitN(string(b), "\n", 2)[0])
		for i, v := range f[1:] {
			n, _ := strconv.ParseUint(v, 10, 64)
			s.total += n
			if i == 3 || i == 4 { // idle, iowait
				s.idle += n
			}
		}
		if dt := s.total - prev.total; prev.total > 0 && dt > 0 {
			s.cpu = 100 * float64(dt-(s.idle-prev.idle)) / float64(dt)
		}
	}
	if b, err := os.ReadFile("/proc/meminfo"); err == nil {
		kv := map[string]float64{}
		for _, l := range strings.Split(string(b), "\n") {
			if f := strings.Fields(l); len(f) >= 2 {
				n, _ := strconv.ParseFloat(f[1], 64)
				kv[strings.TrimSuffix(f[0], ":")] = n * 1024
			}
		}
		s.memTotal, s.memUsed = kv["MemTotal"], kv["MemTotal"]-kv["MemAvailable"]
	}
	if d, err := os.ReadDir("/proc"); err == nil {
		for _, e := range d {
			if _, err := strconv.Atoi(e.Name()); err == nil {
				s.procs++
			}
		}
	}
	return s
}
