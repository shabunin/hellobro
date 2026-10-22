package main

// i3's IPC, so what's written for i3 (i3-msg, swaymsg, i3ipc scripts,
// bars) works with the window manager: a Unix socket, /tmp/hellobro-ipc/
// ipc.sock, which every Alpine jail has at /run/hellobro/ipc.sock with
// I3SOCK and SWAYSOCK set to it, and hellobro-msg in its PATH (this very
// binary, by that name; hellobro --msg too).
//
// A message is "i3-ipc", its length and its type (each 32 bits, the
// machine's byte order), then JSON. Commands, the tree, the workspaces,
// the outputs, marks, the version, the binding modes and state, the
// config (keys.conf), ticks and sync; and events for subscribers:
// workspace, window, mode, binding, tick and shutdown.
//
// What reaches the socket is a jail's: never root on the Kobo. So exec
// runs in a new Alpine window, and of hellobro's own commands only what
// can't reach out of a jail runs: no Kobo shell, no SSH, no clipboard,
// no sharing or radios, no settings (ipcHost).

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"hellobro/kobo"
	"hellobro/wm"
)

const (
	ipcMagic   = "i3-ipc"
	ipcMax     = 1 << 20 // a message's most, in bytes
	ipcJailDir = "/run/hellobro"
	ipcName    = "ipc.sock"
	ipcEvery   = 500 * time.Millisecond // titles are looked at this often, for window events
)

var ipcDir = kobo.EnvOr("HELLOBRO_IPC_DIR", "/tmp/hellobro-ipc")

// The messages' types, and the events' (with the top bit set).
const (
	ipcCommand = iota
	ipcGetWorkspaces
	ipcSubscribe
	ipcGetOutputs
	ipcGetTree
	ipcGetMarks
	ipcGetBarConfig
	ipcGetVersion
	ipcGetBindingModes
	ipcGetConfig
	ipcSendTick
	ipcSync
	ipcGetBindingState
)

const ipcEvent = 1 << 31

var ipcEvents = []string{"workspace", "output", "mode", "window", "barconfig_update", "binding", "shutdown", "tick"}

// ipcTypes are the messages' types by i3-msg's names for them.
var ipcTypes = []string{"command", "get_workspaces", "subscribe", "get_outputs", "get_tree", "get_marks",
	"get_bar_config", "get_version", "get_binding_modes", "get_config", "send_tick", "sync", "get_binding_state"}

func ipcWrite(w io.Writer, typ uint32, payload []byte) error {
	b := make([]byte, 0, len(ipcMagic)+8+len(payload))
	b = append(b, ipcMagic...)
	b = binary.NativeEndian.AppendUint32(b, uint32(len(payload)))
	b = binary.NativeEndian.AppendUint32(b, typ)
	_, err := w.Write(append(b, payload...))
	return err
}

func ipcRead(r io.Reader) (uint32, []byte, error) {
	h := make([]byte, len(ipcMagic)+8)
	if _, err := io.ReadFull(r, h); err != nil {
		return 0, nil, err
	}
	if string(h[:len(ipcMagic)]) != ipcMagic {
		return 0, nil, errors.New("not i3's IPC")
	}
	n := binary.NativeEndian.Uint32(h[len(ipcMagic):])
	typ := binary.NativeEndian.Uint32(h[len(ipcMagic)+4:])
	if n > ipcMax {
		return 0, nil, fmt.Errorf("a message of %d bytes", n)
	}
	b := make([]byte, n)
	_, err := io.ReadFull(r, b)
	return typ, b, err
}

type ipcState struct {
	ln    net.Listener
	conns map[*ipcConn]bool // the loop's
	seen  ipcSeen           // what the last events told
	at    time.Time         // the last look for them
}

type ipcConn struct {
	c      net.Conn
	out    chan []byte     // whole messages, written in order
	events map[string]bool // subscribed to
	tick   bool            // the first tick event is due
}

// ipcSeen is the tree as the events last told it.
type ipcSeen struct {
	ok    bool
	focus int64
	cur   string
	wins  map[int64]ipcWin
	ws    map[string]wm.IPCNode
	order []string // the workspaces
}

type ipcWin struct {
	title, ws, marks    string
	full, float, urgent bool
	json                wm.IPCNode
}

// ipcStart listens on the socket.
func (a *bro) ipcStart() {
	a.ipc.conns = map[*ipcConn]bool{}
	if err := os.MkdirAll(ipcDir, 0o700); err != nil {
		log.Printf("ipc: %v", err)
		return
	}
	path := filepath.Join(ipcDir, ipcName)
	os.Remove(path) // a hellobro's before
	ln, err := net.Listen("unix", path)
	if err != nil {
		log.Printf("ipc: %v", err)
		return
	}
	os.Chmod(path, 0o600)
	a.ipc.ln = ln
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go a.ipcServe(c)
		}
	}()
}

