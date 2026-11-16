package core

import (
	"regexp"
	"strconv"
)

var release = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)(-[^+]*)?`)

// VersionNewer reports whether version a is a later release than b. A
// release is later than a pre-release of the same version, such as v0.2.0
// and v0.2.0-rc.1; two pre-releases of the same version aren't ordered.
// Versions that aren't releases, such as "dev", are never newer.
func VersionNewer(a, b string) bool {
	ma, mb := release.FindStringSubmatch(a), release.FindStringSubmatch(b)
	if ma == nil || mb == nil {
		return false
	}
	for i := 1; i <= 3; i++ {
		x, _ := strconv.Atoi(ma[i])
		y, _ := strconv.Atoi(mb[i])
		if x != y {
			return x > y
		}
	}
	return ma[4] == "" && mb[4] != ""
}
