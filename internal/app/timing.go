package app

import (
	"fmt"
	"strconv"
	"time"
)

// timingsFormat is the --time line: four keys, each with a value of one decimal place in
// milliseconds (SPEED-2).
const timingsFormat = "read_ms=%s parse_ms=%s fonts_ms=%s first_frame_ms=%s"

// Timings are the durations printed by --time (SPEED-2).
type Timings struct{ Read, Parse, Fonts, FirstFrame time.Duration }

// Line is the --time line without its newline (SPEED-2).
func (t Timings) Line() string {
	return fmt.Sprintf(timingsFormat, millis(t.Read), millis(t.Parse), millis(t.Fonts), millis(t.FirstFrame))
}

// millis formats d in milliseconds with one decimal place (SPEED-2).
func millis(d time.Duration) string {
	return strconv.FormatFloat(float64(d)/float64(time.Millisecond), 'f', 1, 64)
}
