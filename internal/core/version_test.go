package core

import "testing"

func TestVersionNewer(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"v0.2.0", "v0.1.9", true},
		{"v0.10.0", "v0.9.0", true},
		{"v0.1.0", "v0.1.0", false},
		{"v0.1.0", "v0.2.0", false},
		{"v0.2.0", "v0.2.0-rc.1", true},
		{"v0.2.0-rc.1", "v0.2.0", false},
		{"v0.2.0-rc.1", "v0.1.0", true},
		{"v0.2.0-rc.2", "v0.2.0-rc.1", false},
		{"v0.2.0+meta", "v0.2.0", false},
		{"dev", "v0.1.0", false},
		{"v0.1.0", "dev", false},
	}
	for _, c := range cases {
		if got := VersionNewer(c.a, c.b); got != c.want {
			t.Errorf("VersionNewer(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}
