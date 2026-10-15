package render

import (
	"image"
	"image/color"
	"reflect"
	"testing"
)

// TestTraceAdd: a nil Trace drops marks without a panic, and a Trace keeps them in order
// (ARCHITECTURE "internal/render": tr may be nil).
func TestTraceAdd(t *testing.T) {
	m1 := Mark{Kind: MarkGlyphs, Rect: image.Rect(0, 0, 10, 10), Text: "a"}
	m2 := Mark{Kind: MarkCodeBg, Rect: image.Rect(1, 1, 5, 5), Colour: color.NRGBA{A: 0xff}}

	var nilTrace *Trace
	nilTrace.add(m1)

	var tr Trace
	tr.add(m1)
	tr.add(m2)
	if !reflect.DeepEqual(tr.Marks, []Mark{m1, m2}) {
		t.Fatalf("Marks = %v, want [m1 m2]", tr.Marks)
	}
}
