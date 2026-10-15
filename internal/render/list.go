package render

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"

	"github.com/LongTermSupport/md-viewer/internal/doc"
)

// Markers (STYLE-LIST-2, STYLE-LIST-3, STYLE-LIST-5).
const (
	bulletSize   unit.Dp = 5   // STYLE-LIST-2: 5 dp across
	bulletOffset unit.Dp = 11  // STYLE-LIST-2: the centre is 11 dp left of the content edge
	bulletRise   float64 = 0.27 // STYLE-LIST-2: the centre is 0.27 em above the baseline
	bulletStroke unit.Dp = 1   // STYLE-LIST-2: the outline of a circle, 1 dp inside its diameter
	numberGap    unit.Dp = 4   // STYLE-LIST-3: the number's right edge is 4 dp left of the content edge
)

// clipArea is an area that a clip operation can push, such as a rectangle, an ellipse or a stroke.
type clipArea interface {
	Push(ops *op.Ops) clip.Stack
}

// List spacing (STYLE-LIST-1, STYLE-LIST-4).
const (
	listIndent   unit.Dp = 32 // STYLE-LIST-1: an item's content is 2em from the list's left edge
	itemGapTight unit.Dp = 4  // STYLE-LIST-4: consecutive items of a tight list, .25em apart
	itemGapLoose unit.Dp = 16 // STYLE-LIST-4: consecutive items of a loose list, 1rem apart
)

// itemGap is the space between consecutive items of a list (STYLE-LIST-4).
func itemGap(tight bool) unit.Dp {
	if tight {
		return itemGapTight
	}
	return itemGapLoose
}

// layoutList lays out a list's items one under another, with itemGap between them. Each item's
// content is indented by listIndent from the list's left edge (STYLE-LIST-1). The list's baseline
// is its first item's first baseline.
func layoutList(gtx layout.Context, ctx blockCtx, l doc.List, tr *Trace) (layout.Dimensions, error) {
	width := gtx.Constraints.Max.X
	gap := gtx.Dp(itemGap(l.Tight))
	y := 0
	firstTop := 0 // the first item's first baseline, from the top of the list
	for i := range l.Items {
		if i > 0 {
			y += gap
		}
		dims, err := at(gtx, tr, image.Pt(0, y), func() (layout.Dimensions, error) {
			return layoutItem(gtx, ctx, l, i, tr)
		})
		if err != nil {
			return layout.Dimensions{}, err
		}
		if i == 0 {
			firstTop = dims.Size.Y - dims.Baseline
		}
		y += dims.Size.Y
	}
	return layout.Dimensions{Size: image.Pt(width, y), Baseline: y - firstTop}, nil
}

// layoutItem lays out item n of list l at the list's origin. The content is indented by listIndent,
// and laid out one list deeper, and tight when the list is (STYLE-LIST-4). An item with no blocks
// is one empty paragraph (DOC-8, STYLE-LIST-2). Its Baseline is that of its marker.
func layoutItem(gtx layout.Context, ctx blockCtx, l doc.List, n int, tr *Trace) (layout.Dimensions, error) {
	width := gtx.Constraints.Max.X
	indent := gtx.Dp(listIndent)
	igtx := gtx
	igtx.Constraints = layout.Constraints{Max: image.Pt(max(width-indent, 0), gtx.Constraints.Max.Y)}
	ictx := ctx
	ictx.depth++
	ictx.tight = l.Tight
	blocks := l.Items[n].Blocks
	if len(blocks) == 0 {
		blocks = []doc.Block{doc.Paragraph{}}
	}
	stack, err := at(igtx, tr, image.Pt(indent, 0), func() (layout.Dimensions, error) {
		return layoutStack(igtx, ictx, blocks, tr)
	})
	if err != nil {
		return layout.Dimensions{}, err
	}
	base, err := markerBaseline(gtx, ctx, blocks[0], stack)
	if err != nil {
		return layout.Dimensions{}, err
	}
	if err := drawMarker(gtx, ctx, l, n, base, tr); err != nil {
		return layout.Dimensions{}, err
	}
	return layout.Dimensions{Size: image.Pt(width, stack.Size.Y), Baseline: stack.Size.Y - base}, nil
}

// markerBaseline is the first baseline of an item, from the item's top. When the item's first
// block leads with text, it is that line's, as laid out in stack. Otherwise it is the first
// baseline of an empty body line, which adds no height (STYLE-LIST-2).
func markerBaseline(gtx layout.Context, ctx blockCtx, first doc.Block, stack layout.Dimensions) (int, error) {
	text, err := leadsWithText(first)
	if err != nil {
		return 0, err
	}
	if text {
		return stack.Size.Y - stack.Baseline, nil
	}
	f, err := newFlow(gtx, ctx.flow(BodyStyle(ctx.text), false, wrapProse), nil)
	if err != nil {
		return 0, err
	}
	return f.baseOff, nil
}

