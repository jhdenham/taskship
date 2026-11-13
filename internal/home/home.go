// Package home locates Taskship's folder for user state.
package home

import (
	"os"
	"path/filepath"
)

// Dir is $TASKSHIP_HOME, or ~/.taskship by default. It isn't created here.
func Dir() (string, error) {
	if h := os.Getenv("TASKSHIP_HOME"); h != "" {
		return h, nil
	}
	h, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(h, ".taskship"), nil
}
