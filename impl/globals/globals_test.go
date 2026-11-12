package globals

import "testing"

// Tests choosing the later of two date/times, with empty earlier than any value.
func TestLaterTime(t *testing.T) {
	a, b := "2026-11-09T17:09:25", "2026-11-10T08:41:02"
	for _, tst := range [][3]string{{a, b, b}, {b, a, b}, {a, a, a}, {"", a, a}, {a, "", a}, {"", "", ""}} {
		if got := LaterTime(tst[0], tst[1]); got != tst[2] {
			t.Errorf("LaterTime(%q, %q): expected %q, got %q", tst[0], tst[1], tst[2], got)
		}
	}
}
