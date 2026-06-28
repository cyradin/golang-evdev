package evdev

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"os"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

func BenchmarkEventReader_Read(b *testing.B) {
	testFile := eventFile(b, 1000)
	f, err := os.Open(testFile)
	require.NoError(b, err)
	b.ResetTimer()

	for b.Loop() {
		// rewind file
		b.StopTimer()

		_, err = f.Seek(0, 0)
		require.NoError(b, err)

		b.StartTimer()

		for range NewEventReader(f).Read() {
		}
	}
}

func TestEventReader_Read(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		reader      io.Reader
		want        []InputEvent
		stopAfter   int
		expectedErr error
	}{
		{
			name:   "single event",
			reader: bytes.NewReader(marshalEvents(t, newEvent())),
			want:   []InputEvent{newEvent()},
		},
		{
			name: "multiple events",
			reader: bytes.NewReader(marshalEvents(
				t,
				newEvent(),
				InputEvent{
					Time:  syscall.Timeval{Sec: 2},
					Type:  2,
					Code:  31,
					Value: 2,
				},
			)),
			want: []InputEvent{
				newEvent(),
				{
					Time:  syscall.Timeval{Sec: 2},
					Type:  2,
					Code:  31,
					Value: 2,
				},
			},
		},
		{
			name:   "empty reader",
			reader: bytes.NewReader(nil),
		},
		{
			name:   "partial event",
			reader: bytes.NewReader(make([]byte, eventsize-1)),
		},
		{
			name:        "read error",
			reader:      errorReader{err: errors.New("error")},
			expectedErr: ErrReadEvents,
		},
		{
			name: "stop after first event",
			reader: bytes.NewReader(marshalEvents(
				t,
				newEvent(),
				newEvent(),
				newEvent(),
			)),
			want:      []InputEvent{newEvent()},
			stopAfter: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var (
				got    []InputEvent
				gotErr error
			)

			i := 0

			for ev, err := range NewEventReader(tt.reader).Read() {
				if err != nil {
					gotErr = err
					break
				}

				got = append(got, ev)
				i++

				if tt.stopAfter > 0 && i >= tt.stopAfter {
					break
				}
			}

			require.ErrorIs(t, gotErr, tt.expectedErr)
			require.Equal(t, tt.want, got)
		})
	}
}

func marshalEvents(tb testing.TB, events ...InputEvent) []byte {
	tb.Helper()

	var buf bytes.Buffer

	for _, ev := range events {
		err := binary.Write(&buf, binary.LittleEndian, ev)
		require.NoError(tb, err)
	}

	return buf.Bytes()
}

type errorReader struct {
	err error
}

func (r errorReader) Read([]byte) (int, error) {
	return 0, r.err
}

func eventFile(tb testing.TB, cnt int) string {
	tb.Helper()

	dir := tb.TempDir()
	f, err := os.CreateTemp(dir, "events-*")
	require.NoError(tb, err)

	defer func() { _ = f.Close() }()

	for range cnt {
		err := binary.Write(f, binary.LittleEndian, newEvent())
		require.NoError(tb, err)
	}

	return f.Name()
}

func newEvent() InputEvent {
	return InputEvent{
		Time:  syscall.Timeval{Sec: 1},
		Type:  1,
		Code:  30,
		Value: 1,
	}
}
