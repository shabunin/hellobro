package main

// The Settings tab: Kobo's Wi-Fi and Bluetooth settings without leaving
// for Nickel, and hellobro's own (settings.go) under them. Each panel has its power, a scan, a list (networks;
// devices) and, for the row picked, what can be done with it: connect,
// disconnect, forget, pair. Passwords and pairing codes are asked for in
// the panel. When hellobro quits, the radios' power goes back to how
// Nickel left it, since Nickel wakes up thinking so.

import (
	"errors"
	"fmt"
	"image"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"hellobro/kobo"
	"hellobro/radio"
)

const (
	panelWiFi = iota
	panelBT
)

var radioRowH = 56 // sizes.go: taller for fingers

const (
	radioPoll    = 3 * time.Second
	discoverTime = 40 * time.Second
)

// radioPanelView is where one panel's parts are on screen.
type radioPanelView struct {
	p      kobo.Panel
	ctl    []image.Rectangle // power, scan, (Wi-Fi: other network, info)
	info   image.Rectangle   // two lines: the status, and what's going on
	list   image.Rectangle
	acts   image.Rectangle // the picked row's actions, or a question
	rowsOn int

	rows    []image.Rectangle // drawn rows, for taps
	buttons []radioButton     // drawn action buttons
	shown   string            // what's drawn, to skip redrawing the same
	input   image.Rectangle   // the question's text box, if it has one
	typed   string            // what it shows
}

type radioButton struct {
	r     image.Rectangle
	label string
	do    func()
}

type radioState struct {
	started    bool
	onDone     bool // radiosOn ran
	btBackDone bool // btBack ran
	wifi       radio.WiFi
	bt         radio.Bluetooth
	focus      int

	ws         radio.WiFiStatus
	nets       []radio.Network
	wSel, wTop int
	wBusy      string     // what's going on, "" when nothing
	wNote      string     // how the last thing went
	wJob       sync.Mutex // held while a Wi-Fi job runs: sleep waits for it
	wAfter     func()     // a job for when the one going on ends (waking's)
	scanned    bool

	bs         radio.BTStatus
	devs       []radio.BTDevice
	bSel, bTop int
	bBusy      string
	bNote      string
	discoverTo time.Time

	polledAt time.Time
	polling  bool
	prompt   *radioPrompt
}

// radioPrompt is a question in a panel: a password, a name, a pairing code.
type radioPrompt struct {
	panel  int
	kind   string // "password", "ssid", and the pairing ones (radio.Ask's)
	text   string // the question
	answer string
	reveal bool
	net    radio.Network
	reply  chan radioReply // pairing asks wait on it
}

type radioReply struct {
	text string
	ok   bool
}

var factsOnce sync.Once

// placeRadio puts Wi-Fi's window (i panelWiFi) or Bluetooth's at p.
func (a *bro) placeRadio(i int, p kobo.Panel) {
	pv := &a.rv[i]
	pv.p, pv.shown = p, ""
	if p.Outer.Empty() || p.Body.Dy() < 300 { // nothing in it to draw or tap
		pv.ctl, pv.info, pv.list, pv.acts, pv.rowsOn = nil, image.Rectangle{}, image.Rectangle{}, image.Rectangle{}, 1
		pv.rows, pv.buttons, pv.input = nil, nil, image.Rectangle{}
		return
	}
	b := pv.p.Body.Inset(kobo.Pad)
	n := 4
	if i == panelBT {
		n = 2
	}
	pv.ctl = pv.ctl[:0]
	bw := min(250, (b.Dx()-12*(n-1))/n)
	for j := range n {
		x := b.Min.X + j*(bw+12)
		pv.ctl = append(pv.ctl, image.Rect(x, b.Min.Y, x+bw, b.Min.Y+64))
	}
	pv.info = image.Rect(b.Min.X, b.Min.Y+76, b.Max.X, b.Min.Y+76+72)
	pv.acts = image.Rect(b.Min.X, b.Max.Y-118, b.Max.X, b.Max.Y)
	pv.list = image.Rect(b.Min.X, pv.info.Max.Y+8, b.Max.X, pv.acts.Min.Y-8)
	pv.rowsOn = max(1, pv.list.Dy()/radioRowH)
}

func (a *bro) enterRadio() {
	a.radioStart()
	a.radioPoll(!a.rs.scanned)
	a.sharePoll()
}

// showingRadio says whether Wi-Fi's or Bluetooth's window shows.
func (a *bro) showingRadio() bool { return a.guiShows("wifi") || a.guiShows("bluetooth") }

