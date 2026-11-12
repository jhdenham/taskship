package core

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestNormalizeLabels(t *testing.T) {
	got, err := NormalizeLabels([]string{" Infra", "cli", "infra", "área/ui"})
	if err != nil || !reflect.DeepEqual(got, []string{"cli", "infra", "área/ui"}) {
		t.Errorf("NormalizeLabels = %q, %v", got, err)
	}
	if got, err := NormalizeLabels(nil); err != nil || got == nil || len(got) != 0 {
		t.Errorf("NormalizeLabels(nil) = %#v, %v", got, err)
	}
	for _, bad := range []string{"", "  ", "two words", "a,b", "tab\there", strings.Repeat("x", 51)} {
		if _, err := NormalizeLabels([]string{bad}); !errors.Is(err, ErrInvalid) {
			t.Errorf("%q: err = %v, want ErrInvalid", bad, err)
		}
	}
	if _, err := NormalizeLabels([]string{strings.Repeat("é", 50)}); err != nil {
		t.Errorf("50 characters: %v", err)
	}
}
