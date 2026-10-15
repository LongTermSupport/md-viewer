package app

import "gioui.org/unit"

// Zoom is the zoom level of the viewer (ZOOM-1). Stub: not yet implemented.
type Zoom struct {
	step int
}

// Percent is the zoom in per cent. Stub: returns 0.
func (z Zoom) Percent() int {
	return 0
}

// In is the next higher level. Stub: returns z.
func (z Zoom) In() Zoom {
	return z
}

// Out is the next lower level. Stub: returns z.
func (z Zoom) Out() Zoom {
	return z
}

// Reset is 100%. Stub: returns z.
func (z Zoom) Reset() Zoom {
	return z
}

// Metric scales m by the zoom (STYLE-ZOOM-3). Stub: returns m.
func (z Zoom) Metric(m unit.Metric) unit.Metric {
	return m
}
