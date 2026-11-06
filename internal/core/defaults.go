package core

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// NameFromFolder suggests a project name from a folder or repository name:
// camelCase is split into words ("MyCoolApp" becomes "my-cool-app"), letters
// are lowercased, and other characters become hyphens. It returns "" if
// nothing usable is left.
func NameFromFolder(folder string) string {
	var b strings.Builder
	runes := []rune(folder)
	for i, r := range runes {
		// Start a word at an uppercase letter that follows a lowercase letter
		// or digit (myApp), or that ends a run of capitals before a lowercase
		// letter (HTTPServer).
		if i > 0 && unicode.IsUpper(r) {
			prev := runes[i-1]
			nextLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if unicode.IsLower(prev) || unicode.IsDigit(prev) || (unicode.IsUpper(prev) && nextLower) {
				b.WriteByte('-')
			}
		}
		b.WriteRune(unicode.ToLower(r))
	}
	name := nonNameChars.ReplaceAllString(b.String(), "-")
	name = strings.TrimLeft(name, "-")
	if len(name) > maxNameLen {
		name = name[:maxNameLen]
	}
	return strings.TrimRight(name, "-")
}

var (
	nonNameChars = regexp.MustCompile(`[^a-z0-9]+`)
	versionWord  = regexp.MustCompile(`^v?[0-9]+$`)
)

// DefaultPrefix suggests a task prefix for a valid project name.
// Numbers and versions (2024, v2) are dropped first. Several words give their
// initials, up to four; a word of up to four characters is used whole; a
// longer word gives its consonant skeleton (see skeleton). If taken reports
// the prefix as used, 2, 3, and so on are appended. A prefix on the blocklist
// is shortened until it isn't. It returns "" if no prefix starting with a
// letter can be made.
func DefaultPrefix(name string, taken func(string) bool) string {
	words := strings.Split(name, "-")
	var kept []string
	for _, w := range words {
		if w != "" && !versionWord.MatchString(w) {
			kept = append(kept, w)
		}
	}
	prefix := prefixFromWords(kept)
	if !prefixRE.MatchString(prefix) {
		// Dropping words left nothing, or a word starting with a digit.
		prefix = prefixFromWords(words)
	}
	if !prefixRE.MatchString(prefix) {
		return ""
	}
	for blockedPrefixes[prefix] {
		prefix = prefix[:len(prefix)-1]
	}
	return FreePrefix(prefix, taken)
}

// FreePrefix returns prefix, or if taken reports it as used, the first of
// prefix+"2", prefix+"3", and so on that isn't.
func FreePrefix(prefix string, taken func(string) bool) string {
	if !taken(prefix) {
		return prefix
	}
	for n := 2; ; n++ {
		if p := prefix + strconv.Itoa(n); !taken(p) {
			return p
		}
	}
}

func prefixFromWords(words []string) string {
	switch {
	case len(words) == 0:
		return ""
	case len(words) > 1:
		var initials []byte
		for _, w := range words[:min(len(words), 4)] {
			if w != "" {
				initials = append(initials, w[0])
			}
		}
		return strings.ToUpper(string(initials))
	case len(words[0]) <= 4:
		return strings.ToUpper(words[0])
	default:
		return strings.ToUpper(skeleton(words[0]))
	}
}

// skeleton returns four letters of a word's consonant skeleton, which is its
// first letter and the consonants after it (counting y and digits), with
// doubled letters collapsed: the first three and the last, so the prefix
// spans the whole word (taskship gives TSKP). If the skeleton is shorter than
// four, vowels are put back from left to right.
func skeleton(word string) string {
	var w []byte
	for i := 0; i < len(word); i++ {
		if i == 0 || word[i] != word[i-1] {
			w = append(w, word[i])
		}
	}
	keep := make([]bool, len(w))
	n := 0
	for i, c := range w {
		if i == 0 || !strings.ContainsRune("aeiou", rune(c)) {
			keep[i] = true
			n++
		}
	}
	for i := range w {
		if n >= 4 {
			break
		}
		if !keep[i] {
			keep[i] = true
			n++
		}
	}
	var out []byte
	for i, c := range w {
		if keep[i] {
			out = append(out, c)
		}
	}
	if len(out) > 4 {
		out = append(out[:3], out[len(out)-1])
	}
	return string(out)
}
