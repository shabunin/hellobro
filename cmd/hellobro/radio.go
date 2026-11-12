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
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"hellobro/radio"
)

const (
	panelWiFi = iota
	panelBT
)

const (
	radioPoll    = 3 * time.Second
	discoverTime = 40 * time.Second
)

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
	read     bool // ws and bs are as read, not yet zero
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
			r.ws, r.bs, r.devs, r.read = ws, bs, devs, true
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
	// Wi-Fi off cuts the chip's power, Bluetooth's too: not with it on
	// (Nickel's headphones, while it's frozen).
	bs, _ := r.bt.Status()
	r.wifi.Restore(bs.Powered)
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
				if !a.showingRadio() { // nowhere else to say so
					a.Toast(strings.TrimSuffix(busy, "…")+": "+err.Error(), 5*time.Second)
				}
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
	r := &a.rs
	wifi, bt := r.wifi, r.bt
	a.radioRun(panelBT, what, func(func(string)) error {
		if err := bt.SetPower(on); err != nil || on {
			return err
		}
		// Wi-Fi off while Bluetooth was on kept the chip powered for it
		// (Down): now neither needs it, so it goes off, driver and all.
		r.wJob.Lock() // after a Wi-Fi job going on, which may turn it on
		defer r.wJob.Unlock()
		if !wifi.Status().On {
			return wifi.SetPower(false, func(s string) { log.Printf("wifi: %s", s) })
		}
		return nil
	})
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

// drawRadioPanel asks Wi-Fi's window (i panelWiFi) or Bluetooth's for a
// frame: what it says changed.
func (a *bro) drawRadioPanel(i int) {
	a.againPicker() // the menu's tiles for them
	a.inkRedraw([]string{"wifi", "bluetooth"}[i])
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

const keyDelete = 111