// ipcExit tells the subscribers, and goes.
func (a *bro) ipcExit() {
	a.ipcSend("shutdown", map[string]string{"change": "exit"})
	if a.ipc.ln != nil {
		a.ipc.ln.Close()
		os.Remove(filepath.Join(ipcDir, ipcName))
	}
	for c := range a.ipc.conns {
		a.ipcDrop(c) // its writer sends what's left, and closes it
	}
}

// ipcServe reads a connection's messages and has the loop answer them.
func (a *bro) ipcServe(c net.Conn) {
	ic := &ipcConn{c: c, out: make(chan []byte, 64)}
	if !a.ipcDo(func() { a.ipc.conns[ic] = true }) {
		c.Close()
		return
	}
	go func() {
		w := bufio.NewWriter(c)
		for m := range ic.out {
			w.Write(m)
			if len(ic.out) == 0 && w.Flush() != nil {
				break
			}
		}
		w.Flush()
		c.Close()
	}()
	r := bufio.NewReader(c)
	for {
		typ, b, err := ipcRead(r)
		if err != nil {
			break
		}
		if !a.ipcDo(func() {
			if a.ipc.conns[ic] {
				a.ipcPut(ic, typ, a.ipcAnswer(ic, typ, b))
			}
			if a.ipc.conns[ic] && ic.tick { // after the reply, as i3 has it
				ic.tick = false
				a.ipcPut(ic, ipcEvent|7, ipcJSON(map[string]any{"first": true, "payload": ""}))
			}
		}) {
			break
		}
	}
	a.ipcDo(func() { a.ipcDrop(ic) })
}

// ipcDo runs f on the loop and waits for it; false if the loop didn't.
func (a *bro) ipcDo(f func()) bool {
	done := make(chan struct{})
	a.Do(func() { f(); close(done) })
	select {
	case <-done:
		return true
	case <-time.After(10 * time.Second):
		return false
	}
}

// ipcPut queues a message for c; one too slow to take it is dropped, the
// loop never waits on it.
func (a *bro) ipcPut(c *ipcConn, typ uint32, payload []byte) {
	var b strings.Builder
	ipcWrite(&b, typ, payload)
	select {
	case c.out <- []byte(b.String()):
	default:
		log.Print("ipc: a client too slow, dropped")
		a.ipcDrop(c)
		c.c.Close()
	}
}

func (a *bro) ipcDrop(c *ipcConn) {
	if a.ipc.conns[c] {
		delete(a.ipc.conns, c)
		close(c.out)
	}
}

func ipcJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte(`{"success":false,"error":` + fmt.Sprintf("%q", err.Error()) + `}`)
	}
	return b
}

// ipcAnswer is the reply to a message.
func (a *bro) ipcAnswer(c *ipcConn, typ uint32, b []byte) []byte {
	t := a.wm.tree
	switch typ {
	case ipcCommand:
		rs := a.wmRunWith(string(b), ipcHost{wmHost{a}})
		if len(rs) == 0 {
			rs = []wm.Result{{ParseError: true, Error: "no command"}}
		}
		return ipcJSON(rs)
	case ipcGetWorkspaces:
		return ipcJSON(t.IPCWorkspaces())
	case ipcSubscribe:
		var names []string
		if err := json.Unmarshal(b, &names); err != nil {
			return ipcJSON(map[string]any{"success": false, "error": "a list of events, as JSON"})
		}
		if c.events == nil {
			c.events = map[string]bool{}
		}
		for _, n := range names {
			if !slices.Contains(ipcEvents, n) {
				return ipcJSON(map[string]any{"success": false, "error": "no event " + n})
			}
			c.events[n] = true
		}
		a.ipcLook() // what's now is what's told from
		c.tick = slices.Contains(names, "tick")
		return ipcJSON(map[string]bool{"success": true})
	case ipcGetOutputs:
		return ipcJSON(t.IPCOutputs())
	case ipcGetTree:
		return ipcJSON(t.IPCTree())
	case ipcGetMarks:
		return ipcJSON(append([]string{}, t.Marks()...))
	case ipcGetBarConfig:
		if len(b) == 0 {
			return []byte("[]")
		}
		return ipcJSON(map[string]any{"success": false, "error": "no bar " + string(b)})
	case ipcGetVersion:
		return ipcJSON(map[string]any{"major": 4, "minor": 23, "patch": 0, "variant": "hellobro",
			"human_readable": "4.23 (hellobro " + version + ")", "loaded_config_file_name": kobo.AppFile(keysTxt)})
	case ipcGetBindingModes:
		return ipcJSON(append([]string{"default"}, a.keyModes()...))
	case ipcGetConfig:
		conf, _ := os.ReadFile(kobo.AppFile(keysTxt))
		return ipcJSON(map[string]string{"config": string(conf)})
	case ipcSendTick:
		a.ipcSend("tick", map[string]any{"first": false, "payload": string(b)})
		return ipcJSON(map[string]bool{"success": true})
	case ipcSync:
		return ipcJSON(map[string]bool{"success": true})
	case ipcGetBindingState:
		mode := a.wm.mode
		if mode == "" {
			mode = "default"
		}
		return ipcJSON(map[string]string{"name": mode})
	}
	return ipcJSON(map[string]any{"success": false, "error": fmt.Sprintf("no message type %d", typ)})
}

