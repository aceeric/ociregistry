package globals

import (
	"strings"
	"testing"
	"time"
)

// est is a fixed zone for the tests, so they don't depend on the time zone of the test machine
// or on its time zone database.
var est = time.FixedZone("EST", -5*60*60)

// setLocal sets the local time zone for the duration of the test.
func setLocal(t *testing.T, loc *time.Location) {
	saved := time.Local
	time.Local = loc
	t.Cleanup(func() { time.Local = saved })
}

// Tests that CurTime is UTC with a Z suffix whatever the local time zone, and that ParseTime
// reads it back but rejects any other format.
func TestCurTimeIsUtc(t *testing.T) {
	setLocal(t, est)
	now := CurTime()
	parsed, err := ParseTime(now)
	if err != nil || !strings.HasSuffix(now, "Z") || parsed.Location() != time.UTC || time.Since(parsed) > time.Minute || time.Since(parsed) < -time.Second {
		t.Fatalf("expected the current UTC time, got %q %v %v", now, parsed, err)
	}
	for _, dt := range []string{"2026-11-14T12:00:00", "2026-11-14T12:00:00-05:00", "", "foobar"} {
		if _, err := ParseTime(dt); err == nil {
			t.Errorf("%q: expected an error", dt)
		}
	}
}

// Tests the legacy date conversion: a legacy date is read in the local time zone, a UTC or
// empty date is unchanged, and anything else is an error.
func TestLegacyToUtc(t *testing.T) {
	setLocal(t, est)
	tests := []struct {
		dt      string
		want    string
		changed bool
		err     bool
	}{
		{"2026-11-14T12:00:00", "2026-11-14T17:00:00Z", true, false},
		{"2026-12-31T22:30:00", "2027-01-01T03:30:00Z", true, false},
		{"2026-11-14T12:00:00Z", "2026-11-14T12:00:00Z", false, false},
		{"", "", false, false},
		{"foobar", "", false, true},
		{"2026-11-14T12:00:00-05:00", "", false, true},
	}
	for _, tst := range tests {
		got, changed, err := LegacyToUtc(tst.dt)
		if got != tst.want || changed != tst.changed || (err != nil) != tst.err {
			t.Errorf("%q: expected %q %t err=%t, got %q %t %v", tst.dt, tst.want, tst.changed, tst.err, got, changed, err)
		}
	}
	// in UTC, as in a container with no time zone, a legacy date only gains the Z
	setLocal(t, time.UTC)
	if got, _, _ := LegacyToUtc("2026-11-14T12:00:00"); got != "2026-11-14T12:00:00Z" {
		t.Errorf("expected the same time with a Z, got %q", got)
	}
}

// Tests IsUtc.
func TestIsUtc(t *testing.T) {
	for dt, want := range map[string]bool{"": true, "2026-11-14T12:00:00Z": true, "2026-11-14T12:00:00": false, "foobar": false} {
		if IsUtc(dt) != want {
			t.Errorf("%q: expected %t", dt, want)
		}
	}
}
