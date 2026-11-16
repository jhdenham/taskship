//go:build !unix

package session

import (
	"errors"
	"os"
)

// tryLock isn't supported here, so no session is ever recorded.
func tryLock(*os.File) (bool, error) {
	return false, errors.ErrUnsupported
}