// ipcSend sends an event to those subscribed to it.
func (a *bro) ipcSend(event string, v any) {
	i := slices.Index(ipcEvents, event)
	var b []byte
	for c := range a.ipc.conns {
		if c.events[event] {
			if b == nil {
				b = ipcJSON(v)
			}
			a.ipcPut(c, ipcEvent|uint32(i), b)
		}
	}
}

// ipcWanted says whether anyone is subscribed to event.
func (a *bro) ipcWanted(event string) bool {
	for c := range a.ipc.conns {
		if c.events[event] {
			return true
		}
	}
	return false
}

// ipcTick looks for what the windows' titles did now and then.
func (a *bro) ipcTick() {
	if time.Since(a.ipc.at) >= ipcEvery {
		a.ipcLook()
	}
}

// ipcLook tells the subscribers what changed in the tree since last time:
// windows new, closed, retitled, moved, floating, fullscreen, marked,
// focused; workspaces made, focused, emptied.
func (a *bro) ipcLook() {
	a.ipc.at = time.Now()
	if a.wm.tree == nil || !a.ipcWanted("window") && !a.ipcWanted("workspace") {
		a.ipc.seen.ok = false
		return
	}
	t := a.wm.tree
	now := ipcSeen{ok: true, cur: t.Current().Name, wins: map[int64]ipcWin{}, ws: map[string]wm.IPCNode{}}
	if f := t.Focused(); f != nil {
		now.focus = f.ID
	}
	for _, w := range t.Workspaces() {
		now.order = append(now.order, w.Name)
		now.ws[w.Name] = t.IPCNode(w)
	}
	for _, n := range t.Windows() {
		now.wins[n.ID] = ipcWin{title: n.Window.Title(), ws: wm.WorkspaceOf(n).Name, marks: strings.Join(n.Marks, " "),
			full: n.Fullscreen, float: floatingNode(n), urgent: n.Urgent, json: t.IPCNode(n)}
	}
	was := a.ipc.seen
	a.ipc.seen = now
	if !was.ok {
		return
	}
	win := func(change string, j wm.IPCNode) {
		a.ipcSend("window", map[string]any{"change": change, "container": j})
	}
	space := func(change string, cur, old *wm.IPCNode) {
		a.ipcSend("workspace", map[string]any{"change": change, "current": cur, "old": old})
	}
	for _, name := range now.order {
		if _, ok := was.ws[name]; !ok {
			j := now.ws[name]
			space("init", &j, nil)
		}
	}
	for _, n := range t.Windows() {
		w, o := now.wins[n.ID], was.wins[n.ID]
		_, had := was.wins[n.ID]
		switch {
		case !had:
			win("new", w.json)
			continue
		case w.title != o.title:
			win("title", w.json)
		}
		if w.ws != o.ws {
			win("move", w.json)
		}
		if w.float != o.float {
			win("floating", w.json)
		}
		if w.full != o.full {
			win("fullscreen_mode", w.json)
		}
		if w.marks != o.marks {
			win("mark", w.json)
		}
		if w.urgent != o.urgent {
			win("urgent", w.json)
		}
	}
	for id, o := range was.wins {
		if _, ok := now.wins[id]; !ok {
			win("close", o.json)
		}
	}
	if now.cur != was.cur {
		cur := now.ws[now.cur]
		var old *wm.IPCNode
		if j, ok := now.ws[was.cur]; ok {
			old = &j
		} else if j, ok := was.ws[was.cur]; ok {
			old = &j
		}
		space("focus", &cur, old)
	}
	if now.focus != was.focus {
		if w, ok := now.wins[now.focus]; ok {
			win("focus", w.json)
		}
	}
	for _, name := range was.order {
		if _, ok := now.ws[name]; !ok {
			j := was.ws[name]
			space("empty", &j, nil)
		}
	}
}

