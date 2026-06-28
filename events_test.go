package evdev

import (
	"encoding/binary"
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
