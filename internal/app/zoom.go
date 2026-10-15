package app

import "gioui.org/unit"

// zoomLevels are the zoom levels in per cent, in order (ZOOM-1).
var zoomLevels = [...]int{50, 67, 75, 80, 90, 100, 110, 125, 150, 175, 200, 250, 300}

// zoomDefault is the index in zoomLevels of 100%, the level the viewer starts at (ZOOM-1).
const zoomDefault = 5

// Zoom is the zoom level, as a number of steps from 100% (ZOOM-1). The zero value is 100%.
type Zoom struct {
	step int
}

// index is the position of z in zoomLevels. The steps never leave the table (In and Out).
func (z Zoom) index() int {
	return zoomDefault + z.step
}

// Percent is the zoom in per cent.
func (z Zoom) Percent() int {
	return zoomLevels[z.index()]
}

// In is the next higher level; at 300% it is 300% again (ZOOM-2).
func (z Zoom) In() Zoom {
	if z.index() < len(zoomLevels)-1 {
		z.step++
	}
	return z
}

// Out is the next lower level; at 50% it is 50% again (ZOOM-2).
func (z Zoom) Out() Zoom {
	if z.index() > 0 {
		z.step--
	}
	return z
}

// Reset is 100% (ZOOM-2).
func (z Zoom) Reset() Zoom {
	return Zoom{}
}

// Metric scales the dp and sp of m by the zoom (STYLE-ZOOM-3).
func (z Zoom) Metric(m unit.Metric) unit.Metric {
	f := float32(z.Percent()) / 100
	m.PxPerDp *= f
	m.PxPerSp *= f
	return m
}
