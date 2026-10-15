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

// TestTraceShiftFrom: shiftFrom moves the marks from an index on by a offset and leaves the marks
// before it, as a centred line's marks move with the line (STYLE-TABLE-4). A nil trace has no
// marks: shiftFrom does nothing and count is 0.
func TestTraceShiftFrom(t *testing.T) {
	var tr Trace
	for i := 0; i < 3; i++ {
		tr.add(Mark{Kind: MarkGlyphs, Rect: image.Rect(i*10, 0, i*10+5, 5)})
	}
	tr.shiftFrom(1, image.Pt(5, 0))
	want := []image.Rectangle{
		image.Rect(0, 0, 5, 5),
		image.Rect(15, 0, 20, 5),
		image.Rect(25, 0, 30, 5),
	}
	if len(tr.Marks) != len(want) {
		t.Fatalf("%d marks, want %d", len(tr.Marks), len(want))
	}
	for i, r := range want {
		if got := tr.Marks[i].Rect; got != r {
			t.Fatalf("mark %d at %v, want %v", i, got, r)
		}
	}
	if n := tr.count(); n != 3 {
		t.Fatalf("count() = %d, want 3", n)
	}

	var nilTrace *Trace
	nilTrace.shiftFrom(0, image.Pt(1, 1))
	if n := nilTrace.count(); n != 0 {
		t.Fatalf("nil trace: count() = %d, want 0", n)
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