// radioPoll reads both radios' state, off the UI, and redraws what
// changed; scan also asks Wi-Fi for the networks around.
func (a *bro) radioPoll(scan bool) {
	r := &a.rs
	if r.polling && !scan {
		return
	}
	r.polling, r.polledAt = true, time.Now()
	if scan {
		r.wBusy = "looking for networks…"
		a.drawRadioPanel(panelWiFi)
	}
	wifi, bt := r.wifi, r.bt
	go func() {
		ws := wifi.Status()
		var nets []radio.Network
		var scanErr error
		if scan && ws.On {
			nets, scanErr = wifi.Scan()
		}
		bs, devs := bt.Status()
		a.Do(func() {
			r.polling, r.polledAt = false, time.Now()
			wKey, bKey := r.selected()
			r.ws, r.bs, r.devs = ws, bs, devs
			a.SetNetwork(netText(ws))
			if scan {
				r.wBusy = ""
				r.scanned = true
				if scanErr != nil {
					r.wNote = scanErr.Error()
				} else if ws.On {
					r.nets = nets
				}
			}
			if !ws.On {
				r.nets = nil
			}
			for i := range r.nets { // the joined one, as of now
				r.nets[i].Current = ws.State == "COMPLETED" && r.nets[i].SSID == ws.SSID
			}
			r.keepSelected(wKey, bKey)
			a.drawRadio()
		})
	}()
}

// selected is what the rows picked are, which outlives the lists' order.
func (r *radioState) selected() (string, string) {
	var w, b string
	if r.wSel >= 0 && r.wSel < len(r.nets) {
		w = r.nets[r.wSel].SSID
	}
	if r.bSel >= 0 && r.bSel < len(r.devs) {
		b = string(r.devs[r.bSel].Path)
	}
	return w, b
}

// keepSelected picks the same network and device again in the new lists.
func (r *radioState) keepSelected(w, b string) {
	for i, n := range r.nets {
		if n.SSID == w {
			r.wSel = i
		}
	}
	for i, d := range r.devs {
		if string(d.Path) == b {
			r.bSel = i
		}
	}
}

func (a *bro) radioTick() {
	r := &a.rs
	a.radiosOn()
	a.btBack()
	if !r.started {
		return
	}
	if r.wAfter != nil && r.wBusy == "" {
		do := r.wAfter
		r.wAfter = nil
		do()
	}
	if !r.discoverTo.IsZero() && time.Now().After(r.discoverTo) {
		r.discoverTo = time.Time{}
		go r.bt.Discover(false)
	}
	every := radioPoll
	if !r.discoverTo.IsZero() {
		every = 1500 * time.Millisecond // devices turn up
	}
	if a.showingRadio() && !r.polling && time.Since(r.polledAt) > every {
		a.radioPoll(false)
	}
}

func (a *bro) radioExit() {
	r := &a.rs
	if nickelClosed && !a.Restarting() {
		a.radioStart() // whether or not we used them, they go off
	}
	if !r.started {
		return
	}
	if p := r.prompt; p != nil && p.reply != nil {
		p.reply <- radioReply{}
	}
	if nickelClosed && !a.Restarting() {
		// Nickel starts again after us, as it boots: it wants the radios
		// off, and turns them on as its settings say. Bluetooth first:
		// Wi-Fi off then powers the chip they share down too.
		log.Print("radio: off for Nickel, which starts again")
		if err := r.bt.SetPower(false); err != nil {
			log.Printf("bluetooth: %v", err)
		}
		if err := r.wifi.SetPower(false, func(s string) { log.Printf("wifi: %s", s) }); err != nil {
			log.Printf("wifi: %v", err)
		}
		r.bt.Keep()
		r.wifi.Keep()
	}
	r.bt.Restore()
	r.wifi.Restore()
}

// nickelClosed says launch.sh closed Nickel for our run (the nickel
// setting), and starts it again after.
var nickelClosed = os.Getenv("HELLOBRO_NICKEL") == "closed"

// nickelBT says Bluetooth was on as Nickel went. The chip's link
// (rtk_hciattach) was Nickel's child and may have gone with it.
var nickelBT = nickelClosed && os.Getenv("HELLOBRO_BT_WAS") == "1"

var startedAt = time.Now()

// btBack brings Bluetooth back, once, a few seconds in, if Nickel's going
// took it.
func (a *bro) btBack() {
	r := &a.rs
	if !nickelBT || r.btBackDone || time.Since(startedAt) < 3*time.Second {
		return
	}
	r.btBackDone = true
	a.radioStart()
	bt := r.bt
	if bs, _ := bt.Status(); bs.Powered {
		return
	}
	a.radioRun(panelBT, "Bluetooth went with Nickel: turning it on again…", func(func(string)) error {
		return bt.SetPower(true)
	})
}

// --- doing things ---

// radioRun runs do off the UI with busy showing in the panel, then notes
// how it went and looks again.
func (a *bro) radioRun(panel int, busy string, do func(progress func(string)) error) {
	r := &a.rs
	b, note := &r.wBusy, &r.wNote
	if panel == panelBT {
		b, note = &r.bBusy, &r.bNote
	}
	if *b != "" {
		return // one thing at a time
	}
	*b, *note = busy, ""
	a.drawRadioPanel(panel)
	log.Printf("radio: %s", busy)
	go func() {
		if panel == panelWiFi {
			r.wJob.Lock()
		}
		err := do(func(s string) {
			a.Do(func() {
				*b = s
				a.drawRadioPanel(panel)
			})
		})
		if panel == panelWiFi {
			r.wJob.Unlock()
		}
		a.Do(func() {
			*b = ""
			if err != nil {
				*note = err.Error()
				log.Printf("radio: %s: %v", busy, err)
			} else {
				*note = "done: " + strings.TrimSuffix(busy, "…")
			}
			a.radioPoll(panel == panelWiFi)
			a.drawRadioPanel(panel)
		})
	}()
}

