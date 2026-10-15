package app

import "time"

// Timings are the durations printed by --time (SPEED-2).
type Timings struct{ Read, Parse, Fonts, FirstFrame time.Duration }

// Line is the --time line without its newline (SPEED-2).
func (t Timings) Line() string { return "" }
