package render

import (
	"fmt"
	"image"
	"image/color"
	"strings"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
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
	depth  int  // the number of lists around this block (STYLE-LIST-2, STYLE-LIST-3)
	tight  bool // this block is a direct child of a tight list's item (STYLE-LIST-4)
}

// quoteCtx is the context of a quote's children: their text is FgMuted (STYLE-QUOTE-1). A quote is
// not an item of a tight list, so its children are not tight, but it does not end a list's
// nesting, so its depth is kept.
func quoteCtx(c blockCtx) blockCtx {
	c.text.Fg = c.root.FgMuted
	c.tight = false
	return c
}

// stackMargins are a block's top and bottom margins in a stack. In a tight list item a paragraph
// or an image has none (STYLE-LIST-4). Any other block has its DOC-LAYOUT-4 margins, with a list
// nested when it is inside a list at any depth.
func stackMargins(ctx blockCtx, b doc.Block) (top, bottom unit.Dp, err error) {
	_, para := b.(doc.Paragraph)
	_, img := b.(doc.Image)
	if ctx.tight && (para || img) {
		return 0, 0, nil
	}
	return Margins(b, ctx.depth > 0)
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

// Block sizes in dp (STYLE-CODE-1, STYLE-QUOTE-1).
const (
	codeBlockPadding unit.Dp = 16 // STYLE-CODE-1: 16 dp on all sides of the text
	codeBlockRadius  unit.Dp = 6  // STYLE-CODE-1: 6 dp corner radius
	ruleHeight       unit.Dp = 4  // STYLE-RULE-1: .25em of 16, the bar's height
	quoteBarWidth    unit.Dp = 4  // STYLE-QUOTE-1: .25em of 16, the bar's width
	quotePadding     unit.Dp = 16 // STYLE-QUOTE-1: the padding after the bar, and on the right
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
	case doc.CodeBlock:
		return layoutCodeBlock(gtx, ctx, b, tr)
	case doc.Rule:
		return layoutRule(gtx, ctx, tr)
	case doc.List:
		return layoutList(gtx, ctx, b, tr)
	case doc.Table, doc.Image:
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
	total := ruleTop + thick
	// The first baseline is the text's, so its distance from the bottom is the total less the text's
	// distance from the top of the flow.
	return layout.Dimensions{Size: image.Pt(dims.Size.X, total), Baseline: total - (dims.Size.Y - dims.Baseline)}, nil
}

// layoutCodeBlock lays out a code block: its lines in Go Mono, wrapped at the character that
// overflows, inside 16 dp of padding, on a rounded background across the content width
// (STYLE-CODE-1, STYLE-CODE-2). The text is recorded first and the background painted before it,
// since the height is known only after the layout (a recorded macro replays the text on top).
func layoutCodeBlock(gtx layout.Context, ctx blockCtx, c doc.CodeBlock, tr *Trace) (layout.Dimensions, error) {
	width := gtx.Constraints.Max.X
	pad := gtx.Dp(codeBlockPadding)
	igtx := gtx
	igtx.Constraints = layout.Constraints{Max: image.Pt(max(width-2*pad, 0), gtx.Constraints.Max.Y)}
	fp := ctx.flow(codeBlockStyle(ctx.root), false, wrapCode)
	fp.palette = ctx.root

	rec := op.Record(gtx.Ops)
	dims, err := at(gtx, tr, image.Pt(pad, pad), func() (layout.Dimensions, error) {
		return layoutFlow(igtx, fp, codeRuns(c.Text), tr)
	})
	call := rec.Stop()
	if err != nil {
		return layout.Dimensions{}, err
	}

	h := dims.Size.Y + 2*pad
	bg := image.Rect(0, 0, width, h)
	fillRoundRect(gtx, bg, codeBlockRadius, ctx.root.BgMuted)
	tr.add(Mark{Kind: MarkCodeBlockBg, Rect: bg, Colour: ctx.root.BgMuted})
	call.Add(gtx.Ops)
	// The first baseline is the padding and the flow's first baseline below the top.
	return layout.Dimensions{Size: image.Pt(width, h), Baseline: h - (pad + (dims.Size.Y - dims.Baseline))}, nil
}

// codeRuns is a code block's text as runs: a break before each line after the first, and a text
// run for each line that is not empty. Text with n newlines has n + 1 lines (STYLE-CODE-2), so
// the empty text is one empty line.
func codeRuns(src string) []doc.Run {
	var runs []doc.Run
	for i, line := range strings.Split(src, "\n") {
		if i > 0 {
			runs = append(runs, doc.Run{Kind: doc.BreakRun})
		}
		if line != "" {
			runs = append(runs, doc.Run{Kind: doc.TextRun, Text: line})
		}
	}
	return runs
}

// fillRoundRect fills r with colour c, with corners of radius rad.
func fillRoundRect(gtx layout.Context, r image.Rectangle, rad unit.Dp, c color.NRGBA) {
	stack := clip.UniformRRect(r, gtx.Dp(rad)).Push(gtx.Ops)
	paint.ColorOp{Color: c}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	stack.Pop()
}

// layoutRule draws a thematic break as a bar across the content width, in Border (STYLE-RULE-1).
func layoutRule(gtx layout.Context, ctx blockCtx, tr *Trace) (layout.Dimensions, error) {
	width := gtx.Constraints.Max.X
	r := image.Rect(0, 0, width, gtx.Dp(ruleHeight))
	fillRect(gtx, r, ctx.root.Border)
	tr.add(Mark{Kind: MarkRuleBar, Rect: r, Colour: ctx.root.Border})
	return layout.Dimensions{Size: r.Size()}, nil
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
	if h := dims.Size.Y; h > 0 {
		bar := image.Rect(0, 0, gtx.Dp(quoteBarWidth), h)
		fillRect(gtx, bar, ctx.root.Border)
		tr.add(Mark{Kind: MarkQuoteBar, Rect: bar, Colour: ctx.root.Border})
	}
	return layout.Dimensions{Size: image.Pt(width, dims.Size.Y), Baseline: dims.Baseline}, nil
}

// layoutStack lays out blocks one under another, with the gaps between them (DOC-LAYOUT-3). The
// first block has no space above it, so its top margin is not used; the last block's bottom
// margin is not used either.
func layoutStack(gtx layout.Context, ctx blockCtx, blocks []doc.Block, tr *Trace) (layout.Dimensions, error) {
	y := 0
	var prevBottom unit.Dp
	firstTop := 0 // the first block's first baseline, from the top of the first block
	for i, b := range blocks {
		top, bottom, err := stackMargins(ctx, b)
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
		if i == 0 {
			firstTop = dims.Size.Y - dims.Baseline
		}
		y += dims.Size.Y
		prevBottom = bottom
	}
	return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, y), Baseline: y - firstTop}, nil
}