// radiosOn turns Wi-Fi and Bluetooth on, once, when the setting asks
// for it, and has them left on when we quit: on is then how they're put
// back. A restart into a new build finds them on and leaves them so.
func (a *bro) radiosOn() {
	r := &a.rs
	if r.onDone || a.setting("radios") != "on" {
		return
	}
	r.onDone = true
	if a.mem.Airplane != nil {
		return // airplane mode: they stay off till it's left
	}
	a.radioStart()
	wifi, bt := r.wifi, r.bt
	a.radioRun(panelWiFi, "turning Wi-Fi and Bluetooth on…", func(p func(string)) error {
		var errs []error
		if bs, _ := bt.Status(); !bs.Powered {
			if err := bt.SetPower(true); err != nil {
				errs = append(errs, fmt.Errorf("bluetooth: %w", err))
			}
		}
		if err := wifi.SetPower(true, p); err != nil {
			errs = append(errs, err)
		}
		bt.Keep()
		wifi.Keep()
		return errors.Join(errs...)
	})
}

// radiosWere is which radios were on before airplane mode.
type radiosWere struct {
	WiFi      bool `json:"wifi,omitempty"`
	Bluetooth bool `json:"bluetooth,omitempty"`
}

// airplane turns Wi-Fi and Bluetooth off, keeping which were on; again,
// it turns those back on. Bluetooth goes first: Wi-Fi's chip is its too,
// and only with Bluetooth off does it lose its power.
func (a *bro) airplane() {
	a.radioStart()
	r := &a.rs
	if r.wBusy != "" { // which were on isn't kept till it's done
		a.Toast("the radios are busy: a moment", 3*time.Second)
		return
	}
	wifi, bt := r.wifi, r.bt
	if was := a.mem.Airplane; was != nil {
		a.mem.Airplane = nil
		a.mem.save()
		a.Toast("airplane mode off", 3*time.Second)
		a.radioRun(panelWiFi, "leaving airplane mode…", func(p func(string)) error {
			var errs []error
			if was.Bluetooth {
				if err := bt.SetPower(true); err != nil {
					errs = append(errs, fmt.Errorf("bluetooth: %w", err))
				}
			}
			if was.WiFi {
				if err := wifi.SetPower(true, p); err != nil {
					errs = append(errs, err)
				}
			}
			bt.Keep()
			wifi.Keep()
			return errors.Join(errs...)
		})
		return
	}
	a.mem.Airplane = &radiosWere{}
	a.mem.save()
	a.Toast("airplane mode: Wi-Fi and Bluetooth off", 3*time.Second)
	a.radioRun(panelWiFi, "airplane mode: turning the radios off…", func(p func(string)) error {
		ws := wifi.Status()
		bs, _ := bt.Status()
		a.Do(func() {
			if a.mem.Airplane != nil {
				a.mem.Airplane.WiFi, a.mem.Airplane.Bluetooth = ws.On, bs.Powered
				a.mem.save()
			}
		})
		var errs []error
		if bs.Powered {
			if err := bt.SetPower(false); err != nil {
				errs = append(errs, fmt.Errorf("bluetooth: %w", err))
			}
		}
		if err := wifi.SetPower(false, p); err != nil {
			errs = append(errs, err)
		}
		bt.Keep() // and so when we quit: Nickel finds them off
		wifi.Keep()
		return errors.Join(errs...)
	})
}

// airplaneOver forgets airplane mode: a radio's been turned on by hand.
func (a *bro) airplaneOver() {
	if a.mem.Airplane != nil {
		a.mem.Airplane = nil
		a.mem.save()
	}
}

// airplaneNote is airplane mode's, for the menu.
func (a *bro) airplaneNote() string {
	if a.mem.Airplane != nil {
		return "on: tap to leave"
	}
	return "Wi-Fi, Bluetooth off"
}

func (a *bro) wifiPower() {
	on := !a.rs.ws.On
	if on {
		a.airplaneOver()
	}
	what := map[bool]string{true: "turning Wi-Fi on…", false: "turning Wi-Fi off…"}[on]
	wifi, bt := a.rs.wifi, a.rs.bt
	a.radioRun(panelWiFi, what, func(p func(string)) error {
		if !on {
			return wifiOff(wifi, bt, p)
		}
		return wifi.SetPower(true, p)
	})
}

// wifiOff turns Wi-Fi off; with Bluetooth on, the chip keeps its power,
// which on the Elipsa is Bluetooth's too. Off the loop: it blocks.
func wifiOff(wifi radio.WiFi, bt radio.Bluetooth, progress func(string)) error {
	if bs, _ := bt.Status(); bs.Powered {
		return wifi.Down(progress)
	}
	return wifi.SetPower(false, progress)
}

