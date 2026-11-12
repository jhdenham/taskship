package core

import (
	"errors"
	"testing"
)

func TestParseTaskRef(t *testing.T) {
	good := []struct {
		in     string
		prefix string
		seq    int
	}{
		{"PROJ-42", "PROJ", 42},
		{"proj-1", "PROJ", 1},
		{"A1-7", "A1", 7},
	}
	for _, tt := range good {
		prefix, seq, err := ParseTaskRef(tt.in)
		if err != nil || prefix != tt.prefix || seq != tt.seq {
			t.Errorf("ParseTaskRef(%q) = %q, %d, %v; want %q, %d", tt.in, prefix, seq, err, tt.prefix, tt.seq)
		}
	}
	for _, in := range []string{"", "PROJ", "PROJ-", "PROJ-0", "PROJ-01", "-1", "1-1", "PROJ-1a", "PR J-1", "ABCDEFGHIJK-1", "PROJ-99999999999", "taks"} {
		if _, _, err := ParseTaskRef(in); !errors.Is(err, ErrInvalid) {
			t.Errorf("ParseTaskRef(%q) err = %v, want ErrInvalid", in, err)
		}
	}
	if got := FormatTaskRef("PROJ", 42); got != "PROJ-42" {
		t.Errorf("FormatTaskRef = %q", got)
	}
}

func TestValidateTask(t *testing.T) {
	valid := Task{Title: "Fix bug", Status: StatusTodo, Priority: PriorityNone}
	if err := ValidateTask(valid); err != nil {
		t.Fatalf("valid task: %v", err)
	}
	tests := map[string]func(*Task){
		"empty title":      func(t *Task) { t.Title = "  " },
		"multiline title":  func(t *Task) { t.Title = "a\nb" },
		"unknown status":   func(t *Task) { t.Status = "open" },
		"empty status":     func(t *Task) { t.Status = "" },
		"unknown priority": func(t *Task) { t.Priority = "p1" },
		"comments marker":  func(t *Task) { t.Description = "Body\n  " + CommentsMarker + "\n## Comments" },
	}
	mentioned := valid
	mentioned.Description = "The marker is " + CommentsMarker + " on its own line."
	if err := ValidateTask(mentioned); err != nil {
		t.Errorf("marker inside a line: %v", err)
	}
	for name, mutate := range tests {
		task := valid
		mutate(&task)
		if err := ValidateTask(task); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", name, err)
		}
	}
}
