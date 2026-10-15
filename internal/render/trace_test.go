package render

import (
	"image"
	"image/color"
	"reflect"
	"testing"

	"github.com/LongTermSupport/md-viewer/internal/doc"
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

// TestTraceShift: marks are recorded in the trace's origin, which push moves and pop restores,
// link areas included (ARCHITECTURE "internal/render", Trace). A nil trace takes push and pop
// without a panic.
func TestTraceShift(t *testing.T) {
	var tr Trace
	outer := tr.push(image.Pt(10, 20))
	tr.add(Mark{Kind: MarkGlyphs, Rect: image.Rect(0, 0, 5, 5)})
	inner := tr.push(image.Pt(1, 1))
	tr.add(Mark{Kind: MarkGlyphs, Rect: image.Rect(0, 0, 5, 5)})
	tr.pop(inner)
	tr.pop(outer)
	tr.add(Mark{Kind: MarkGlyphs, Rect: image.Rect(0, 0, 5, 5)})

	want := []image.Rectangle{
		image.Rect(10, 20, 15, 25),
		image.Rect(11, 21, 16, 26),
		image.Rect(0, 0, 5, 5),
	}
	if len(tr.Marks) != len(want) {
		t.Fatalf("%d marks, want %d", len(tr.Marks), len(want))
	}
	for i, r := range want {
		if got := tr.Marks[i].Rect; got != r {
			t.Fatalf("mark %d at %v, want %v", i, got, r)
		}
	}

	var link Trace
	prev := link.push(image.Pt(10, 20))
	link.add(Mark{Kind: MarkLinkArea, Rect: image.Rect(0, 0, 5, 5), Link: &doc.Target{Kind: doc.External, Value: "https://e.org/"}})
	link.pop(prev)
	if len(link.Marks) != 1 {
		t.Fatalf("link: %d marks, want 1", len(link.Marks))
	}
	if got := link.Marks[0].Rect; got != image.Rect(10, 20, 15, 25) {
		t.Fatalf("link area at %v, want (10,20)-(15,25)", got)
	}

	var nilTrace *Trace
	nilPrev := nilTrace.push(image.Pt(1, 1))
	nilTrace.pop(nilPrev)
}