func (a *bro) btPower() {
	on := !a.rs.bs.Powered
	if on {
		a.airplaneOver()
	}
	what := map[bool]string{true: "turning Bluetooth on…", false: "turning Bluetooth off…"}[on]
	a.radioRun(panelBT, what, func(func(string)) error { return a.rs.bt.SetPower(on) })
}

func (a *bro) btScan() {
	r := &a.rs
	if !r.discoverTo.IsZero() {
		r.discoverTo = time.Time{}
		a.radioRun(panelBT, "stopping the scan…", func(func(string)) error { return r.bt.Discover(false) })
		return
	}
	r.discoverTo = time.Now().Add(discoverTime)
	a.radioRun(panelBT, "starting a scan…", func(func(string)) error { return r.bt.Discover(true) })
}

// wifiGo does the obvious thing with network i: join it, or leave it.
func (a *bro) wifiGo(i int) {
	r := &a.rs
	if i < 0 || i >= len(r.nets) {
		return
	}
	n := r.nets[i]
	switch {
	case n.Current:
		a.radioRun(panelWiFi, "disconnecting…", func(func(string)) error { return r.wifi.Disconnect() })
	case n.Known() || n.Security == "":
		a.wifiConnect(n, "")
	case n.Security == "EAP":
		r.wNote = "enterprise (EAP) networks aren't supported here"
		a.drawRadioPanel(panelWiFi)
	default:
		a.radioAsk(&radioPrompt{panel: panelWiFi, kind: "password", net: n, text: "Password for " + n.SSID + " (" + n.Security + "):"})
	}
}

func (a *bro) wifiConnect(n radio.Network, password string) {
	a.radioRun(panelWiFi, "joining "+n.SSID+"…", func(p func(string)) error { return a.rs.wifi.Connect(n, password, p) })
}

func (a *bro) wifiForget(i int) {
	r := &a.rs
	if i < 0 || i >= len(r.nets) || !r.nets[i].Known() {
		return
	}
	n := r.nets[i]
	a.radioRun(panelWiFi, "forgetting "+n.SSID+"…", func(func(string)) error { return r.wifi.Forget(n) })
}

// btGo does the obvious thing with device i: pair, connect, disconnect.
func (a *bro) btGo(i int) {
	r := &a.rs
	if i < 0 || i >= len(r.devs) {
		return
	}
	d := r.devs[i]
	name := btName(d)
	switch {
	case d.Connected:
		a.radioRun(panelBT, "disconnecting "+name+"…", func(func(string)) error { return r.bt.Disconnect(d) })
	case d.Paired:
		a.radioRun(panelBT, "connecting "+name+"…", func(func(string)) error { return r.bt.Connect(d) })
	default:
		a.radioRun(panelBT, "pairing "+name+"…", func(func(string)) error { return r.bt.Pair(d) })
	}
}

func (a *bro) btForget(i int) {
	r := &a.rs
	if i < 0 || i >= len(r.devs) || !r.devs[i].Paired && r.devs[i].Connected {
		return
	}
	d := r.devs[i]
	a.radioRun(panelBT, "forgetting "+btName(d)+"…", func(func(string)) error { return r.bt.Forget(d) })
}

func btName(d radio.BTDevice) string {
	if d.Name != "" {
		return d.Name
	}
	return d.Addr
}

// --- questions ---

func (a *bro) radioAsk(p *radioPrompt) {
	if old := a.rs.prompt; old != nil && old.reply != nil {
		old.reply <- radioReply{}
	}
	a.rs.prompt = p
	a.rs.focus = p.panel
	a.openGUI([]string{"wifi", "bluetooth"}[p.panel]) // where it's asked
	a.drawRadio()
}

// radioAnswer ends the question: ok with the text typed, or canceled.
func (a *bro) radioAnswer(ok bool) {
	p := a.rs.prompt
	if p == nil {
		return
	}
	a.rs.prompt = nil
	switch {
	case p.reply != nil:
		p.reply <- radioReply{p.answer, ok}
	case !ok:
	case p.kind == "password":
		a.wifiConnect(p.net, p.answer)
	case p.kind == "ssid" && strings.TrimSpace(p.answer) != "":
		n := radio.Network{SSID: strings.TrimSpace(p.answer), ID: -1, Hidden: true, Security: "WPA2"}
		a.radioAsk(&radioPrompt{panel: panelWiFi, kind: "password", net: n, text: "Password for " + n.SSID + " (empty: an open network):"})
		return
	}
	a.drawRadio()
}

// radioAgent puts BlueZ's pairing questions in the Bluetooth panel.
type radioAgent struct{ a *bro }

