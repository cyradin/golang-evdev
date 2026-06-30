//go:build linux

package evdev

import (
	"syscall"
	"unsafe"
)

const MAX_NAME_SIZE = 256

const (
	iocNRBits   = 8  // ioctl command number bits
	iocTypeBits = 8  // ioctl type (usually device class, e.g. 'E' for evdev)
	iocSizeBits = 14 // size of argument
	iocDirBits  = 2  // direction (none/read/write/rw)

	iocNRShift   = 0
	iocTypeShift = iocNRShift + iocNRBits
	iocSizeShift = iocTypeShift + iocTypeBits
	iocDirShift  = iocSizeShift + iocSizeBits

	iocNone  = 0
	iocWrite = 1
	iocRead  = 2

	evIO = 'E' // evdev ioctl type (from linux/input.h)
)

func _IOC(dir, typ, nr, size uintptr) uintptr {
	return (dir << iocDirShift) |
		(typ << iocTypeShift) |
		(nr << iocNRShift) |
		(size << iocSizeShift)
}

func _IOR(nr, size uintptr) uintptr {
	return _IOC(iocRead, evIO, nr, size)
}

func _IOW(nr, size uintptr) uintptr {
	return _IOC(iocWrite, evIO, nr, size)
}

// input_id from linux/input.h
type InputID struct {
	Bustype uint16
	Vendor  uint16
	Product uint16
	Version uint16
}

// absolute axis information (EVIOCGABS / EVIOCSABS)
type InputAbsInfo struct {
	Value      int32
	Minimum    int32
	Maximum    int32
	Fuzz       int32
	Flat       int32
	Resolution int32
}

// keymap entry (EVIOCGKEYCODE_V2 / EVIOCSKEYCODE_V2)
type InputKeymapEntry struct {
	Flags    uint8
	Len      uint8
	Index    uint16
	Keycode  uint32
	Scancode [32]byte
}

// device metadata queries
const (
	evIOCGID      = 0x02 // device identification
	evIOCGVERSION = 0x01 // evdev driver version
	evIOCGREP     = 0x03 // keyboard repeat settings
	evIOCGKEYCODE = 0x04 // keycode mapping (legacy)
)

// device control ioctls
const (
	evIOCGRAB     = 0x90 // exclusive device grab
	evIOCSCLOCKID = 0xA0 // set timestamp clock
)

// bitmask / query ranges
const (
	evIOCGNAME = 0x06 // device name
	evIOCGPHYS = 0x07 // physical device path
	evIOCGUNIQ = 0x08 // unique device id
	evIOCGPROP = 0x09 // device properties

	evIOCGKEY = 0x18 // key state bitmap
	evIOCGLED = 0x19 // LED state bitmap
	evIOCGSND = 0x1A // sound state bitmap
	evIOCGSW  = 0x1B // switch state bitmap
)

const (
	evIOCGBITBase = 0x20 // event capability bitmap base (EV_KEY, EV_REL, etc.)
	evIOCGABSBase = 0x40 // absolute axis query base
	evIOCSABSBase = 0xC0 // absolute axis set base
)

// string-based device queries
func EVIOCGNAME(length uintptr) uintptr { return _IOR(evIOCGNAME, length) }
func EVIOCGPHYS(length uintptr) uintptr { return _IOR(evIOCGPHYS, length) }
func EVIOCGUNIQ(length uintptr) uintptr { return _IOR(evIOCGUNIQ, length) }
func EVIOCGPROP(length uintptr) uintptr { return _IOR(evIOCGPROP, length) }

// bitmasks
func EVIOCGKEY(length uintptr) uintptr { return _IOR(evIOCGKEY, length) }
func EVIOCGLED(length uintptr) uintptr { return _IOR(evIOCGLED, length) }
func EVIOCGSND(length uintptr) uintptr { return _IOR(evIOCGSND, length) }
func EVIOCGSW(length uintptr) uintptr  { return _IOR(evIOCGSW, length) }

// capability bit querying (EVIOCGBIT)
func EVIOCGBIT(ev int, length int) uintptr {
	return _IOR(evIOCGBITBase+uintptr(ev), uintptr(length))
}

// absolute axis access
func EVIOCGABS(abs uintptr) uintptr {
	return _IOR(evIOCGABSBase+abs, unsafe.Sizeof(InputAbsInfo{}))
}

func EVIOCSABS(abs uintptr) uintptr {
	return _IOW(evIOCSABSBase+abs, unsafe.Sizeof(InputAbsInfo{}))
}

var (
	EVIOCGID      = _IOR(evIOCGID, unsafe.Sizeof(InputID{}))
	EVIOCGVERSION = _IOR(evIOCGVERSION, unsafe.Sizeof(int32(0)))
	EVIOCGREP     = _IOR(evIOCGREP, unsafe.Sizeof([2]uint32{}))
	EVIOCSREP     = _IOW(evIOCGREP, unsafe.Sizeof([2]uint32{}))

	EVIOCGKEYCODE    = _IOR(evIOCGKEYCODE, unsafe.Sizeof(InputKeymapEntry{}))
	EVIOCGKEYCODE_V2 = _IOR(evIOCGKEYCODE, unsafe.Sizeof(InputKeymapEntry{}))
	EVIOCSKEYCODE    = _IOW(evIOCGKEYCODE, unsafe.Sizeof(InputKeymapEntry{}))
	EVIOCSKEYCODE_V2 = _IOW(evIOCGKEYCODE, unsafe.Sizeof(InputKeymapEntry{}))

	EVIOCGRAB     = _IOW(evIOCGRAB, unsafe.Sizeof(int32(0)))
	EVIOCSCLOCKID = _IOW(evIOCSCLOCKID, unsafe.Sizeof(int32(0)))
)

func ioctl(fd uintptr, name uintptr, data unsafe.Pointer) syscall.Errno {
	_, _, err := syscall.RawSyscall(syscall.SYS_IOCTL, fd, name, uintptr(data))
	return err
}
