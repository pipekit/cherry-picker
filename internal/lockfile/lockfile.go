// Package lockfile provides advisory exclusive file locking for serializing
// writers to the shared state file. Readers do not lock; they rely on atomic
// writes (see internal/state) so they never observe a torn file.
package lockfile

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// Lock represents a held advisory exclusive lock on a sidecar lock file.
type Lock struct {
	file *os.File
}

// Acquire takes an exclusive (LOCK_EX) advisory lock on "<path>.lock" and
// blocks until it is available. The lock is held until Release is called.
//
// The lock is kept on a sidecar file rather than on the state file itself:
// the state file's inode changes on every atomic save (os.Rename), which would
// invalidate a lock held on the original inode.
func Acquire(path string) (*Lock, error) {
	lockPath := path + ".lock"
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600) //nolint:gosec // lock path derived from config flag
	if err != nil {
		return nil, fmt.Errorf("failed to open lock file %s: %w", lockPath, err)
	}

	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("failed to acquire lock on %s: %w", lockPath, err)
	}

	return &Lock{file: f}, nil
}

// Release unlocks and closes the sidecar lock file. It is safe to call once.
func (l *Lock) Release() error {
	if l == nil || l.file == nil {
		return nil
	}
	// Closing the descriptor releases the flock; do it explicitly first so the
	// unlock is not deferred to GC if Close races.
	unlockErr := unix.Flock(int(l.file.Fd()), unix.LOCK_UN)
	closeErr := l.file.Close()
	l.file = nil
	if unlockErr != nil {
		return fmt.Errorf("failed to release lock: %w", unlockErr)
	}
	if closeErr != nil {
		return fmt.Errorf("failed to close lock file: %w", closeErr)
	}
	return nil
}
