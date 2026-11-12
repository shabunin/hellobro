package kobo

// Auto-rotate: the screen turns as the Kobo is held. The Elipsa's
// accelerometer (a KX122 on I²C) works out which side is up by itself
// (its tilt engine), and we ask it now and then, as FBInk does when it
// starts: a byte, its TSCP register. Nickel's own rotation events go to
// the buttons' device, which Nickel holds (grabbed) while it's frozen,
// so they never reach us. A side up for two looks running turns the
// screen, if nothing has touched it for a moment (not mid-stroke), and
// only to a rotation the touch is calibrated for: the first time is
// the Rotate command's, to calibrate it.

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

const (
	kx122TSCP    = 0x10 // the side up now
	i2cSlaveF    = 0x0706
	i2cSMBus     = 0x0720
	tiltEvery    = time.Second
	tiltStill    = 2 * time.Second // untouched this long before it turns
	smbusRead    = 1
	smbusByteDat = 2
)

// tiltOf is the canonical rotation (FB_ROTATE_*) a TSCP byte says, or
// -1 for none (face up or down, or not sure). The Elipsa's board has
// it the other way up (FBInk's translate_kx122).
func tiltOf(v byte) int {
	r := -1
	switch {
	case v&(1<<2) != 0: // y+ up
		r = 0
	case v&(1<<4) != 0: // x+ right
		r = 1
	case v&(1<<3) != 0: // y- down
		r = 2
	case v&(1<<5) != 0: // x- left
		r = 3
	}
	if r < 0 {
		return -1
	}
	return (r + 2) & 3
}

// tiltSensor reads the TSCP byte: from the chip, or the harness's file
// (HELLOBRO_TILT, the byte in hex).
type tiltSensor struct {
	f    *os.File
	fake string
}

func openTilt() (*tiltSensor, error) {
	if p := os.Getenv("HELLOBRO_TILT"); p != "" {
		return &tiltSensor{fake: p}, nil
	}
	ds, _ := filepath.Glob("/sys/bus/i2c/drivers/kx122/*-*")
	if len(ds) == 0 {
		return nil, fmt.Errorf("no kx122")
	}
	var bus, addr int
	if _, err := fmt.Sscanf(filepath.Base(ds[0]), "%d-%x", &bus, &addr); err != nil {
		return nil, fmt.Errorf("kx122 at %s: %v", ds[0], err)
	}
	f, err := os.Open(fmt.Sprintf("/dev/i2c-%d", bus))
	if err != nil {
		return nil, err
	}
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), i2cSlaveF, uintptr(addr)); e != 0 { // forced: the kernel's driver has it too
		f.Close()
		return nil, fmt.Errorf("I2C_SLAVE_FORCE %#x: %v", addr, e)
	}
	return &tiltSensor{f: f}, nil
}

// read is the side up, as tiltOf says.
func (t *tiltSensor) read() (int, error) {
	if t.fake != "" {
		b, err := os.ReadFile(t.fake)
		if err != nil {
			return -1, err
		}
		v, err := strconv.ParseUint(strings.TrimSpace(string(b)), 16, 8)
		return tiltOf(byte(v)), err
	}
	var data [34]byte // union i2c_smbus_data
	arg := struct {
		rw   uint8
		cmd  uint8
		size uint32
		data unsafe.Pointer
	}{rw: smbusRead, cmd: kx122TSCP, size: smbusByteDat, data: unsafe.Pointer(&data)}
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, t.f.Fd(), i2cSMBus, uintptr(unsafe.Pointer(&arg))); e != 0 {
		return -1, e
	}
	return tiltOf(data[0]), nil
}

var (
	autoRotate atomic.Bool
	tiltNow    atomic.Int32 // the side up, as last read
)

// SetAutoRotate turns the screen as it's held, or doesn't.
func (a *App) SetAutoRotate(on bool) {
	autoRotate.Store(on)
	if on && !a.tiltOn {
		a.tiltOn = true
		go a.watchTilt()
	}
}

// watchTilt asks the sensor every second, while auto-rotate is on.
func (a *App) watchTilt() {
	var t *tiltSensor
	last, same := -1, 0
	for {
		time.Sleep(tiltEvery)
		if !autoRotate.Load() {
			last, same = -1, 0
			continue
		}
		if t == nil {
			var err error
			if t, err = openTilt(); err != nil {
				log.Printf("auto-rotate: %v", err)
				return
			}
		}
		c, err := t.read()
		if err != nil {
			log.Printf("auto-rotate: %v", err)
			continue
		}
		tiltNow.Store(int32(c))
		if c != last {
			last, same = c, 0
			continue
		}
		if same++; same == 1 && c >= 0 { // two looks the same: once
			a.Do(func() { a.tiltTo(c) })
		}
	}
}

// tiltTo turns the screen to canonical rotation c, if it may now.
func (a *App) tiltTo(c int) {
	s := a.scr.info
	if !autoRotate.Load() || a.asleep || a.calib != nil || s.Canonical == c || int(tiltNow.Load()) != c {
		return
	}
	if time.Since(a.inputAt) < tiltStill || len(a.gestures) > 0 {
		go func() { // again, once it's still
			time.Sleep(tiltStill)
			a.Do(func() { a.tiltTo(c) })
		}()
		return
	}
	for r, rc := range s.RotaMap {
		if rc != c {
			continue
		}
		if _, ok := a.cal[r]; !ok && a.touch != nil {
			log.Printf("auto-rotate: rotation %d isn't calibrated: Rotate to it once", r)
			return
		}
		log.Printf("auto-rotate: held at %d, rotation %d", c, r)
		a.SetRotation(r)
		return
	}
}
