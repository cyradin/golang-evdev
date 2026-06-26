//go:build linux

package evdev

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

// A Linux input device from which events can be read.
type InputDevice struct {
	Fn string // path to input device (devnode)

	Name string   // device name
	Phys string   // physical topology of device
	File *os.File // an open file handle to the input device

	Bustype uint16 // bus type identifier
	Vendor  uint16 // vendor identifier
	Product uint16 // product identifier
	Version uint16 // version identifier

	EvdevVersion int // evdev protocol version

	Capabilities     map[CapabilityType][]CapabilityCode // supported event types and codes.
	CapabilitiesFlat map[int][]int
}

// Open an evdev input device.
func Open(devnode string) (*InputDevice, error) {
	// #nosec G304 -- devnode comes from trusted kernel enumeration (/sys or /proc)
	f, err := os.Open(devnode)
	if err != nil {
		return nil, err
	}

	dev := InputDevice{}
	dev.Fn = devnode
	dev.File = f

	err = dev.set_device_info()
	if err != nil {
		return nil, err
	}

	err = dev.set_device_capabilities()
	if err != nil {
		return nil, err
	}

	return &dev, nil
}

// Read and return a slice of input events from device.
func (d *InputDevice) Read() ([]InputEvent, error) {
	events := make([]InputEvent, 16)
	buffer := make([]byte, eventsize*16)

	if _, err := d.File.Read(buffer); err != nil {
		return nil, err
	}

	b := bytes.NewBuffer(buffer)
	if err := binary.Read(b, binary.LittleEndian, &events); err != nil {
		return events, err
	}

	// remove trailing structures
	for i := range events {
		if events[i].Time.Sec == 0 {
			events = events[:i]
			break
		}
	}

	return events, nil
}

// Read and return a single input event.
func (d *InputDevice) ReadOne() (*InputEvent, error) {
	event := InputEvent{}
	buffer := make([]byte, eventsize)

	if _, err := d.File.Read(buffer); err != nil {
		return &event, err
	}

	b := bytes.NewBuffer(buffer)
	if err := binary.Read(b, binary.LittleEndian, &event); err != nil {
		return &event, err
	}

	return &event, nil
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
		d.Fn, d.File.Fd(), d.Name, d.Phys, d.Bustype,
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

	err := ioctl(d.File.Fd(), uintptr(EVIOCGBIT(0, EV_MAX)), unsafe.Pointer(evbits))
	if err != 0 {
		return err
	}

	// Build a map of the device's capabilities
	for evtype := range EV_MAX {
		//nolint:mnd
		if evbits[evtype/8]&(1<<uint(evtype%8)) != 0 {
			eventcodes := make([]CapabilityCode, 0)

			err = ioctl(d.File.Fd(), uintptr(EVIOCGBIT(evtype, KEY_MAX)), unsafe.Pointer(codebits))
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
	info := device_info{}

	name := new([MAX_NAME_SIZE]byte)
	phys := new([MAX_NAME_SIZE]byte)

	err := ioctl(d.File.Fd(), uintptr(EVIOCGID), unsafe.Pointer(&info))
	if err != 0 {
		return fmt.Errorf("get device id: %w", err)
	}

	err = ioctl(d.File.Fd(), uintptr(EVIOCGNAME), unsafe.Pointer(name))
	if err != 0 {
		return fmt.Errorf("get device name: %w", err)
	}

	// it's ok if the topology info is not available
	_ = ioctl(d.File.Fd(), uintptr(EVIOCGPHYS), unsafe.Pointer(phys))

	d.Name = bytes_to_string(name[:])
	d.Phys = bytes_to_string(phys[:])

	d.Vendor = info.vendor
	d.Bustype = info.bustype
	d.Product = info.product
	d.Version = info.version

	if err := ioctl(d.File.Fd(), uintptr(EVIOCGVERSION), unsafe.Pointer(&d.EvdevVersion)); err != 0 {
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
	_ = ioctl(d.File.Fd(), uintptr(EVIOCGREP), unsafe.Pointer(repeat_delay))

	return repeat_delay
}

// Set repeat rate and delay.
func (d *InputDevice) SetRepeatRate(repeat, delay uint) {
	repeat_delay := new([2]uint)
	repeat_delay[0], repeat_delay[1] = repeat, delay
	_ = ioctl(d.File.Fd(), uintptr(EVIOCSREP), unsafe.Pointer(repeat_delay))
}

// Grab the input device exclusively.
func (d *InputDevice) Grab() error {
	grab := int(1)
	if err := ioctl(d.File.Fd(), uintptr(EVIOCGRAB), unsafe.Pointer(&grab)); err != 0 {
		return err
	}

	return nil
}

// Release a grabbed input device.
func (d *InputDevice) Release() error {
	if err := ioctl(d.File.Fd(), uintptr(EVIOCGRAB), unsafe.Pointer(nil)); err != 0 {
		return err
	}

	return nil
}

type CapabilityType struct {
	Type int
	Name string
}

type CapabilityCode struct {
	Code int
	Name string
}

// Corresponds to the input_id struct.
type device_info struct {
	bustype, vendor, product, version uint16
}

// Determine if a path exist and is a character input device.
func IsInputDevice(path string) bool {
	fi, err := os.Stat(path)

	if os.IsNotExist(err) {
		return false
	}

	m := fi.Mode()
	if m&os.ModeCharDevice == 0 { //nolint:staticcheck
		return false
	}

	return true
}

// Return a list of accessible input device names matched by
// deviceglob (default '/dev/input/event*').
func ListInputDevicePaths(device_glob string) ([]string, error) {
	paths, err := filepath.Glob(device_glob)
	if err != nil {
		return nil, err
	}

	devices := make([]string, 0)

	for _, path := range paths {
		if IsInputDevice(path) {
			devices = append(devices, path)
		}
	}

	return devices, nil
}

// Return a list of accessible input devices matched by deviceglob
// (default '/dev/input/event/*').
func ListInputDevices(deviceGlobArg ...string) ([]*InputDevice, error) {
	deviceGlob := "/dev/input/event*"
	if len(deviceGlobArg) > 0 {
		deviceGlob = deviceGlobArg[0]
	}

	fns, _ := ListInputDevicePaths(deviceGlob)
	devices := make([]*InputDevice, 0)

	for i := range fns {
		dev, err := Open(fns[i])
		if err == nil {
			devices = append(devices, dev)
		}
	}

	return devices, nil
}

func bytes_to_string(b []byte) string {
	idx := bytes.IndexByte(b[:], 0)
	return string(b[:idx])
}
