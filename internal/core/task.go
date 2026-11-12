package core

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Status is a task's workflow status. Archiving is separate (Task.ArchivedAt).
type Status string

const (
	StatusBacklog    Status = "backlog"
	StatusTodo       Status = "todo"
	StatusInProgress Status = "in_progress"
	StatusDone       Status = "done"
	StatusCanceled   Status = "canceled"
)

// Statuses lists all statuses in workflow order.
var Statuses = []Status{StatusBacklog, StatusTodo, StatusInProgress, StatusDone, StatusCanceled}

// Priority is a task's priority; PriorityNone means unset.
type Priority string

const (
	PriorityNone   Priority = "none"
	PriorityLow    Priority = "low"
	PriorityMedium Priority = "medium"
	PriorityHigh   Priority = "high"
	PriorityUrgent Priority = "urgent"
)

// Priorities lists all priorities from lowest to highest.
var Priorities = []Priority{PriorityNone, PriorityLow, PriorityMedium, PriorityHigh, PriorityUrgent}

// Task is a Taskship-owned task. Ref (e.g. PROJ-42) is derived from the
// project's task prefix and the task's sequence number.
type Task struct {
	ID          string     `json:"id"`
	Ref         string     `json:"ref"`
	ProjectID   string     `json:"project_id"`
	Project     string     `json:"project"` // project name, for display
	Sequence    int        `json:"sequence_number"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Status      Status     `json:"status"`
	Priority    Priority   `json:"priority"`
	Labels      []string   `json:"labels"` // normalized (see NormalizeLabels), never nil from the store
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	ArchivedAt  *time.Time `json:"archived_at,omitempty"`
	// CompletedAt is when the task last moved to done; it is cleared when the
	// task leaves done. CompletedRunID is the agent run that did it, if any.
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
	CompletedRunID string     `json:"completed_run_id,omitempty"`
	CompletedBy    *RunRef    `json:"-"` // names CompletedRunID for display
}

// ParseStatus validates a status name.
func ParseStatus(s string) (Status, error) {
	for _, st := range Statuses {
		if string(st) == s {
			return st, nil
		}
	}
	return "", fmt.Errorf("%w: unknown status %q (valid: %s)", ErrInvalid, s, join(Statuses))
}

// ParsePriority validates a priority name.
func ParsePriority(s string) (Priority, error) {
	for _, p := range Priorities {
		if string(p) == s {
			return p, nil
		}
	}
	return "", fmt.Errorf("%w: unknown priority %q (valid: %s)", ErrInvalid, s, join(Priorities))
}

// ValidateTask checks the user-supplied fields of a new or edited task.
func ValidateTask(t Task) error {
	if strings.TrimSpace(t.Title) == "" {
		return fmt.Errorf("%w: title must not be empty", ErrInvalid)
	}
	if strings.ContainsAny(t.Title, "\r\n") {
		return fmt.Errorf("%w: title must be a single line", ErrInvalid)
	}
	if _, err := ParseStatus(string(t.Status)); err != nil {
		return err
	}
	_, err := ParsePriority(string(t.Priority))
	return err
}

var taskRefRE = regexp.MustCompile(`^([A-Z][A-Z0-9]{0,9})-([1-9][0-9]{0,9})$`)

// ParseTaskRef splits a ref such as PROJ-42 (case-insensitive) into its
// upper-cased prefix and sequence number.
func ParseTaskRef(ref string) (prefix string, seq int, err error) {
	m := taskRefRE.FindStringSubmatch(strings.ToUpper(ref))
	if m == nil {
		return "", 0, fmt.Errorf("%w: %q is not a task reference (expected PREFIX-NUMBER, e.g. PROJ-42)", ErrInvalid, ref)
	}
	seq, err = strconv.Atoi(m[2])
	if err != nil {
		return "", 0, fmt.Errorf("%w: task number in %q is too large", ErrInvalid, ref)
	}
	return m[1], seq, nil
}

// FormatTaskRef returns the display ref for a task, e.g. PROJ-42.
func FormatTaskRef(prefix string, seq int) string {
	return fmt.Sprintf("%s-%d", prefix, seq)
}

func join[T ~string](values []T) string {
	s := make([]string, len(values))
	for i, v := range values {
		s[i] = string(v)
	}
	return strings.Join(s, ", ")
}
