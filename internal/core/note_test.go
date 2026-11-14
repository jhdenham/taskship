package core

import (
	"errors"
	"strings"
	"testing"
)

func TestCleanNoteBody(t *testing.T) {
	if got, err := CleanNoteBody("\n\nTry a compact board layout  \n"); err != nil || got != "Try a compact board layout" {
		t.Errorf("CleanNoteBody = %q, %v", got, err)
	}
	if _, err := CleanNoteBody(" \n"); !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), "note must not be empty") {
		t.Errorf("empty note: err = %v", err)
	}
}

func TestParseNoteNumber(t *testing.T) {
	if n, err := ParseNoteNumber("#3"); err != nil || n != 3 {
		t.Errorf("ParseNoteNumber = %d, %v", n, err)
	}
	if _, err := ParseNoteNumber("x"); !errors.Is(err, ErrInvalid) || !strings.Contains(err.Error(), "note number") {
		t.Errorf("bad number: err = %v", err)
	}
}
