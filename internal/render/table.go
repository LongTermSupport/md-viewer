package render

import (
	"fmt"
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/unit"

	"github.com/LongTermSupport/md-viewer/internal/doc"
)

// Table geometry (STYLE-TABLE-1, STYLE-TABLE-5).
const (
	cellPadX  unit.Dp = 13 // STYLE-TABLE-1: 13 dp left and right of a cell's text
	cellPadY  unit.Dp = 6  // STYLE-TABLE-1: 6 dp above and below a cell's text
	tableLine unit.Dp = 1  // STYLE-TABLE-1: the border on every cell edge, one line per edge
	minColumn unit.Dp = 64 // STYLE-TABLE-5: no column is narrower than 64 dp
)

// headerStyle is the header row's text style: the body style in the bold weight (STYLE-TABLE-2).
func headerStyle(p Palette) TextStyle {
	s := BodyStyle(p)
	s.Font.Weight = boldWeight
	return s
}

// cellFlow is the flow parameters of the cells of a row: the header style for the header row, and
// the body style for the others.
func cellFlow(ctx blockCtx, header bool) flowParams {
	if header {
		return ctx.flow(headerStyle(ctx.text), false, wrapProse)
	}
	return ctx.flow(BodyStyle(ctx.text), false, wrapProse)
}

// checkTable is an error naming a malformed table: no columns, a count of alignments other than the
// header's, a body row whose length is not the header's, or an alignment that is not one of the
// four. GFM cannot give any of these (DOC-10 pads a short row), so they come from a model built by
// hand.
func checkTable(t doc.Table) error {
	if len(t.Align) == 0 {
		return fmt.Errorf("render: table with no columns")
	}
	if len(t.Align) != len(t.Header) {
		return fmt.Errorf("render: table has %d alignments for %d header cells", len(t.Align), len(t.Header))
	}
	for r, row := range t.Rows {
		if len(row) != len(t.Header) {
			return fmt.Errorf("render: table row %d has %d cells, the header has %d", r+1, len(row), len(t.Header))
		}
	}
	for _, a := range t.Align {
		if _, err := cellAlign(a, false); err != nil {
			return err
		}
	}
	return nil
}

// textWidth is the width in px of runs laid out on one line, unwrapped (STYLE-TABLE-5). The flow
// is laid out into a scratch list of operations, so nothing is drawn and no marks are recorded.
func textWidth(gtx layout.Context, fp flowParams, runs []doc.Run) (int, error) {
	var scratch op.Ops
	mgtx := gtx
	mgtx.Ops = &scratch
	mgtx.Constraints = layout.Constraints{Max: image.Pt(unboundedWidth, gtx.Constraints.Max.Y)}
	dims, err := layoutFlow(mgtx, fp, runs, nil)
	if err != nil {
		return 0, err
	}
	return dims.Size.X, nil
}

// columnWidths is each column's width in px (STYLE-TABLE-5). A column's natural width N is its
// widest cell on one line, plus 26 dp of padding and its 1 dp left border. The table's closing
// border takes 1 dp of the width, so the columns are fitted to the width less that.
func columnWidths(gtx layout.Context, ctx blockCtx, t doc.Table) ([]int, error) {
	line := gtx.Dp(tableLine)
	padX := gtx.Dp(cellPadX)
	natural := make([]int, len(t.Align))
	rows := append([][]doc.Cell{t.Header}, t.Rows...)
	for r, row := range rows {
		fp := cellFlow(ctx, r == 0)
		for i, c := range row {
			w, err := textWidth(gtx, fp, c.Runs)
			if err != nil {
				return nil, err
			}
			natural[i] = max(natural[i], w)
		}
	}
	for i := range natural {
		natural[i] += 2*padX + line
	}
	return ColumnWidths(natural, gtx.Constraints.Max.X-line, gtx.Dp(minColumn)), nil
}

// rowColour is the fill of row r (STYLE-TABLE-2, STYLE-TABLE-3): the header is Canvas, and body row
// k (counting from 1) is BgMuted when k is even, otherwise Canvas.
func rowColour(ctx blockCtx, r int) color.NRGBA {
	if r > 0 && r%2 == 0 {
		return ctx.root.BgMuted
	}
	return ctx.root.Canvas
}

// cutWidth is r cut at x = w, the right edge of a table's content (STYLE-TABLE-5). A rectangle
// that starts at or past w is empty.
func cutWidth(r image.Rectangle, w int) image.Rectangle {
	// The fields are set directly: image.Rect would swap the corners of a rectangle that starts past w.
	return image.Rectangle{Min: r.Min, Max: image.Pt(min(r.Max.X, w), r.Max.Y)}
}

// paintCut paints r cut at x = w in colour c, and records the part painted as a mark of kind k. A
// rectangle with nothing left of w is neither painted nor recorded.
func paintCut(gtx layout.Context, tr *Trace, k MarkKind, r image.Rectangle, w int, c color.NRGBA) {
	in := cutWidth(r, w)
	if in.Empty() {
		return
	}
	fillRect(gtx, in, c)
	tr.add(Mark{Kind: k, Rect: in, Colour: c})
}

