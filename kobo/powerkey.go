package kobo

// The power button when someone else holds its input device: Nickel grabs
// it, and stays frozen with the grab all the while we run, so its events
// never reach us. The button's GPIO still shows in debugfs, though, as a
// line like
//
//	gpio-356 (                    |Power               ) in  hi
//
// so we poll that: a press is its level going away from what it was at
// the start, and coming back.

import (
	"log"
	"os"
	"strings"
	"time"
)

var gpioDebug = EnvOr("HELLOBRO_GPIO", "/sys/kernel/debug/gpio") // the harness has a stand-in

const powerPoll = 100 * time.Millisecond

// powerLevel is the power button's GPIO level (true: high), if debugfs
// shows it.
func powerLevel() (hi, ok bool) {
	b, err := os.ReadFile(gpioDebug)
	if err != nil {
		return false, false
	}
	for _, line := range strings.Split(string(b), "\n") {
		i := strings.IndexByte(line, '|')
		j := strings.IndexByte(line, ')')
		if i < 0 || j < i || !strings.EqualFold(strings.TrimSpace(line[i+1:j]), "power") {
			continue
		}
		f := strings.Fields(line[j+1:])
		if len(f) < 2 {
			return false, false
		}
		return f[1] == "hi", true
	}
	return false, false
}

// watchPowerGPIO sends the power button's presses and releases, as from
// its input device; false if its GPIO can't be read.
func watchPowerGPIO(out chan<- KeyMsg) bool {
	idle, ok := powerLevel()
	if !ok {
		return false
	}
	log.Printf("power button: watching its GPIO in %s (idle %v)", gpioDebug, map[bool]string{true: "hi", false: "lo"}[idle])
	go func() {
		down := false
		for range time.Tick(powerPoll) {
			hi, ok := powerLevel()
			if !ok || (hi != idle) == down {
				continue
			}
			down = !down
			out <- KeyMsg{Code: keyPower, Value: int32(b2i(down)), button: true, at: time.Now()}
		}
	}()
	return true
}
