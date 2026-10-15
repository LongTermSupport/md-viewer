package app

import (
	"math"
	"testing"

	"gioui.org/unit"
)

// zoomAfter presses In (up true) or Out n times from the start, 100%.
func zoomAfter(n int, up bool) Zoom {
	z := Zoom{}
	for range n {
		if up {
			z = z.In()
		} else {
			z = z.Out()
		}
	}
	return z
}

// TestZoomLevels checks ZOOM-1's levels, the limits at 300 and 50, and STYLE-ZOOM-3's metric.
func TestZoomLevels(t *testing.T) {
	if got := (Zoom{}).Percent(); got != 100 {
		t.Errorf("zero Zoom is %d%%, want 100%%", got)
	}

	var up []int
	z := Zoom{}
	for range 8 {
		z = z.In()
		up = append(up, z.Percent())
	}
	wantUp := []int{110, 125, 150, 175, 200, 250, 300, 300}
	for i := range wantUp {
		if up[i] != wantUp[i] {
			t.Errorf("In step %d is %d%%, want %d%%", i+1, up[i], wantUp[i])
		}
	}

	var down []int
	z = Zoom{}
	for range 6 {
		z = z.Out()
		down = append(down, z.Percent())
	}
	wantDown := []int{90, 80, 75, 67, 50, 50}
	for i := range wantDown {
		if down[i] != wantDown[i] {
			t.Errorf("Out step %d is %d%%, want %d%%", i+1, down[i], wantDown[i])
		}
	}

	if got := zoomAfter(6, true).Reset().Percent(); got != 100 {
		t.Errorf("Reset from 250%% is %d%%, want 100%%", got)
	}
}

// TestZoomMetric checks that Metric multiplies both the dp and the sp scale (STYLE-ZOOM-3).
func TestZoomMetric(t *testing.T) {
	tests := []struct {
		name      string
		zoom      Zoom
		in        unit.Metric
		wantScale float64
	}{
		{"zoom 100 leaves the metric alone", Zoom{}, unit.Metric{PxPerDp: 1, PxPerSp: 1}, 1},
		{"zoom 200 doubles it", zoomAfter(5, true), unit.Metric{PxPerDp: 1, PxPerSp: 1}, 2},
		{"zoom 125 scales a 1.5 metric", zoomAfter(2, true), unit.Metric{PxPerDp: 1.5, PxPerSp: 1.5}, 1.25},
		{"zoom 67 scales to 0.67", zoomAfter(4, false), unit.Metric{PxPerDp: 1, PxPerSp: 1}, 0.67},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.zoom.Metric(tt.in)
			if math.Abs(float64(got.PxPerDp)-tt.wantScale*float64(tt.in.PxPerDp)) > 1e-6 {
				t.Errorf("PxPerDp = %v, want %v", got.PxPerDp, tt.wantScale*float64(tt.in.PxPerDp))
			}
			if math.Abs(float64(got.PxPerSp)-tt.wantScale*float64(tt.in.PxPerSp)) > 1e-6 {
				t.Errorf("PxPerSp = %v, want %v", got.PxPerSp, tt.wantScale*float64(tt.in.PxPerSp))
			}
		})
	}
}
