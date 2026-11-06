// Package core defines Taskship's domain types and validation rules,
// independent of storage backend and presentation.
package core

import (
	"errors"
	"fmt"
	"regexp"
	"time"
)

// Error kinds. Wrap these so callers can classify failures with errors.Is.
var (
	ErrInvalid   = errors.New("invalid input")
	ErrNotFound  = errors.New("not found")
	ErrAmbiguous = errors.New("ambiguous")
)

// Project groups tasks. Name identifies it (e.g. --project myproject) and
// TaskPrefix forms task refs such as PROJ-42.
type Project struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	TaskPrefix  string     `json:"task_prefix"`
	Description string     `json:"description"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	ArchivedAt  *time.Time `json:"archived_at,omitempty"`
}

var (
	nameRE   = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)
	prefixRE = regexp.MustCompile(`^[A-Z][A-Z0-9]{0,9}$`)
)

const maxNameLen = 40

// ValidateProject checks the user-supplied fields of a new or edited project.
func ValidateProject(p Project) error {
	if len(p.Name) > maxNameLen || !nameRE.MatchString(p.Name) {
		return fmt.Errorf("%w: name %q must be lowercase letters, digits, and single hyphens, start with a letter, and be at most %d characters", ErrInvalid, p.Name, maxNameLen)
	}
	if !prefixRE.MatchString(p.TaskPrefix) {
		return fmt.Errorf("%w: task prefix %q must be 1-10 uppercase letters or digits, starting with a letter", ErrInvalid, p.TaskPrefix)
	}
	return nil
}
