package core

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Comment is a note on a task. Number is its position in the task's comments
// (#1, #2, ...); numbers of deleted comments are not reused.
type Comment struct {
	ID        string    `json:"id"`
	TaskID    string    `json:"task_id"`
	Number    int       `json:"number"`
	Body      string    `json:"body"` // Markdown
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Edited reports whether the comment changed after it was added.
func (c Comment) Edited() bool { return !c.UpdatedAt.Equal(c.CreatedAt) }

// CleanCommentBody drops leading blank lines and trailing whitespace from a
// comment body and checks that something is left.
func CleanCommentBody(body string) (string, error) {
	body = strings.TrimRight(strings.TrimLeft(strings.ReplaceAll(body, "\r\n", "\n"), "\n"), " \t\n")
	if strings.TrimSpace(body) == "" {
		return "", fmt.Errorf("%w: comment must not be empty", ErrInvalid)
	}
	return body, nil
}

// ParseCommentNumber parses a comment number such as "2" or "#2".
func ParseCommentNumber(s string) (int, error) {
	n, err := strconv.Atoi(strings.TrimPrefix(s, "#"))
	if err != nil || n < 1 {
		return 0, fmt.Errorf("%w: %q is not a comment number (expected a number such as 2)", ErrInvalid, s)
	}
	return n, nil
}
