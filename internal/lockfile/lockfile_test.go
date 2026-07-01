package lockfile

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestLockMutualExclusion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.yaml")

	first, err := Acquire(path)
	require.NoError(t, err)

	acquired := make(chan struct{})
	go func() {
		second, err := Acquire(path)
		if err != nil {
			return
		}
		close(acquired)
		_ = second.Release()
	}()

	// The second Acquire must block while the first lock is held.
	select {
	case <-acquired:
		t.Fatal("second Acquire succeeded while the first lock was held")
	case <-time.After(150 * time.Millisecond):
	}

	require.NoError(t, first.Release())

	// After release, the second Acquire should proceed promptly.
	select {
	case <-acquired:
	case <-time.After(2 * time.Second):
		t.Fatal("second Acquire did not proceed after the first lock was released")
	}
}

func TestReleaseIsIdempotentOnNil(t *testing.T) {
	var l *Lock
	require.NoError(t, l.Release())
}
