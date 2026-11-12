package kobo

// Sleep: the Kobo suspended to RAM, with a sleep screen on the panel, the
// way Nickel and KOReader do it on these devices (state-extended, then
// "mem"), and the app back as it was on waking. Nickel holds the power
// button, so going to sleep is ours to ask for (the header's button,
// Ctrl+Alt+S, Sleep); the button still wakes the device up.

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

// SleepScreener is a Program that draws the sleep screen itself (an
// agenda, say): on c, all of it, as of now; false leaves it to us. A
// time to draw it again (a new day) wakes the Kobo then for a moment,
// by the RTC's alarm, and it sleeps on.
type SleepScreener interface {
	SleepScreen(c Canvas, now time.Time) (again time.Time, ok bool)
}

// Sleep puts the Kobo to sleep, now.
func (a *App) Sleep() {
	if a.asleep || a.calib != nil {
		return
	}
	log.Print("sleep: going to sleep")
	var prep func()
	if s, ok := a.prog.(Sleeper); ok {
		prep = s.Sleep()
	}
	light := -1
	if a.light != nil {
		light = a.lightLevel()
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
		var woke string
		for {
			again := a.sleepAgain
			setAlarm(again)
			woke = suspend()
			if again.IsZero() || !alarmRang(again, time.Now()) {
				break
			}
			// The alarm: the sleep screen as of now, and on sleeping.
			log.Printf("sleep: the sleep screen again (%s)", woke)
			done := make(chan struct{})
			a.Do(func() {
				a.asleep = false
				a.drawSleep()
				a.WaitBlit()
				a.markAll(true)
				a.flush()
				a.WaitBlit()
				a.asleep = true
				close(done)
			})
			<-done
		}
		setAlarm(time.Time{})
		a.alive.Store(time.Now().UnixNano())
		a.Do(func() { a.wake(light, woke) })
	}()
}

const rtcAlarm = "/sys/class/rtc/rtc0/wakealarm"

// setAlarm has the RTC wake the Kobo at t; zero: not.
func setAlarm(t time.Time) {
	if os.Getenv("HELLOBRO_FAKE_SLEEP") != "" || !exists(rtcAlarm) {
		return
	}
	writeSys(rtcAlarm, "0") // a new one only over none
	if t.IsZero() {
		return
	}
	if err := writeSys(rtcAlarm, fmt.Sprint(t.Unix())); err != nil {
		log.Printf("sleep: the alarm: %v", err)
	}
}

// alarmRang says waking at now was the alarm set for again, not the
// power button: the kernel doesn't tell them apart (both the PMIC's).
// A press within a minute of it is taken for the alarm.
func alarmRang(again, now time.Time) bool {
	return !now.Before(again.Add(-10*time.Second)) && now.Before(again.Add(time.Minute))
}

func (a *App) wake(light int, why string) {
	log.Printf("sleep: awake (%s)", why)
	a.WaitBlit()
	a.asleep = false
	a.sleeping.Store(false)
	a.wokeAt = time.Now()
	a.inputAt = time.Now()
	if a.light != nil && light > 0 {
		a.light.set(light)
	}
	if a.locked { // the lock screen again, over the sleep screen
		a.drawLock()
	}
	a.drawAll(true)
	if s, ok := a.prog.(Sleeper); ok {
		s.Wake()
	}
}

// drawSleep is the screen that stays on the panel while asleep: the
// Program's, if it has one.
func (a *App) drawSleep() {
	c := a.panel()
	a.sleepAgain = time.Time{}
	if s, ok := a.prog.(SleepScreener); ok {
		if again, ok := s.SleepScreen(c, time.Now()); ok {
			a.sleepAgain = again
			return
		}
	}
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

// powerPressed is the power button: it sleeps.
func (a *App) powerPressed() {
	switch {
	case a.asleep || a.calib != nil || time.Since(a.wokeAt) < 3*time.Second: // the press that woke it
		log.Print("power button: pressed (nothing to do)")
	default:
		log.Print("power button: sleeping")
		a.Sleep()
	}
}

// IdleFor is how long since the last touch or key.
func (a *App) IdleFor() time.Duration { return time.Since(a.inputAt) }

// PowerButton says whether the power button reaches us.
func (a *App) PowerButton() bool { return !a.noPower }