// layoutRow lays out the cells of a row whose top line is at top, each in its column's text box
// with its column's alignment (STYLE-TABLE-1, STYLE-TABLE-4). It returns the height of the tallest
// cell's text.
func layoutRow(gtx layout.Context, ctx blockCtx, aligns []doc.Align, row []doc.Cell, header bool, xs, cols []int, top int, tr *Trace) (int, error) {
	line := gtx.Dp(tableLine)
	padX := gtx.Dp(cellPadX)
	padY := gtx.Dp(cellPadY)
	fp := cellFlow(ctx, header)
	h := 0
	for i, c := range row {
		al, err := cellAlign(aligns[i], header)
		if err != nil {
			return 0, err
		}
		cfp := fp
		cfp.align = al
		pt := image.Pt(xs[i]+line+padX, top+line+padY)
		cgtx := gtx
		cgtx.Constraints = layout.Constraints{Max: image.Pt(max(cols[i]-line-2*padX, 0), gtx.Constraints.Max.Y)}
		dims, err := at(cgtx, tr, pt, func() (layout.Dimensions, error) {
			return layoutFlow(cgtx, cfp, c.Runs, tr)
		})
		if err != nil {
			return 0, err
		}
		h = max(h, dims.Size.Y)
	}
	return h, nil
}

// layoutTable lays out a table as a grid (STYLE-TABLE-1, STYLE-TABLE-5, STYLE-TABLE-6). Line i of
// the columns is at the sum of the widths before it, and row r's top line is the sum of the rows
// before it, each a border, 6 dp, its tallest cell and 6 dp. Each row's cells are recorded, its
// fill is painted, and then the cells are replayed over it, clipped to the table's width. The lines
// are painted last. Nothing is drawn beyond the width it is given, and the lines and fills that
// are recorded are cut there; glyph and link marks of a clipped column stay in the trace, and the
// clip blocks their clicks.
func layoutTable(gtx layout.Context, ctx blockCtx, t doc.Table, tr *Trace) (layout.Dimensions, error) {
	if err := checkTable(t); err != nil {
		return layout.Dimensions{}, err
	}
	width := gtx.Constraints.Max.X
	line := gtx.Dp(tableLine)
	padY := gtx.Dp(cellPadY)
	cols, err := columnWidths(gtx, ctx, t)
	if err != nil {
		return layout.Dimensions{}, err
	}
	xs := make([]int, len(cols)+1) // xs[i] is the x of vertical line i; the last is the right edge
	for i, w := range cols {
		xs[i+1] = xs[i] + w
	}
	tw := xs[len(cols)] + line      // the table's width, with its closing border
	clipW := max(min(tw, width), 0) // the table's content is clipped at the width it is given

	tops := []int{0} // tops[r] is the top line of row r; the last is the bottom line
	y := 0
	for r := 0; r <= len(t.Rows); r++ {
		row, header := t.Header, r == 0
		if !header {
			row = t.Rows[r-1]
		}
		top := y
		rec := op.Record(gtx.Ops)
		h, err := layoutRow(gtx, ctx, t.Align, row, header, xs, cols, top, tr)
		call := rec.Stop()
		if err != nil {
			return layout.Dimensions{}, err
		}
		y = top + line + 2*padY + h
		tops = append(tops, y)
		paintCut(gtx, tr, MarkRowFill, image.Rect(0, top+line, tw, y), clipW, rowColour(ctx, r))
		stack := clip.Rect(image.Rect(0, top, clipW, y)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		stack.Pop()
	}

	bottom := y + line // the table's height, with its closing border
	for i := range xs {
		paintCut(gtx, tr, MarkTableLine, image.Rect(xs[i], 0, xs[i]+line, bottom), clipW, ctx.root.Border)
	}
	for _, top := range tops {
		paintCut(gtx, tr, MarkTableLine, image.Rect(0, top, tw, top+line), clipW, ctx.root.Border)
	}

	// The first baseline is the header row's text's, which sits a border and 6 dp below the top.
	f, err := newFlow(gtx, cellFlow(ctx, true), nil)
	if err != nil {
		return layout.Dimensions{}, err
	}
	return layout.Dimensions{Size: image.Pt(clipW, bottom), Baseline: bottom - (line + padY + f.baseOff)}, nil
}

// cellAlign is how the lines of a column's cells sit (STYLE-TABLE-4): Left and a body None are at
// the start, Centre and a header None are centred, and Right is at the end. An alignment that is
// not one of the four is an error that names it.
func cellAlign(a doc.Align, header bool) (textAlign, error) {
	switch a {
	case doc.AlignLeft:
		return alignStart, nil
	case doc.AlignCentre:
		return alignCentre, nil
	case doc.AlignRight:
		return alignEnd, nil
	case doc.AlignNone:
		if header {
			return alignCentre, nil
		}
		return alignStart, nil
	default:
		return alignStart, fmt.Errorf("render: no cell alignment for alignment %d", a)
	}
}
