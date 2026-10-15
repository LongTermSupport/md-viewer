package app

import (
	"testing"
	"time"
)

// TestTimingsLine checks the --time line byte for byte (SPEED-2): each value has one decimal
// place, and no row is a rounding tie. The expectations are literals, so a wrong constant fails.
func TestTimingsLine(t *testing.T) {
	cases := []struct {
		name string
		t    Timings
		want string
	}{
		{
			name: "sample",
			t:    Timings{Read: 400 * time.Microsecond, Parse: 12340 * time.Microsecond, Fonts: 6100 * time.Microsecond, FirstFrame: 88 * time.Millisecond},
			want: "read_ms=0.4 parse_ms=12.3 fonts_ms=6.1 first_frame_ms=88.0",
		},
		{
			name: "zero",
			t:    Timings{},
			want: "read_ms=0.0 parse_ms=0.0 fonts_ms=0.0 first_frame_ms=0.0",
		},
		{
			name: "above one second",
			t:    Timings{Read: 1234560 * time.Microsecond, FirstFrame: time.Second},
			want: "read_ms=1234.6 parse_ms=0.0 fonts_ms=0.0 first_frame_ms=1000.0",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.t.Line(); got != c.want {
				t.Errorf("Line() = %q, want %q", got, c.want)
			}
		})
	}
}
