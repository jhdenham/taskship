package core

import (
	"errors"
	"testing"
)

func TestCleanCommentBody(t *testing.T) {
	if got, err := CleanCommentBody("\r\n\n    indented code\r\nnext  \n\n"); err != nil || got != "    indented code\nnext" {
		t.Errorf("CleanCommentBody = %q, %v", got, err)
	}
	for _, empty := range []string{"", " \n\t\n"} {
		if _, err := CleanCommentBody(empty); !errors.Is(err, ErrInvalid) {
			t.Errorf("%q: err = %v, want ErrInvalid", empty, err)
		}
	}
}

func TestParseCommentNumber(t *testing.T) {
	for in, want := range map[string]int{"2": 2, "#12": 12} {
		if n, err := ParseCommentNumber(in); err != nil || n != want {
			t.Errorf("ParseCommentNumber(%q) = %d, %v", in, n, err)
		}
	}
	for _, bad := range []string{"", "0", "-1", "#", "two", "##2"} {
		if _, err := ParseCommentNumber(bad); !errors.Is(err, ErrInvalid) {
			t.Errorf("ParseCommentNumber(%q): err = %v", bad, err)
		}
	}
}