func (g radioAgent) Ask(q radio.Ask) (string, bool) {
	a := g.a
	var text string
	switch q.Kind {
	case "display":
		text = fmt.Sprintf("Type %s on %s, then Enter", q.Passkey, q.Device)
		if q.Entered > 0 {
			text += fmt.Sprintf(" (%d typed)", q.Entered)
		}
		a.Do(func() {
			if p := a.rs.prompt; p != nil && p.kind == "display" {
				p.text = text
				a.drawRadioPanel(panelBT)
				return
			}
			a.radioAsk(&radioPrompt{panel: panelBT, kind: "display", text: text})
		})
		return "", true
	case "confirm":
		text = fmt.Sprintf("Does %s show %s? Then pair.", q.Device, q.Passkey)
	case "authorize":
		text = fmt.Sprintf("Let %s pair?", q.Device)
	case "pin":
		text = fmt.Sprintf("PIN for %s (often 0000):", q.Device)
	case "passkey":
		text = fmt.Sprintf("The number %s shows:", q.Device)
	}
	reply := make(chan radioReply, 1)
	a.Do(func() { a.radioAsk(&radioPrompt{panel: panelBT, kind: q.Kind, text: text, reply: reply}) })
	select {
	case r := <-reply:
		return r.text, r.ok
	case <-time.After(60 * time.Second):
		a.Do(func() {
			if a.rs.prompt != nil && a.rs.prompt.reply == reply {
				a.rs.prompt = nil
				a.drawRadio()
			}
		})
		return "", false
	}
}

func (g radioAgent) Cancel() {
	a := g.a
	a.Do(func() {
		if p := a.rs.prompt; p != nil && p.panel == panelBT {
			if p.reply != nil {
				p.reply <- radioReply{}
			}
			a.rs.prompt = nil
			a.drawRadio()
		}
	})
}

// --- drawing ---

func (a *bro) drawRadio() {
	a.drawRadioPanel(panelWiFi)
	a.drawRadioPanel(panelBT)
	a.drawShare()
	a.drawSettings()
}

