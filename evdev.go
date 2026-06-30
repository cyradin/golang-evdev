package evdev

import (
	"os"
	"path/filepath"
)

// Return a list of accessible input device names matched by
// deviceglob (default '/dev/input/event*').
func ListInputDevicePaths(deviceGlob string) ([]string, error) {
	paths, err := filepath.Glob(deviceGlob)
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
		dev, err := NewInputDevice(fns[i])
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
