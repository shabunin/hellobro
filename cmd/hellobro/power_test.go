package main

import (
	"math"
	"testing"
	"time"
)

func TestParseProcStat(t *testing.T) {
	pid, pt, ok := parseProcStat("1234 (my (odd) app) S 1 1234 1234 0 -1 4194560 100 0 0 0 250 50 0 0 20 0 3 0 100 0 0")
	if !ok || pid != 1234 || pt.name != "my (odd) app" || pt.ticks != 300 {
		t.Errorf("got %d %+v %v", pid, pt, ok)
	}
	if _, _, ok := parseProcStat("garbage"); ok {
		t.Error("garbage parsed")
	}
}

func TestParseInterrupts(t *testing.T) {
	m := parseInterrupts(`           CPU0       CPU1
 19:   13070003    100     GIC-0  30 Level     arch_timer
 32:       9102      0  sunxi_pio_edge   8 Edge      elan_ktf
 68:         11      0  sunxi_pio_edge   8 Edge      sdc0 cd
196:    1328498      2     GIC-0  74 Level     uart1
IPI2:      15244     10  Rescheduling interrupts
Err:          0
`)
	for name, want := range map[string]uint64{"timer": 13070103, "touch": 9102, "sdc0 cd": 11, "uart1": 1328500, "rescheduling": 15254} {
		if m[name] != want {
			t.Errorf("%s: %d, want %d", name, m[name], want)
		}
	}
	if len(m) != 5 {
		t.Errorf("got %v", m)
	}
}

func TestParseHistory(t *testing.T) {
	h := parseHistory("100 50 270 d\nbad line\n200 49 280 d\n300 48 500 c\n", time.Unix(150, 0))
	if len(h) != 2 || h[0].pct != 49 || h[0].ma != 280 || h[0].charging || !h[1].charging {
		t.Errorf("got %+v", h)
	}
	if got := h[0].String(); got != "200 49 280 d" {
		t.Errorf("String: %q", got)
	}
}

func TestDrainRate(t *testing.T) {
	t0 := time.Unix(1e9, 0)
	run := func(pcts ...int) []batSample {
		var h []batSample
		for i, p := range pcts {
			h = append(h, batSample{at: t0.Add(time.Duration(i*10) * time.Minute), pct: p})
		}
		return h
	}
	// 6 % in an hour.
	if rate, over, ok := drainRate(run(80, 79, 78, 77, 76, 75, 74)); !ok || math.Abs(rate+6) > 0.01 || over != time.Hour {
		t.Errorf("got %v %v %v", rate, over, ok)
	}
	// Too soon to say.
	if _, _, ok := drainRate(run(80, 80)); ok {
		t.Error("10 min, no change: should wait")
	}
	// A charge in between: only the run since.
	h := run(50, 49, 48, 60, 59, 58)
	h[3].charging = false
	h[0].charging, h[1].charging, h[2].charging = true, true, true
	if rate, over, ok := drainRate(h); !ok || over != 20*time.Minute || math.Abs(rate+6) > 0.01 {
		t.Errorf("after a charge: %v %v %v", rate, over, ok)
	}
	// A gap (hellobro off): only the run since.
	h = run(80, 79, 78)
	for i, p := range []int{60, 59, 58} {
		h = append(h, batSample{at: t0.Add(2*time.Hour + time.Duration(i*10)*time.Minute), pct: p})
	}
	if rate, over, ok := drainRate(h); !ok || over != 20*time.Minute || math.Abs(rate+6) > 0.01 {
		t.Errorf("after a gap: %v %v %v", rate, over, ok)
	}
}

func TestUsageSince(t *testing.T) {
	t0 := time.Unix(1e9, 0)
	p := &usageCounts{at: t0, idle: 100, total: 200, freq: map[int]uint64{480000: 100, 1800000: 50},
		cstate: map[string]uint64{"WFI": 0}, procs: map[int]procTicks{1: {"hellobro", 100}, 2: {"idle", 5}},
		irqs: map[string]uint64{"timer": 1000}}
	c := &usageCounts{at: t0.Add(10 * time.Second), idle: 190, total: 300, freq: map[int]uint64{480000: 190, 1800000: 60},
		cstate: map[string]uint64{"WFI": 5e6}, procs: map[int]procTicks{1: {"hellobro", 150}, 2: {"idle", 5}, 3: {"new", 9}},
		irqs: map[string]uint64{"timer": 3000}}
	u := usageSince(p, c)
	if math.Abs(u.busy-0.1) > 1e-9 {
		t.Errorf("busy %v", u.busy)
	}
	if len(u.freqs) != 2 || u.freqs[0].name != "480 MHz" || math.Abs(u.freqs[0].v-0.9) > 1e-9 || u.freqs[1].name != "1.8 GHz" {
		t.Errorf("freqs %+v", u.freqs)
	}
	if len(u.cstates) != 1 || math.Abs(u.cstates[0].v-0.5) > 1e-9 {
		t.Errorf("cstates %+v", u.cstates)
	}
	if len(u.procs) != 1 || u.procs[0].name != "hellobro" || math.Abs(u.procs[0].v-5) > 1e-9 { // 50 ticks in 10 s: 5 %
		t.Errorf("procs %+v", u.procs)
	}
	if math.Abs(u.wakeups-200) > 1e-9 {
		t.Errorf("wakeups %v", u.wakeups)
	}
}
