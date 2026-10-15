package render

import (
	"fmt"
	"image"

	"gioui.org/layout"
	"gioui.org/op"
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
// header's, or a body row whose length is not the header's. GFM cannot give any of these (DOC-10
// pads a short row), so they come from a model built by hand.
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

// layoutTable lays out a table as a grid (STYLE-TABLE-1, STYLE-TABLE-5, STYLE-TABLE-6). Line i of
// the columns is at the sum of the widths before it, and row r's top line is the sum of the rows
// before it, each a border, 6 dp, its tallest cell and 6 dp. Each cell's text is placed 1 dp and
// 6 dp (or 13 dp across) inside its top-left corner. The lines are painted after the cells.
func layoutTable(gtx layout.Context, ctx blockCtx, t doc.Table, tr *Trace) (layout.Dimensions, error) {
	if err := checkTable(t); err != nil {
		return layout.Dimensions{}, err
	}
	width := gtx.Constraints.Max.X
	line := gtx.Dp(tableLine)
	padX := gtx.Dp(cellPadX)
	padY := gtx.Dp(cellPadY)
	cols, err := columnWidths(gtx, ctx, t)
	if err != nil {
		return layout.Dimensions{}, err
	}
	xs := make([]int, len(cols)+1) // xs[i] is the x of vertical line i; the last is the right edge
	for i, w := range cols {
		xs[i+1] = xs[i] + w
	}
	tw := xs[len(cols)] + line // the table's width, with its closing border

	tops := []int{0} // tops[r] is the top line of row r; the last is the bottom line
	y := 0
	for r := 0; r <= len(t.Rows); r++ {
		row := t.Header
		if r > 0 {
			row = t.Rows[r-1]
		}
		fp := cellFlow(ctx, r == 0)
		h := 0
		for i, c := range row {
			pt := image.Pt(xs[i]+line+padX, y+line+padY)
			cgtx := gtx
			cgtx.Constraints = layout.Constraints{Max: image.Pt(max(cols[i]-line-2*padX, 0), gtx.Constraints.Max.Y)}
			dims, err := at(cgtx, tr, pt, func() (layout.Dimensions, error) {
				return layoutFlow(cgtx, fp, c.Runs, tr)
			})
			if err != nil {
				return layout.Dimensions{}, err
			}
			h = max(h, dims.Size.Y)
		}
		y += line + 2*padY + h
		tops = append(tops, y)
	}

	bottom := y + line // the table's height, with its closing border
	for i := range xs {
		r := image.Rect(xs[i], 0, xs[i]+line, bottom)
		fillRect(gtx, r, ctx.root.Border)
		tr.add(Mark{Kind: MarkTableLine, Rect: r, Colour: ctx.root.Border})
	}
	for _, top := range tops {
		r := image.Rect(0, top, tw, top+line)
		fillRect(gtx, r, ctx.root.Border)
		tr.add(Mark{Kind: MarkTableLine, Rect: r, Colour: ctx.root.Border})
	}

	// The first baseline is the header row's text's, which sits a border and 6 dp below the top.
	f, err := newFlow(gtx, cellFlow(ctx, true), nil)
	if err != nil {
		return layout.Dimensions{}, err
	}
	size := image.Pt(max(min(tw, width), 0), bottom)
	return layout.Dimensions{Size: size, Baseline: bottom - (line + padY + f.baseOff)}, nil
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
