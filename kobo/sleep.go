package kobo

// Sleep: the Kobo suspended to RAM, with a sleep screen on the panel, the
// way Nickel and KOReader do it on these devices (state-extended, then
// "mem"), and the app back as it was on waking. Nickel holds the power
// button, so going to sleep is ours to ask for (the header's button,
// Ctrl+Alt+S, Sleep); the button still wakes the device up. A Dozer may
// doze instead (doze.go), or go on to sleep from a doze.

import (
	"fmt"
	"image"
	"log"
	"os"
	"strings"
	"syscall"
	"time"
)

// Sleeper is a Program that wants a say around sleeping.
type Sleeper interface {
	// Sleep is called on the loop before sleeping; the func it returns,
	// if any, runs off the loop before the suspend (Wi-Fi off, say).
	Sleep() func()
	// Wake is called on the loop once awake, the screen redrawn.
	Wake()
}

// Sleep puts the Kobo to sleep, now.
func (a *App) Sleep() {
	if a.asleep || a.calib != nil {
		return
	}
	if d, ok := a.prog.(Dozer); ok && !a.dozing && d.DozeInstead() {
		a.Doze()
		return
	}
	log.Print("sleep: going to sleep")
	var prep func()
	if s, ok := a.prog.(Sleeper); ok {
		prep = s.Sleep() // while still dozing, if it was
	}
	light := -1
	if a.light != nil {
		light = a.light.level
	}
	if a.dozing { // from a doze: the page's canvas back, the light as before it
		a.endDoze()
		light = a.dozeLight
	}
	a.HideGauge()
	a.lightOverlay = nil
	a.drawSleep()
	a.WaitBlit()
	a.markAll(true)
	a.flush() // the sleep screen; nothing else gets to the panel until we wake
	a.asleep = true
	a.sleeping.Store(true)
	if a.light != nil {
		a.light.set(0)
	}
	go func() {
		if prep != nil {
			prep()
		}
		woke := suspend()
		a.alive.Store(time.Now().UnixNano())
		a.Do(func() { a.wake(light, woke) })
	}()
}

func (a *App) wake(light int, why string) {
	log.Printf("sleep: awake (%s)", why)
	a.WaitBlit()
	a.asleep = false
	a.sleeping.Store(false)
	a.inputAt = time.Now()
	if a.light != nil && light > 0 {
		a.light.set(light)
	}
	a.drawAll(true)
	if s, ok := a.prog.(Sleeper); ok {
		s.Wake()
	}
}

// drawSleep is the screen that stays on the panel while asleep.
func (a *App) drawSleep() {
	c := a.C
	b := c.Bounds()
	c.Fill(b, White)
	cy := b.Dy() / 2
	c.Text(a.F.Huge, image.Rect(0, cy-260, b.Dx(), cy-120), a.cfg.Title, Black, AlignCenter)
	c.Fill(image.Rect(b.Dx()/2-160, cy-90, b.Dx()/2+160, cy-86), Black)
	c.Text(a.F.Title, image.Rect(0, cy-60, b.Dx(), cy+10), "sleeping", Black, AlignCenter)
	c.Text(a.F.Body, image.Rect(0, cy+40, b.Dx(), cy+90), "Press the power button to wake up.", Dark, AlignCenter)
	note := "since " + time.Now().Format("15:04")
	if bat, err := os.ReadFile("/sys/class/power_supply/battery/capacity"); err == nil {
		note += fmt.Sprintf("  ·  battery %s%%", strings.TrimSpace(string(bat)))
	}
	c.Text(a.F.Small, image.Rect(0, cy+110, b.Dx(), cy+150), note, Mid, AlignCenter)
}

// suspend sleeps until something wakes the device, and says what.
func suspend() string {
	if os.Getenv("HELLOBRO_FAKE_SLEEP") != "" || !exists("/sys/power/state-extended") {
		log.Print("sleep: pretending (no Kobo here)")
		time.Sleep(3 * time.Second)
		return "pretend"
	}
	// As Nickel: tell the drivers, give them a moment, sync, then suspend
	// unless a wakeup event came meanwhile.
	if err := writeSys("/sys/power/state-extended", "1"); err != nil {
		log.Printf("sleep: state-extended: %v", err)
	}
	time.Sleep(2 * time.Second)
	syscall.Sync()
	var err error
	for try := 0; try < 3; try++ {
		if wc, e := os.ReadFile("/sys/power/wakeup_count"); e == nil {
			if e := writeSys("/sys/power/wakeup_count", strings.TrimSpace(string(wc))); e != nil {
				log.Printf("sleep: a wakeup event came first, again in a second")
				time.Sleep(time.Second)
				continue
			}
		}
		log.Print("sleep: suspending")
		if err = writeSys("/sys/power/state", "mem"); err == nil {
			break
		}
		log.Printf("sleep: the kernel wouldn't: %v", err)
		time.Sleep(time.Second)
	}
	writeSys("/sys/power/state-extended", "0")
	time.Sleep(100 * time.Millisecond)
	if err != nil {
		return "didn't sleep: " + err.Error()
	}
	return "woken by " + wakeupIRQ()
}

// wakeupIRQ names the interrupt that woke us, as far as the kernel says.
func wakeupIRQ() string {
	b, err := os.ReadFile("/sys/power/pm_wakeup_irq")
	if err != nil {
		return "something"
	}
	irq := strings.TrimSpace(string(b))
	ints, _ := os.ReadFile("/proc/interrupts")
	for _, line := range strings.Split(string(ints), "\n") {
		if f := strings.Fields(line); len(f) > 1 && f[0] == irq+":" {
			return "irq " + irq + " (" + strings.Join(f[len(f)-2:], " ") + ")"
		}
	}
	return "irq " + irq
}

func writeSys(path, v string) error {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	_, err = f.WriteString(v)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
