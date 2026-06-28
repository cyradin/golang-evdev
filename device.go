//go:build linux

package evdev

import (
	"bytes"
	"fmt"
	"iter"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

type CapabilityType struct {
	Type int
	Name string
}

type CapabilityCode struct {
	Code int
	Name string
}

// A Linux input device from which events can be read.
type InputDevice struct {
	Fn string // path to input device (devnode)

	Name string // device name
	Phys string // physical topology of device

	Bustype uint16 // bus type identifier
	Vendor  uint16 // vendor identifier
	Product uint16 // product identifier
	Version uint16 // version identifier

	EvdevVersion int // evdev protocol version

	Capabilities     map[CapabilityType][]CapabilityCode // supported event types and codes.
	CapabilitiesFlat map[int][]int

	file *os.File
}

// Open an evdev input device.
func NewInputDevice(devnode string) (*InputDevice, error) {
	// #nosec G304 -- devnode comes from trusted kernel enumeration (/sys or /proc)
	f, err := os.Open(devnode)
	if err != nil {
		return nil, err
	}

	dev := InputDevice{
		Fn:   devnode,
		file: f,
	}

	if err := dev.set_device_info(); err != nil {
		return nil, err
	}

	if err := dev.set_device_capabilities(); err != nil {
		return nil, err
	}

	return &dev, nil
}

func (d *InputDevice) Close() error {
	return d.file.Close()
}

// Read input events from device.
func (d *InputDevice) Read() iter.Seq2[InputEvent, error] {
	return NewEventReader(d.file).Read()
}

// Get a useful description for an input device. Example:
//
//	InputDevice /dev/input/event3 (fd 3)
//	  name Logitech USB Laser Mouse
//	  phys usb-0000:00:12.0-2/input0
//	  bus 0x3, vendor 0x46d, product 0xc069, version 0x110
//	  events EV_KEY 1, EV_SYN 0, EV_REL 2, EV_MSC 4
func (d *InputDevice) String() string {
	evtypes := make([]string, 0, len(d.Capabilities))
	for ev := range d.Capabilities {
		evtypes = append(evtypes, fmt.Sprintf("%s %d", ev.Name, ev.Type))
	}

	evtypes_s := strings.Join(evtypes, ", ")

	return fmt.Sprintf(
		"InputDevice %s (fd %d)\n"+
			"  name %s\n"+
			"  phys %s\n"+
			"  bus 0x%04x, vendor 0x%04x, product 0x%04x, version 0x%04x\n"+
			"  events %s",
		d.Fn, d.file.Fd(), d.Name, d.Phys, d.Bustype,
		d.Vendor, d.Product, d.Version, evtypes_s)
}

// Gets the event types and event codes that the input device supports.
func (d *InputDevice) set_device_capabilities() error {
	// Capabilities is a map of supported event types to lists of
	// events e.g: {1: [272, 273, 274, 275], 2: [0, 1, 6, 8]}
	// capabilities := make(map[int][]int)
	capabilities := make(map[CapabilityType][]CapabilityCode)

	evbits := new([(EV_MAX + 1) / 8]byte)
	codebits := new([(KEY_MAX + 1) / 8]byte)
	// absbits  := new([6]byte)

	err := ioctl(d.file.Fd(), uintptr(EVIOCGBIT(0, EV_MAX)), unsafe.Pointer(evbits))
	if err != 0 {
		return err
	}

	// Build a map of the device's capabilities
	for evtype := range EV_MAX {
		//nolint:mnd
		if evbits[evtype/8]&(1<<uint(evtype%8)) != 0 {
			eventcodes := make([]CapabilityCode, 0)

			err = ioctl(d.file.Fd(), uintptr(EVIOCGBIT(evtype, KEY_MAX)), unsafe.Pointer(codebits))
			if err != 0 {
				// ignore invalid capabilities such as EV_REP for some devices
				if err == syscall.EINVAL {
					continue
				}

				return err
			}

			for evcode := range KEY_MAX {
				//nolint:mnd
				if codebits[evcode/8]&(1<<uint(evcode%8)) != 0 {
					c := CapabilityCode{evcode, ByEventType[evtype][evcode]}
					eventcodes = append(eventcodes, c)
				}
			}

			// capabilities[EV_KEY] = [KEY_A, KEY_B, KEY_C, ...]
			key := CapabilityType{evtype, EV[evtype]}
			capabilities[key] = eventcodes
		}
	}

	d.Capabilities = capabilities

	return nil
}

// An all-in-one function for describing an input device.
func (d *InputDevice) set_device_info() error {
	// Corresponds to the input_id struct.
	type device_info struct {
		bustype, vendor, product, version uint16
	}

	info := device_info{}

	name := new([MAX_NAME_SIZE]byte)
	phys := new([MAX_NAME_SIZE]byte)

	err := ioctl(d.file.Fd(), uintptr(EVIOCGID), unsafe.Pointer(&info))
	if err != 0 {
		return fmt.Errorf("get device id: %w", err)
	}

	err = ioctl(d.file.Fd(), uintptr(EVIOCGNAME), unsafe.Pointer(name))
	if err != 0 {
		return fmt.Errorf("get device name: %w", err)
	}

	// it's ok if the topology info is not available
	_ = ioctl(d.file.Fd(), uintptr(EVIOCGPHYS), unsafe.Pointer(phys))

	d.Name = bytes_to_string(name[:])
	d.Phys = bytes_to_string(phys[:])

	d.Vendor = info.vendor
	d.Bustype = info.bustype
	d.Product = info.product
	d.Version = info.version

	if err := ioctl(d.file.Fd(), uintptr(EVIOCGVERSION), unsafe.Pointer(&d.EvdevVersion)); err != 0 {
		return fmt.Errorf("get driver version: %w", err)
	}

	return nil
}

// Get repeat rate as a two element array.
//
//	[0] repeat rate in characters per second
//	[1] amount of time that a key must be depressed before it will start
//	    to repeat (in milliseconds)
func (d *InputDevice) GetRepeatRate() *[2]uint {
	repeat_delay := new([2]uint)
	_ = ioctl(d.file.Fd(), uintptr(EVIOCGREP), unsafe.Pointer(repeat_delay))

	return repeat_delay
}

// Set repeat rate and delay.
func (d *InputDevice) SetRepeatRate(repeat, delay uint) {
	repeat_delay := new([2]uint)
	repeat_delay[0], repeat_delay[1] = repeat, delay
	_ = ioctl(d.file.Fd(), uintptr(EVIOCSREP), unsafe.Pointer(repeat_delay))
}

// Grab the input device exclusively.
func (d *InputDevice) Grab() error {
	grab := int(1)
	if err := ioctl(d.file.Fd(), uintptr(EVIOCGRAB), unsafe.Pointer(&grab)); err != 0 {
		return err
	}

	return nil
}

// Release a grabbed input device.
func (d *InputDevice) Release() error {
	if err := ioctl(d.file.Fd(), uintptr(EVIOCGRAB), unsafe.Pointer(nil)); err != 0 {
		return err
	}

	return nil
}

func bytes_to_string(b []byte) string {
	idx := bytes.IndexByte(b, 0)

	return string(b[:idx])
}