// ipcHost runs what comes over the socket: as the keys' do, but of
// hellobro's own commands only those that can't reach out of a jail.
type ipcHost struct{ wmHost }

// ipcCommands are hellobro's commands the socket may run.
var ipcCommands = map[string]bool{
	"launcher": true, "palette": true, "reload": true,
	"music.play": true, "music.next": true, "music.prev": true, "music.stop": true, "music.louder": true, "music.quieter": true,
	"statusbar": true, "font.bigger": true, "font.smaller": true, "font.reset": true,
	"light.up": true, "light.down": true, "light.toggle": true, "rotate": true,
	"screen.portrait": true, "screen.landscape": true, "screen.portrait2": true, "screen.landscape2": true,
	"doze": true, "sleep": true, "wifi.scan": true, "bt.scan": true,
}

func (h ipcHost) Command(n *wm.Node, name string, args []string) (bool, error) {
	ok := ipcCommands[name] && len(args) == 0
	if name == "open" && len(args) == 1 {
		ok = args[0] == "launcher" || args[0] == "alpine" || guiName(args[0]) != ""
	}
	if !ok {
		if name != "open" && name != "exit" && commandByID(name) == nil {
			return false, nil
		}
		return true, fmt.Errorf("%s: not over IPC", strings.TrimSpace(name+" "+strings.Join(args, " ")))
	}
	return h.wmHost.Command(n, name, args)
}

// --- hellobro-msg ---

// msgMain is hellobro-msg, as i3-msg: [-s socket] [-t type] [-q] [-m]
// [message]. It exits 1 if it couldn't, 2 if a command failed.
func msgMain(args []string) {
	typ, sock, quiet, monitor := "command", "", false, false
	var words []string
	for i := 0; i < len(args); i++ {
		switch f := args[i]; {
		case (f == "-t" || f == "--type") && i+1 < len(args):
			i++
			typ = args[i]
		case (f == "-s" || f == "--socket") && i+1 < len(args):
			i++
			sock = args[i]
		case f == "-q" || f == "--quiet":
			quiet = true
		case f == "-m" || f == "--monitor":
			monitor = true
		case f == "-r" || f == "--raw": // i3-msg's plain JSON, which is all this prints
		case f == "-h" || f == "--help":
			fmt.Println("hellobro-msg [-s socket] [-t type] [-q] [-m] [-r] [message]: i3-msg, for hellobro's window manager")
			fmt.Println("types: " + strings.Join(ipcTypes, ", "))
			return
		default:
			words = append(words, f)
		}
	}
	i := slices.Index(ipcTypes, typ)
	if i < 0 {
		fmt.Fprintf(os.Stderr, "hellobro-msg: no type %q (%s)\n", typ, strings.Join(ipcTypes, ", "))
		os.Exit(1)
	}
	c, err := net.Dial("unix", msgSocket(sock))
	if err != nil {
		fmt.Fprintf(os.Stderr, "hellobro-msg: %v\n", err)
		os.Exit(1)
	}
	defer c.Close()
	if err := ipcWrite(c, uint32(i), []byte(strings.Join(words, " "))); err != nil {
		fmt.Fprintf(os.Stderr, "hellobro-msg: %v\n", err)
		os.Exit(1)
	}
	r := bufio.NewReader(c)
	for {
		rt, b, err := ipcRead(r)
		if err != nil {
			fmt.Fprintf(os.Stderr, "hellobro-msg: %v\n", err)
			os.Exit(1)
		}
		if !quiet {
			fmt.Println(string(b))
		}
		if rt&ipcEvent != 0 {
			continue
		}
		if i == ipcCommand {
			var rs []wm.Result
			json.Unmarshal(b, &rs)
			for _, r := range rs {
				if !r.Success {
					if quiet {
						fmt.Fprintln(os.Stderr, r.Error)
					}
					os.Exit(2)
				}
			}
		}
		if !monitor || i != ipcSubscribe {
			return
		}
	}
}

// msgSocket is the socket: as asked, or I3SOCK's, SWAYSOCK's, the jail's
// or hellobro's.
func msgSocket(sock string) string {
	for _, s := range []string{sock, os.Getenv("I3SOCK"), os.Getenv("SWAYSOCK")} {
		if s != "" {
			return s
		}
	}
	if _, err := os.Stat(filepath.Join(ipcJailDir, ipcName)); err == nil {
		return filepath.Join(ipcJailDir, ipcName)
	}
	return filepath.Join(ipcDir, ipcName)
}
