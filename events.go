package evdev

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"iter"
	"syscall"
	"unsafe"
)

var ErrReadEvents = fmt.Errorf("read events")

type InputEvent struct {
	Time  syscall.Timeval // time in seconds since epoch at which event occurred
	Type  uint16          // event type - one of ecodes.EV_*
	Code  uint16          // event code related to the event type
	Value int32           // event value related to the event type
}

func (e InputEvent) IsEmpty() bool {
	return e.Time.Sec == 0
}

// Get a useful description for an input event. Example:
//
//	event at 1347905437.435795, code 01, type 02, val 02
func (e *InputEvent) String() string {
	return fmt.Sprintf("event at %d.%d, code %02d, type %02d, val %02d",
		e.Time.Sec, e.Time.Usec, e.Code, e.Type, e.Value)
}

var eventsize = int(unsafe.Sizeof(InputEvent{}))

type KeyEventState uint8

const (
	KeyUp   KeyEventState = 0x0
	KeyDown KeyEventState = 0x1
	KeyHold KeyEventState = 0x2
)

// KeyEvents are used to describe state changes of keyboards, buttons,
// or other key-like devices.
type KeyEvent struct {
	Event    *InputEvent
	Scancode uint16
	Keycode  uint16
	State    KeyEventState
}

func (e *KeyEvent) New(ev *InputEvent) {
	e.Event = ev
	e.Keycode = 0 // :todo
	e.Scancode = ev.Code

	//nolint:mnd
	switch ev.Value {
	case 0:
		e.State = KeyUp
	case 2:
		e.State = KeyHold
	case 1:
		e.State = KeyDown
	}
}

func NewKeyEvent(ev *InputEvent) *KeyEvent {
	kev := &KeyEvent{}
	kev.New(ev)

	return kev
}

func (e *KeyEvent) String() string {
	state := "unknown"

	switch e.State {
	case KeyUp:
		state = "up"
	case KeyHold:
		state = "hold"
	case KeyDown:
		state = "down"
	}

	return fmt.Sprintf("key event at %d.%d, %d (%d), (%s)",
		e.Event.Time.Sec, e.Event.Time.Usec,
		e.Scancode, e.Event.Code, state)
}

// RelEvents are used to describe relative axis value changes,
// e.g. moving the mouse 5 units to the left.
type RelEvent struct {
	Event *InputEvent
}

func (e *RelEvent) New(ev *InputEvent) {
	e.Event = ev
}

func NewRelEvent(ev *InputEvent) *RelEvent {
	rev := &RelEvent{}
	rev.New(ev)

	return rev
}

func (e *RelEvent) String() string {
	return fmt.Sprintf("relative axis event at %d.%d, %s",
		e.Event.Time.Sec, e.Event.Time.Usec,
		REL[int(e.Event.Code)])
}

// TODO: Make this work

var EventFactory map[uint16]any = make(map[uint16]any)

func init() {
	EventFactory[uint16(EV_KEY)] = NewKeyEvent
	EventFactory[uint16(EV_REL)] = NewRelEvent
}

type EventReader struct {
	in     io.Reader
	events []InputEvent
	buf    []byte
}

func NewEventReader(in io.Reader) *EventReader {
	const bufferSize = 16

	return &EventReader{
		in:     in,
		events: make([]InputEvent, bufferSize),
		buf:    make([]byte, bufferSize*eventsize),
	}
}

func (r *EventReader) Read() iter.Seq2[InputEvent, error] {
	return func(yield func(InputEvent, error) bool) {
		for {
			n, err := r.in.Read(r.buf)
			if n > 0 {
				if procErr := r.process(n, yield); procErr != nil {
					yield(InputEvent{}, procErr)

					return
				}
			}

			if err != nil {
				if !errors.Is(err, io.EOF) {
					yield(InputEvent{}, fmt.Errorf("%w: %w", ErrReadEvents, err))
				}

				return
			}
		}
	}
}

func (r *EventReader) process(n int, yield func(InputEvent, error) bool) error {
	cnt := n / eventsize
	if cnt == 0 {
		return nil
	}

	tempEvents := r.events[:cnt]

	if _, err := binary.Decode(r.buf[:n], binary.LittleEndian, &tempEvents); err != nil {
		return fmt.Errorf("decode error: %w", err)
	}

	for i := range cnt {
		if !yield(tempEvents[i], nil) {
			return nil
		}
	}

	return nil
}
