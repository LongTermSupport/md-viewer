package render

import (
	"fmt"
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"

	"github.com/LongTermSupport/md-viewer/internal/doc"
)

// blockCtx is what a block is laid out with. Its text is drawn in text, and its rules, bars and
// code blocks in root. Inside a quote the text is muted and root is not (STYLE-QUOTE-1).
type blockCtx struct {
	text   Palette
	root   Palette
	shaper *text.Shaper
	links  *links
}

// quoteCtx is the context of a quote's children: their text is FgMuted (STYLE-QUOTE-1).
func quoteCtx(c blockCtx) blockCtx {
	c.text.Fg = c.root.FgMuted
	return c
}

// flow is the flow parameters for text laid out in this context.
func (c blockCtx) flow(base TextStyle, inHeading bool, wrap wrapMode) flowParams {
	return flowParams{
		shaper:    c.shaper,
		base:      base,
		palette:   c.text,
		inHeading: inHeading,
		links:     c.links,
		wrap:      wrap,
	}
}

// Quote sizes in dp (STYLE-QUOTE-1).
const (
	quoteBarWidth unit.Dp = 4  // STYLE-QUOTE-1: .25em of 16, the bar's width
	quotePadding  unit.Dp = 16 // STYLE-QUOTE-1: the padding after the bar, and on the right
)

// at lays out fn with its ops and its trace offset by pt, so that the marks fn records are in the
// coordinates of the block that contains it. Both are moved and restored together.
func at(gtx layout.Context, tr *Trace, pt image.Point, fn func() (layout.Dimensions, error)) (layout.Dimensions, error) {
	stack := op.Offset(pt).Push(gtx.Ops)
	prev := tr.push(pt)
	dims, err := fn()
	tr.pop(prev)
	stack.Pop()
	return dims, err
}

// blockGap is the space between blocks prev and next: the larger of prev's bottom margin and
// next's top margin (DOC-LAYOUT-3).
func blockGap(prev, next doc.Block) (unit.Dp, error) {
	_, prevBottom, err := Margins(prev, false)
	if err != nil {
		return 0, err
	}
	nextTop, _, err := Margins(next, false)
	if err != nil {
		return 0, err
	}
	return Gap(prevBottom, nextTop), nil
}

// layoutBlock lays out one block at the width of gtx, and records its marks in tr. A block with
// no layout is an error, and so is a Heading level that HeadingStyle rejects.
func layoutBlock(gtx layout.Context, ctx blockCtx, b doc.Block, tr *Trace) (layout.Dimensions, error) {
	width := gtx.Constraints.Max.X
	switch b := b.(type) {
	case doc.Paragraph:
		return layoutFlow(gtx, ctx.flow(BodyStyle(ctx.text), false, wrapProse), b.Runs, tr)
	case doc.Heading:
		return layoutHeading(gtx, ctx, b, tr)
	case doc.Quote:
		return layoutQuote(gtx, ctx, b, tr)
	case doc.CodeBlock, doc.Rule, doc.List, doc.Table, doc.Image:
		// Placeholders until their milestones: height 0, with the margins the caller gives.
		return layout.Dimensions{Size: image.Pt(width, 0)}, nil
	default:
		return layout.Dimensions{}, fmt.Errorf("render: no layout for block %T", b)
	}
}

// layoutHeading lays out a heading's text, then the padding below it, then its rule (STYLE-TEXT-2).
func layoutHeading(gtx layout.Context, ctx blockCtx, h doc.Heading, tr *Trace) (layout.Dimensions, error) {
	style, rule, err := HeadingStyle(h.Level, ctx.text)
	if err != nil {
		return layout.Dimensions{}, err
	}
	dims, err := layoutFlow(gtx, ctx.flow(style, true, wrapProse), h.Runs, tr)
	if err != nil {
		return layout.Dimensions{}, err
	}
	ruleTop := dims.Size.Y + gtx.Dp(rule.PaddingBelow)
	thick := gtx.Dp(rule.Thickness)
	if rule.Thickness > 0 {
		r := image.Rect(0, ruleTop, gtx.Constraints.Max.X, ruleTop+thick)
		fillRect(gtx, r, rule.Colour)
		tr.add(Mark{Kind: MarkHeadingRule, Rect: r, Colour: rule.Colour})
	}
	return layout.Dimensions{Size: image.Pt(dims.Size.X, ruleTop+thick)}, nil
}

// layoutQuote lays out a quote's children, indented past the bar and its padding, in muted text
// (STYLE-QUOTE-1).
func layoutQuote(gtx layout.Context, ctx blockCtx, q doc.Quote, tr *Trace) (layout.Dimensions, error) {
	width := gtx.Constraints.Max.X
	inset := gtx.Dp(quoteBarWidth + quotePadding)
	right := gtx.Dp(quotePadding)
	cgtx := gtx
	cgtx.Constraints = layout.Constraints{Max: image.Pt(max(width-inset-right, 0), gtx.Constraints.Max.Y)}
	dims, err := at(gtx, tr, image.Pt(inset, 0), func() (layout.Dimensions, error) {
		return layoutStack(cgtx, quoteCtx(ctx), q.Blocks, tr)
	})
	if err != nil {
		return layout.Dimensions{}, err
	}
	return layout.Dimensions{Size: image.Pt(width, dims.Size.Y)}, nil
}

// layoutStack lays out blocks one under another, with the gaps between them (DOC-LAYOUT-3). The
// first block has no space above it, so its top margin is not used; the last block's bottom
// margin is not used either.
func layoutStack(gtx layout.Context, ctx blockCtx, blocks []doc.Block, tr *Trace) (layout.Dimensions, error) {
	y := 0
	var prevBottom unit.Dp
	for i, b := range blocks {
		top, bottom, err := Margins(b, false)
		if err != nil {
			return layout.Dimensions{}, err
		}
		if i > 0 {
			y += gtx.Dp(Gap(prevBottom, top))
		}
		dims, err := at(gtx, tr, image.Pt(0, y), func() (layout.Dimensions, error) {
			return layoutBlock(gtx, ctx, b, tr)
		})
		if err != nil {
			return layout.Dimensions{}, err
		}
		y += dims.Size.Y
		prevBottom = bottom
	}
	return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, y)}, nil
}
