package core

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

const maxLabelLen = 50

// NormalizeLabels validates labels and returns them lowercased, sorted and
// without duplicates. A label is free-form text of at most 50 characters
// without whitespace or commas. The result is never nil.
func NormalizeLabels(labels []string) ([]string, error) {
	out := []string{}
	for _, l := range labels {
		l = strings.ToLower(strings.TrimSpace(l))
		switch {
		case l == "":
			return nil, fmt.Errorf("%w: labels must not be empty", ErrInvalid)
		case strings.ContainsFunc(l, func(r rune) bool { return unicode.IsSpace(r) || r == ',' }):
			return nil, fmt.Errorf("%w: label %q must not contain spaces or commas", ErrInvalid, l)
		case utf8.RuneCountInString(l) > maxLabelLen:
			return nil, fmt.Errorf("%w: label %q is longer than %d characters", ErrInvalid, l, maxLabelLen)
		}
		out = append(out, l)
	}
	slices.Sort(out)
	return slices.Compact(out), nil
}
