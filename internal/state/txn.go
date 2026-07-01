package state

import (
	"errors"
	"os"

	"github.com/alan/cherry-picker/internal/lockfile"
)

// Update is the transactional primitive every writer uses. It acquires the
// exclusive writer lock, reloads the current on-disk state (so it picks up any
// changes made by another writer since this process last read the file),
// applies mutate, and saves atomically. Reloading inside the lock is what
// prevents read-modify-write clobbering between the daemon and CLI commands.
func Update(path string, mutate func(*Config) error) error {
	lk, err := lockfile.Acquire(path)
	if err != nil {
		return err
	}
	defer func() { _ = lk.Release() }()

	c, err := Load(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			c = &Config{}
		} else {
			return err
		}
	}

	if err := mutate(c); err != nil {
		return err
	}

	return Save(path, c)
}