func (a *bro) drawRadioPanel(i int) {
	win := []string{"wifi", "bluetooth"}[i]
	if !a.guiShows(win) || a.rv[i].list.Empty() {
		return
	}
	r, pv, c := &a.rs, &a.rv[i], a.C
	type row struct {
		mark, name, detail string
		bars               int
	}
	var (
		status, line1, line2 string
		ctl                  []string
		rows                 []row
		sel, top             *int
		acts                 []radioButton
	)
	if i == panelWiFi {
		ws := r.ws
		power := "turn on"
		if ws.On {
			power = "turn off"
		}
		ctl = []string{power, "scan", "other network…", "info"}
		switch {
		case !ws.On:
			status, line1 = "off", "Wi-Fi is off."
		case ws.State == "COMPLETED":
			status = ws.SSID
			line1 = fmt.Sprintf("%s · %d dBm · %s", ws.SSID, ws.Signal, band(ws.Freq))
			if ws.IP != "" {
				line1 += " · " + ws.IP
			} else {
				line1 += " · no address yet"
			}
		default:
			status, line1 = "not connected", "not connected ("+strings.ToLower(ws.State)+")"
		}
		line2 = r.wNote
		if r.wBusy != "" {
			line2 = r.wBusy
		}
		for _, n := range r.nets {
			mark := ""
			switch {
			case n.Current:
				mark = "●"
			case n.Known():
				mark = "○"
			}
			detail := n.Security
			if detail == "" {
				detail = "open"
			}
			if n.Signal == 0 {
				detail = "saved, not around"
			} else if n.Freq > 0 {
				detail += " · " + band(n.Freq)
			}
			rows = append(rows, row{mark, n.SSID, detail, n.Bars()})
		}
		sel, top = &r.wSel, &r.wTop
		if *sel >= 0 && *sel < len(r.nets) {
			n, s := r.nets[*sel], *sel
			if n.Current {
				acts = append(acts, radioButton{label: "disconnect", do: func() { a.wifiGo(s) }})
			} else {
				acts = append(acts, radioButton{label: "connect", do: func() { a.wifiGo(s) }})
			}
			if n.Known() {
				acts = append(acts, radioButton{label: "forget", do: func() { a.wifiForget(s) }})
			}
		}
	} else {
		bs := r.bs
		power := "turn on"
		if bs.Powered {
			power = "turn off"
		}
		scan := "scan"
		if !r.discoverTo.IsZero() {
			scan = "stop scan"
		}
		ctl = []string{power, scan}
		switch {
		case !bs.Running:
			status, line1 = "off", bs.Err
		case !bs.Powered && bs.Addr == "": // no adapter: the chip itself is off
			status, line1 = "off", "Bluetooth is off. Turning it on takes a few seconds."
		case !bs.Powered:
			status, line1 = "off", fmt.Sprintf("Bluetooth is off (%s, %s).", bs.Name, bs.Addr)
		default:
			n := 0
			for _, d := range r.devs {
				if d.Connected {
					n++
				}
			}
			status = fmt.Sprintf("%d connected", n)
			line1 = fmt.Sprintf("%s (%s)", bs.Name, bs.Addr)
			if !r.discoverTo.IsZero() {
				line1 += fmt.Sprintf(" · scanning, %.0f s more", time.Until(r.discoverTo).Seconds())
				status = "scanning"
			}
		}
		line2 = r.bNote
		if r.bBusy != "" {
			line2 = r.bBusy
		}
		for _, d := range r.devs {
			mark := ""
			switch {
			case d.Connected:
				mark = "●"
			case d.Paired:
				mark = "○"
			}
			var parts []string
			if k := d.Kind(); k != "" {
				parts = append(parts, k)
			}
			switch {
			case d.Connected:
				parts = append(parts, "connected")
			case d.Paired:
				parts = append(parts, "paired")
			default:
				parts = append(parts, "new")
			}
			bars := 0
			if d.RSSI != 0 {
				bars = radio.Network{Signal: int(d.RSSI)}.Bars()
			}
			name := d.Name
			if name == "" {
				name = d.Addr
			}
			rows = append(rows, row{mark, name, strings.Join(parts, " · "), bars})
		}
		sel, top = &r.bSel, &r.bTop
		if *sel >= 0 && *sel < len(r.devs) {
			d, s := r.devs[*sel], *sel
			switch {
			case d.Connected:
				acts = append(acts, radioButton{label: "disconnect", do: func() { a.btGo(s) }})
			case d.Paired:
				acts = append(acts, radioButton{label: "connect", do: func() { a.btGo(s) }})
			default:
				acts = append(acts, radioButton{label: "pair", do: func() { a.btGo(s) }})
			}
			if d.Paired {
				acts = append(acts, radioButton{label: "forget", do: func() { a.btForget(s) }})
			}
		}
	}
	*sel = max(0, min(*sel, len(rows)-1))
	if *sel < *top {
		*top = *sel
	}
	if *sel >= *top+pv.rowsOn {
		*top = *sel - pv.rowsOn + 1
	}
	*top = max(0, min(*top, len(rows)-pv.rowsOn))
	focused := a.guiFocused(win)
	prompt := r.prompt
	if prompt != nil && prompt.panel != i {
		prompt = nil
	}

	// Draw only what changed: nothing, or just the text typed, or all.
	key := fmt.Sprint(status, line1, line2, ctl, rows, *sel, *top, focused, len(acts))
	if prompt != nil {
		key += fmt.Sprint(prompt.text, prompt.reveal)
	}
	if key == pv.shown {
		if prompt != nil && !pv.input.Empty() {
			a.drawRadioInput(pv, prompt)
		}
		return
	}
	pv.shown, pv.input, pv.typed = key, image.Rectangle{}, "\x00"

	p := a.winPanel(pv.p, focused)
	a.DrawPanel(p, status)
	for j, label := range ctl {
		b := pv.ctl[j]
		c.Fill(b, kobo.White)
		c.Frame(b, 2, kobo.Black)
		c.Text(a.F.Small, b, label, kobo.Black, kobo.AlignCenter)
	}
	c.Text(a.F.Small, image.Rect(pv.info.Min.X, pv.info.Min.Y, pv.info.Max.X, pv.info.Min.Y+36), line1, kobo.Black, kobo.AlignLeft)
	c.Text(a.F.Small, image.Rect(pv.info.Min.X, pv.info.Min.Y+36, pv.info.Max.X, pv.info.Max.Y), line2, kobo.Dark, kobo.AlignLeft)

	pv.rows = pv.rows[:0]
	c.Fill(image.Rect(pv.list.Min.X, pv.list.Min.Y-4, pv.list.Max.X, pv.list.Min.Y-2), kobo.Light)
	if len(rows) == 0 {
		empty := "No networks: scan, or turn Wi-Fi on."
		if i == panelBT {
			empty = "No devices yet: scan, with the device in pairing mode."
		}
		c.Text(a.F.Small, image.Rect(pv.list.Min.X, pv.list.Min.Y, pv.list.Max.X, pv.list.Min.Y+radioRowH), empty, kobo.Dark, kobo.AlignLeft)
	}
	for j := *top; j < min(len(rows), *top+pv.rowsOn); j++ {
		rw := rows[j]
		rr := image.Rect(pv.list.Min.X, pv.list.Min.Y+(j-*top)*radioRowH, pv.list.Max.X-14, pv.list.Min.Y+(j-*top+1)*radioRowH)
		pv.rows = append(pv.rows, rr)
		fg, dim := uint8(kobo.Black), uint8(kobo.Dark)
		if j == *sel {
			if focused {
				c.Fill(rr, kobo.Black)
				fg, dim = kobo.White, kobo.Light
			} else {
				c.Frame(rr, 2, kobo.Black)
			}
		}
		t := rr.Inset(4)
		c.Text(a.F.Body, image.Rect(t.Min.X+8, t.Min.Y, t.Min.X+40, t.Max.Y), rw.mark, fg, kobo.AlignLeft)
		t.Min.X += 44
		a.signalBars(image.Rect(t.Max.X-44, t.Min.Y+8, t.Max.X-4, t.Max.Y-8), rw.bars, fg, dim)
		t.Max.X -= 60
		dw := kobo.TextWidth(a.F.Small, rw.detail)
		c.Text(a.F.Small, image.Rect(t.Max.X-dw, t.Min.Y, t.Max.X, t.Max.Y), rw.detail, dim, kobo.AlignRight)
		t.Max.X -= dw + 20
		c.Text(a.F.Body, t, rw.name, fg, kobo.AlignLeft)
	}
	if n := len(rows); n > pv.rowsOn {
		tr := image.Rect(pv.list.Max.X-6, pv.list.Min.Y, pv.list.Max.X, pv.list.Min.Y+pv.rowsOn*radioRowH)
		c.Fill(tr, kobo.Light)
		h := max(24, tr.Dy()*pv.rowsOn/n)
		y := tr.Min.Y + (tr.Dy()-h)**top/max(1, n-pv.rowsOn)
		c.Fill(image.Rect(tr.Min.X, y, tr.Max.X, y+h), kobo.Dark)
	}

	// The picked row's actions, or the question.
	ar := pv.acts
	c.Fill(image.Rect(ar.Min.X, ar.Min.Y, ar.Max.X, ar.Min.Y+2), kobo.Light)
	textR := image.Rect(ar.Min.X, ar.Min.Y+6, ar.Max.X, ar.Min.Y+44)
	btnY := image.Rect(ar.Min.X, ar.Max.Y-66, ar.Max.X, ar.Max.Y-2)
	pv.buttons = pv.buttons[:0]
	if prompt != nil {
		c.Text(a.F.Small, textR, prompt.text, kobo.Black, kobo.AlignLeft)
		var labels []string
		switch prompt.kind {
		case "password", "ssid", "pin", "passkey":
			labels = []string{"ok", "cancel"}
			if prompt.kind == "password" {
				labels = append(labels, map[bool]string{true: "hide", false: "show"}[prompt.reveal])
			}
		case "confirm", "authorize":
			labels = []string{"pair", "no"}
		case "display":
			labels = []string{"cancel"}
		}
		x := btnY.Max.X
		for k := len(labels) - 1; k >= 0; k-- {
			w := kobo.TextWidth(a.F.Small, labels[k]) + 56
			b := image.Rect(x-w, btnY.Min.Y, x, btnY.Max.Y)
			x = b.Min.X - 12
			label := labels[k]
			pv.buttons = append(pv.buttons, radioButton{r: b, label: label, do: func() { a.radioPromptButton(label) }})
			c.Fill(b, kobo.White)
			c.Frame(b, 2, kobo.Black)
			c.Text(a.F.Small, b, label, kobo.Black, kobo.AlignCenter)
		}
		if prompt.kind != "confirm" && prompt.kind != "authorize" && prompt.kind != "display" {
			pv.input = image.Rect(btnY.Min.X, btnY.Min.Y, x-12, btnY.Max.Y)
			c.Frame(pv.input, 2, kobo.Black)
			a.drawRadioInput(pv, prompt)
		}
	} else {
		hint := "Tap a row, or ↑ ↓ and Enter; f: forget; s: scan; o: on/off."
		if i == panelWiFi {
			hint = strings.TrimSuffix(hint, ".") + "; i: info."
		}
		if len(acts) > 0 {
			hint = rows[*sel].name
		}
		c.Text(a.F.Small, textR, hint, kobo.Dark, kobo.AlignLeft)
		x := btnY.Min.X
		for _, b := range acts {
			w := max(200, kobo.TextWidth(a.F.Small, b.label)+56)
			b.r = image.Rect(x, btnY.Min.Y, x+w, btnY.Max.Y)
			x += w + 12
			pv.buttons = append(pv.buttons, b)
			c.Fill(b.r, kobo.White)
			c.Frame(b.r, 2, kobo.Black)
			c.Text(a.F.Small, b.r, b.label, kobo.Black, kobo.AlignCenter)
		}
	}
	a.Mark(p.Outer, true)
}

