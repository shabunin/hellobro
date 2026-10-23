package keyfeed

// The keyboard the keys go to: a uinput device, made the old way
// (struct uinput_user_dev written, then UI_DEV_CREATE), which old
// kernels (3.0) know too. The device's readers (hellobro) take it for any
// other keyboard.

import (
	"bytes"
	"encoding/binary"
	"os"
	"syscall"
	"time"
	"unsafe"
)

const (
	uiSetEvBit   = 0x40045564 // _IOW('U', 100, int)
	uiSetKeyBit  = 0x40045565 // _IOW('U', 101, int)
	uiDevCreate  = 0x5501     // _IO('U', 1)
	uiDevDestroy = 0x5502     // _IO('U', 2)
	evSyn        = 0
	evKey        = 1
	busVirtual   = 6
)

// Keyboard is a uinput keyboard.
type Keyboard struct{ f *os.File }

// NewKeyboard makes a keyboard called name.
func NewKeyboard(name string) (*Keyboard, error) {
	f, err := os.OpenFile("/dev/uinput", os.O_WRONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	k := &Keyboard{f}
	if err := k.ioctl(uiSetEvBit, evKey); err != nil {
		f.Close()
		return nil, err
	}
	for c := uintptr(1); c < KeyMax; c++ {
		if err := k.ioctl(uiSetKeyBit, c); err != nil {
			f.Close()
			return nil, err
		}
	}
	// struct uinput_user_dev: name[80], struct input_id, ff_effects_max,
	// and the absolute axes' ranges (none here).
	var dev bytes.Buffer
	n := make([]byte, 80)
	copy(n, name)
	dev.Write(n)
	binary.Write(&dev, binary.LittleEndian, [4]uint16{busVirtual, 0x1209, 0xb0b0, 1})
	dev.Write(make([]byte, 4+4*64*4))
	if _, err := f.Write(dev.Bytes()); err != nil {
		f.Close()
		return nil, err
	}
	if err := k.ioctl(uiDevCreate, 0); err != nil {
		f.Close()
		return nil, err
	}
	return k, nil
}

func (k *Keyboard) ioctl(req, arg uintptr) error {
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, k.f.Fd(), req, arg); errno != 0 {
		return errno
	}
	return nil
}

// inputEvent is struct input_event, as wide as this machine's timeval.
type inputEvent struct {
	Time  syscall.Timeval
	Type  uint16
	Code  uint16
	Value int32
}

// Send sends the events, each its own report.
func (k *Keyboard) Send(evs []Event) error {
	var buf bytes.Buffer
	now := syscall.NsecToTimeval(time.Now().UnixNano())
	for _, e := range evs {
		for _, ie := range []inputEvent{{now, evKey, e.Code, e.Value}, {now, evSyn, 0, 0}} {
			buf.Write(unsafe.Slice((*byte)(unsafe.Pointer(&ie)), unsafe.Sizeof(ie)))
		}
	}
	if buf.Len() == 0 {
		return nil
	}
	_, err := k.f.Write(buf.Bytes())
	return err
}

// Close takes the keyboard away.
func (k *Keyboard) Close() error {
	k.ioctl(uiDevDestroy, 0)
	return k.f.Close()
}
