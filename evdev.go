package evdev

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
)

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
