package tui

import "testing"

func TestFormatTokens(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{
		{0, ""},
		{-5, ""},
		{42, "42"},
		{999, "999"},
		{1000, "1K"},
		{52500, "53K"},
		{274596, "275K"},
		{999499, "999K"},
		{1_000_000, "1.0M"},
		{1_250_000, "1.2M"},
	}
	for _, c := range cases {
		if got := formatTokens(c.n); got != c.want {
			t.Errorf("formatTokens(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}
