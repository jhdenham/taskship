package core

import "time"

// Note is a thought that isn't a task yet, kept in a project. Number is its
// position in the project's notes (#1, #2, ...); numbers of deleted notes are
// not reused. A note has no author unless an agent run wrote it.
type Note struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"project_id"`
	Number    int       `json:"number"`
	Body      string    `json:"body"`
	RunID     string    `json:"run_id,omitempty"` // the agent run that wrote it, if any
	Run       *RunRef   `json:"-"`                // names RunID for display
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Edited reports whether the note changed after it was added.
func (n Note) Edited() bool { return !n.UpdatedAt.Equal(n.CreatedAt) }

// CleanNoteBody drops leading blank lines and trailing whitespace from a note
// body and checks that something is left.
func CleanNoteBody(body string) (string, error) { return cleanBody(body, "note") }

// ParseNoteNumber parses a note number such as "3" or "#3".
func ParseNoteNumber(s string) (int, error) { return parseNumber(s, "note") }
