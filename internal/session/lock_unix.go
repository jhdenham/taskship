//go:build unix

package session

import (
	"errors"
	"os"
	"syscall"
)

// tryLock takes an exclusive lock on f without waiting, and reports whether
// it got it. The lock lasts until f is closed.
func tryLock(f *os.File) (bool, error) {
	err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return false, nil
	}
	return err == nil, err
}