// drawRadioInput draws what's typed in the question's box, if it changed.
func (a *bro) drawRadioInput(pv *radioPanelView, p *radioPrompt) {
	shown := p.answer
	if p.kind == "password" && !p.reveal {
		shown = strings.Repeat("•", len([]rune(shown)))
	}
	if shown == pv.typed {
		return
	}
	pv.typed = shown
	c, in := a.C, pv.input.Inset(2)
	c.Fill(in, kobo.White)
	tr := in.Inset(4)
	tr.Min.X += 8
	c.Text(a.F.Body, tr, shown, kobo.Black, kobo.AlignLeft)
	cx := min(tr.Max.X-3, tr.Min.X+kobo.TextWidth(a.F.Body, shown)+3)
	c.Fill(image.Rect(cx, in.Min.Y+10, cx+3, in.Max.Y-10), kobo.Black)
	a.Mark(in, false) // black on white: the fast waveform will do
}

// signalBars draws n of 4 bars rising left to right.
func (a *bro) signalBars(r image.Rectangle, n int, on, off uint8) {
	if n <= 0 {
		return
	}
	w := r.Dx() / 4
	for k := range 4 {
		h := r.Dy() * (k + 1) / 4
		b := image.Rect(r.Min.X+k*w+2, r.Max.Y-h, r.Min.X+(k+1)*w-2, r.Max.Y)
		if k < n {
			a.C.Fill(b, on)
		} else {
			a.C.Frame(b, 1, off)
		}
	}
}