// drawMarker draws the marker of item n of list l, whose first baseline is base from the item's
// top. The marker sits at the content edge's left, in the list's text colour (STYLE-LIST-2). An
// ordered or unordered item is marked with its number or bullet, and a task item with its box
// only (STYLE-LIST-5).
func drawMarker(gtx layout.Context, ctx blockCtx, l doc.List, n, base int, tr *Trace) error {
	switch task := l.Items[n].Task; task {
	case doc.NotTask:
		if l.Ordered {
			return drawNumber(gtx, ctx, gtx.Dp(listIndent), base, OrderedMarker(l.Start+n, ctx.depth), tr)
		}
		return drawBullet(gtx, ctx, gtx.Dp(listIndent), base, BulletShape(ctx.depth), tr)
	case doc.Open, doc.Done:
		return nil
	default:
		return fmt.Errorf("render: no marker for task state %d", task)
	}
}

// drawNumber draws the number s of an ordered item, right-aligned numberGap left of the content
// edge at edge and on the baseline at base (STYLE-LIST-3). The number is laid out unwrapped in the
// body style, recorded, and replayed at its place. Each glyph run it records becomes a Number mark.
func drawNumber(gtx layout.Context, ctx blockCtx, edge, base int, s string, tr *Trace) error {
	fp := ctx.flow(BodyStyle(ctx.text), false, wrapProse)
	scratch := &Trace{}
	ngtx := gtx
	ngtx.Constraints = layout.Constraints{Max: image.Pt(unboundedWidth, gtx.Constraints.Max.Y)}
	rec := op.Record(gtx.Ops)
	dims, err := layoutFlow(ngtx, fp, []doc.Run{{Kind: doc.TextRun, Text: s}}, scratch)
	call := rec.Stop()
	if err != nil {
		return err
	}
	off := image.Pt(edge-gtx.Dp(numberGap)-dims.Size.X, base-(dims.Size.Y-dims.Baseline))
	stack := op.Offset(off).Push(gtx.Ops)
	call.Add(gtx.Ops)
	stack.Pop()
	for _, m := range marks(scratch, MarkGlyphs) {
		tr.add(Mark{Kind: MarkNumber, Rect: m.Rect.Add(off), Colour: m.Colour, Text: m.Text})
	}
	return nil
}

// drawBullet draws an unordered marker of shape (STYLE-LIST-2). Its centre is bulletOffset left of
// the content edge at edge and bulletRise em above the baseline at base. Its rectangle has its
// top-left corner at the rounded centre less half its size, so it is centred to within half a
// pixel.
func drawBullet(gtx layout.Context, ctx blockCtx, edge, base int, shape Bullet, tr *Trace) error {
	size := gtx.Dp(bulletSize)
	cx := float64(edge) - dpPx(gtx, bulletOffset)
	cy := float64(base) - bulletRise*pxOf(gtx, bodySize)
	left := int(math.Round(cx - float64(size)/2))
	top := int(math.Round(cy - float64(size)/2))
	r := image.Rect(left, top, left+size, top+size)
	switch shape {
	case BulletDisc:
		fillClip(gtx, clip.Ellipse(r), ctx.text.Fg)
	case BulletCircle:
		strokeCircle(gtx, r, ctx.text.Fg)
	case BulletSquare:
		fillRect(gtx, r, ctx.text.Fg)
	default:
		return fmt.Errorf("render: no bullet drawn for %s", shape)
	}
	tr.add(Mark{Kind: MarkBullet, Rect: r, Colour: ctx.text.Fg, Text: shape.String()})
	return nil
}

// strokeCircle draws the outline of a circle that fills r (STYLE-LIST-2). The stroke lies inside
// r, so the outer diameter is r's side and the path's radius is half of that less half the stroke.
func strokeCircle(gtx layout.Context, r image.Rectangle, c color.NRGBA) {
	w := dpPx(gtx, bulletStroke)
	radius := float64(r.Dx())/2 - w/2
	cx := float64(r.Min.X) + float64(r.Dx())/2
	cy := float64(r.Min.Y) + float64(r.Dy())/2
	var p clip.Path
	p.Begin(gtx.Ops)
	p.MoveTo(f32.Pt(float32(cx+radius), float32(cy)))
	p.Arc(f32.Pt(float32(-radius), 0), f32.Pt(float32(-radius), 0), 2*math.Pi)
	fillClip(gtx, clip.Stroke{Path: p.End(), Width: float32(w)}.Op(), c)
}

// fillClip fills the area that a is pushed as, with colour c.
func fillClip(gtx layout.Context, a clipArea, c color.NRGBA) {
	stack := a.Push(gtx.Ops)
	paint.ColorOp{Color: c}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	stack.Pop()
}

// leadsWithText reports whether a block's first line is text, found through quotes and nested
// lists (STYLE-LIST-2). An empty item is an empty line of text. A rule, a table or an image has
// no text line, and an empty quote or list has none either. A block with no layout is an error
// naming its type.
func leadsWithText(b doc.Block) (bool, error) {
	switch b := b.(type) {
	case doc.Paragraph, doc.Heading, doc.CodeBlock:
		return true, nil
	case doc.Quote:
		if len(b.Blocks) == 0 {
			return false, nil
		}
		return leadsWithText(b.Blocks[0])
	case doc.List:
		if len(b.Items) == 0 {
			return false, nil
		}
		first := b.Items[0]
		if len(first.Blocks) == 0 {
			return true, nil
		}
		return leadsWithText(first.Blocks[0])
	case doc.Rule, doc.Table, doc.Image:
		return false, nil
	default:
		return false, fmt.Errorf("render: no first line for block %T", b)
	}
}