func band(mhz int) string {
	switch {
	case mhz >= 5900:
		return "6 GHz"
	case mhz >= 4900:
		return "5 GHz"
	case mhz > 0:
		return "2.4 GHz"
	}
	return ""
}

// --- input ---

func (a *bro) radioPromptButton(label string) {
	p := a.rs.prompt
	if p == nil {
		return
	}
	switch label {
	case "show", "hide":
		p.reveal = !p.reveal
		a.drawRadioPanel(p.panel)
	case "ok", "pair":
		a.radioAnswer(true)
	default:
		a.radioAnswer(false)
	}
}

func (a *bro) radioTap(pt image.Point) {
	r := &a.rs
	if pt.In(a.sv.p.Outer) {
		a.settingsTap(pt)
		return
	}
	if pt.In(a.shp.p.Outer) {
		a.shareTap(pt)
		return
	}
	for i := range a.rv {
		pv := &a.rv[i]
		if !pt.In(pv.p.Outer) {
			continue
		}
		for _, b := range pv.buttons {
			if kobo.Hit(pt, b.r) {
				b.do()
				return
			}
		}
		if r.prompt != nil {
			return // answer it first
		}
		for j, c := range pv.ctl {
			if !kobo.Hit(pt, c) {
				continue
			}
			switch {
			case i == panelWiFi && j == 0:
				a.wifiPower()
			case i == panelWiFi && j == 1:
				a.radioPoll(true)
			case i == panelWiFi && j == 2:
				a.radioAsk(&radioPrompt{panel: panelWiFi, kind: "ssid", text: "The network's name (SSID):"})
			case i == panelWiFi && j == 3:
				a.wifiInfo()
			case i == panelBT && j == 0:
				a.btPower()
			case i == panelBT && j == 1:
				a.btScan()
			}
			return
		}
		for k, rr := range pv.rows {
			if !pt.In(rr) {
				continue
			}
			sel := &r.wSel
			top := r.wTop
			if i == panelBT {
				sel, top = &r.bSel, r.bTop
			}
			if *sel == top+k { // a second tap: do it
				if i == panelWiFi {
					a.wifiGo(*sel)
				} else {
					a.btGo(*sel)
				}
				return
			}
			*sel = top + k
			a.drawRadioPanel(i)
			return
		}
		return
	}
}

// radioKey takes a key on the tab; false leaves it to the app.
func (a *bro) radioKey(code uint16, value int32) bool {
	if value == 0 {
		return true
	}
	r, m := &a.rs, &a.tv.mods
	if p := r.prompt; p != nil {
		ch, printable := kobo.KeyChar(code, m.shift, m.caps)
		switch {
		case code == kobo.KeyEsc:
			a.radioAnswer(false)
		case code == kobo.KeyEnter:
			a.radioAnswer(p.kind != "display")
		case p.kind == "confirm" || p.kind == "authorize":
			if ch == "y" {
				a.radioAnswer(true)
			} else if ch == "n" {
				a.radioAnswer(false)
			}
		case p.kind == "display":
		case code == kobo.KeyBackspace:
			if rs := []rune(p.answer); len(rs) > 0 {
				p.answer = string(rs[:len(rs)-1])
				a.drawRadioPanel(p.panel)
			}
		case printable && !m.ctrl:
			p.answer += ch
			a.drawRadioPanel(p.panel)
		}
		return true
	}
	if r.focus == panelSet {
		if code == kobo.KeyEsc {
			return false
		}
		a.settingsKey(code)
		return true
	}
	if r.focus == panelShare {
		return a.shareKey(code)
	}
	sel, n := &r.wSel, len(r.nets)
	if r.focus == panelBT {
		sel, n = &r.bSel, len(r.devs)
	}
	move := func(d int) {
		*sel = max(0, min(*sel+d, n-1))
		a.drawRadioPanel(r.focus)
	}
	page := a.rv[r.focus].rowsOn
	ch, _ := kobo.KeyChar(code, false, false)
	switch {
	case code == kobo.KeyEsc:
		return false
	case code == keyUp:
		move(-1)
	case code == keyDown:
		move(1)
	case code == keyPgUp:
		move(-page)
	case code == keyPgDn:
		move(page)
	case code == keyHome:
		move(-n)
	case code == keyEnd:
		move(n)
	case code == kobo.KeyEnter:
		if r.focus == panelWiFi {
			a.wifiGo(*sel)
		} else {
			a.btGo(*sel)
		}
	case ch == "f" || code == keyDelete:
		if r.focus == panelWiFi {
			a.wifiForget(*sel)
		} else {
			a.btForget(*sel)
		}
	case ch == "s":
		if r.focus == panelWiFi {
			a.radioPoll(true)
		} else {
			a.btScan()
		}
	case ch == "o":
		if r.focus == panelWiFi {
			a.wifiPower()
		} else {
			a.btPower()
		}
	case ch == "n" && r.focus == panelWiFi:
		a.radioAsk(&radioPrompt{panel: panelWiFi, kind: "ssid", text: "The network's name (SSID):"})
	case ch == "i" && r.focus == panelWiFi:
		a.wifiInfo()
	}
	return true
}

const keyDelete = 111
