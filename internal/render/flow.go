package render

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"
	"unicode/utf8"

	"gioui.org/gesture"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/unit"
	"golang.org/x/image/math/fixed"

	"github.com/LongTermSupport/md-viewer/internal/doc"
)

// links holds one click gesture per link target (LINK-1). Every run of one link shares the
// target pointer (DOC-4), so every fragment of the link shares one gesture. order keeps the
// polling deterministic.
type links struct {
	clicks map[*doc.Target]*gesture.Click
	order  []*doc.Target
}

// newLinks returns an empty set of link gestures.
func newLinks() *links {
	return &links{}
}

// clicked drains the link gestures and returns the first click, if any (LINK-1). It is polled
// once per frame.
func (l *links) clicked(gtx layout.Context) (doc.Target, bool) {
	return doc.Target{}, false
}

// flowParams are the inputs to layoutFlow that do not change within one call.
type flowParams struct {
	shaper    *text.Shaper
	base      TextStyle // the block's base style; its size times its line height is each line's height
	palette   Palette
	inHeading bool // RunStyle's code rule (STYLE-TEXT-5)
	links     *links
}

// Inline code padding in em of the run's size (STYLE-TEXT-5).
const (
	codePadXEm        = 0.4 // STYLE-TEXT-5: .4em left and right
	codePadYEm        = 0.2 // STYLE-TEXT-5: .2em above and below
	codePadXHeadingEm = 0.2 // STYLE-TEXT-5: .2em left and right inside a heading, none vertical
)

// codeRadius is the corner radius of a code background (STYLE-TEXT-5).
const codeRadius unit.Dp = 6 // STYLE-TEXT-5: 6 dp

// Strikethrough (STYLE-TEXT-4): the top edge's height above the baseline in em, and the
// thickness's divisor of the size in px, with its minimum in dp.
const (
	strikeRiseEm  = 0.3 // STYLE-TEXT-4: 0.3 em above the baseline
	strikeDivisor = 16  // STYLE-TEXT-4: size ÷ 16
	strikeMin     = 1   // STYLE-TEXT-4: at least 1 dp
)

// truncator is the text shaping puts where a line was cut. It is a zero-width space, so it takes
// no room (a shaper with the default truncator reserves room for it, and the glyph of the
// truncator is skipped).
const truncator = "​"

// unboundedWidth is a line width no text reaches. It stands for "no limit" in a shaping call.
const unboundedWidth = 1 << 20

// probeText is shaped once per call to find the base face's ascent and descent.
const probeText = "x"

// flow is the state of one layoutFlow call: the current line, and what has been drawn.
type flow struct {
	gtx     layout.Context
	fp      flowParams
	tr      *Trace
	width   int // the content width: gtx.Constraints.Max.X
	lineH   int // L: the height of every line
	baseOff int // the first baseline, from a line's top

	top        int  // the top of the current line
	cursor     int  // the x of the next fragment on the current line
	hasContent bool // a fragment has been placed on the current line
	lines      int  // lines closed so far
	widest     int  // the widest closed line
}

// layoutFlow lays out runs as wrapped lines and records what it drew in tr, which may be nil.
// The line box is the base style's size times its line height (STYLE-TEXT-1, F6), and every
// line is that tall, so the block is lines × L tall. A run's style sets its font, colour and
// marks; the base style sets the baseline.
func layoutFlow(gtx layout.Context, fp flowParams, runs []doc.Run, tr *Trace) (layout.Dimensions, error) {
	f, err := newFlow(gtx, fp, tr)
	if err != nil {
		return layout.Dimensions{}, err
	}
	for _, r := range runs {
		switch r.Kind {
		case doc.TextRun:
			if err := f.textRun(r); err != nil {
				return layout.Dimensions{}, err
			}
		case doc.BreakRun:
			f.closeLine()
		default:
			return layout.Dimensions{}, fmt.Errorf("render: no text flow for run kind %d", r.Kind)
		}
	}
	if f.hasContent || f.lines == 0 {
		f.closeLine()
	}
	size := image.Pt(max(f.widest, gtx.Constraints.Min.X), f.lines*f.lineH)
	return layout.Dimensions{Size: size, Baseline: size.Y - f.baseOff}, nil
}

// newFlow fixes the line height and the first baseline from the base style.
func newFlow(gtx layout.Context, fp flowParams, tr *Trace) (*flow, error) {
	lineH := int(math.Round(pxOf(gtx, fp.base.Size) * float64(fp.base.LineHeight)))
	ascent, descent, err := faceMetrics(gtx, fp)
	if err != nil {
		return nil, err
	}
	baseOff := int(math.Round((float64(lineH)-(ascent+descent))/2 + ascent))
	return &flow{
		gtx:     gtx,
		fp:      fp,
		tr:      tr,
		width:   gtx.Constraints.Max.X,
		lineH:   lineH,
		baseOff: baseOff,
	}, nil
}

// pxOf is a size in px before any rounding (ARCHITECTURE, I3).
func pxOf(gtx layout.Context, s unit.Sp) float64 {
	return float64(s) * float64(gtx.Metric.PxPerSp)
}

// pxPerEm is the size the shaper draws a style at: the size in px, rounded up to a whole pixel
// (ARCHITECTURE, I4).
func pxPerEm(gtx layout.Context, s unit.Sp) fixed.Int26_6 {
	return fixed.I(int(math.Ceil(pxOf(gtx, s))))
}

// faceMetrics returns the base face's ascent and descent in px, from the shaper's metrics of
// probeText in the base style.
func faceMetrics(gtx layout.Context, fp flowParams) (ascent, descent float64, err error) {
	fp.shaper.LayoutString(text.Parameters{
		Font:       fp.base.Font,
		PxPerEm:    pxPerEm(gtx, fp.base.Size),
		MaxWidth:   unboundedWidth,
		Locale:     gtx.Locale,
		WrapPolicy: text.WrapWords,
	}, probeText)
	found := false
	for g, ok := fp.shaper.NextGlyph(); ok; g, ok = fp.shaper.NextGlyph() {
		if !found {
			ascent = fixedPx(g.Ascent)
			descent = fixedPx(g.Descent)
			found = true
		}
	}
	if !found {
		return 0, 0, fmt.Errorf("render: no glyphs for the base face of %q", probeText)
	}
	return ascent, descent, nil
}

// fixedPx converts a fixed-point length in px to a float.
func fixedPx(v fixed.Int26_6) float64 {
	return float64(v) / 64
}

// textRun places the text of one run, breaking it across lines as needed (WRAP-1). A word that
// does not fit the rest of its line moves to the next line (WrapWords, words first). A word
// wider than a whole line breaks at a character (WrapHeuristically), and only then. A non-code
// run drops its leading spaces where it starts a line.
func (f *flow) textRun(r doc.Run) error {
	ts := RunStyle(f.fp.base, r.Style, r.Link != nil, f.fp.palette, f.fp.inHeading)
	code := r.Style&doc.Code != 0
	padX, padY := f.codePads(ts, code)
	rest := r.Text
	for rest != "" {
		if !f.hasContent && !code {
			rest = strings.TrimLeft(rest, " ")
			if rest == "" {
				break
			}
		}
		width := f.width - f.cursor - 2*padX
		glyphs, n, err := f.shape(ts, rest, text.WrapWords, width)
		if err != nil {
			return err
		}
		if n == 0 {
			if f.hasContent {
				f.closeLine()
				continue
			}
			// The first word is wider than a whole line: break it at a character.
			glyphs, n, err = f.shape(ts, rest, text.WrapHeuristically, width)
			if err != nil {
				return err
			}
		}
		if n == 0 {
			// Not even one grapheme fits the line: place the first rune alone, past the
			// line's end, so that the loop always advances.
			_, size := utf8.DecodeRuneInString(rest)
			glyphs, n, err = f.shape(ts, rest[:size], text.WrapWords, unboundedWidth)
			if err != nil {
				return err
			}
		}
		f.place(r, ts, rest[:n], glyphs, padX, padY)
		rest = rest[n:]
		if rest != "" {
			f.closeLine()
		}
	}
	return nil
}

// shape lays out s as one line at most maxWidth px wide. It returns the glyphs without the
// truncator, and the number of bytes of s that the glyphs show.
func (f *flow) shape(ts TextStyle, s string, policy text.WrapPolicy, maxWidth int) ([]text.Glyph, int, error) {
	if maxWidth <= 0 {
		return nil, 0, nil // no room at all: nothing is shown
	}
	f.fp.shaper.LayoutString(text.Parameters{
		Font:       ts.Font,
		PxPerEm:    pxPerEm(f.gtx, ts.Size),
		MaxLines:   1,
		MaxWidth:   maxWidth,
		Truncator:  truncator,
		Locale:     f.gtx.Locale,
		WrapPolicy: policy,
	}, s)
	var glyphs []text.Glyph
	runes := 0
	for g, ok := f.fp.shaper.NextGlyph(); ok; g, ok = f.fp.shaper.NextGlyph() {
		if g.Flags&text.FlagTruncator != 0 {
			continue
		}
		glyphs = append(glyphs, g)
		runes += int(g.Runes)
	}
	return glyphs, byteLen(s, runes), nil
}

// byteLen is the number of bytes that the first runes runes of s take. An invalid byte is one
// rune (DOC-15, P7).
func byteLen(s string, runes int) int {
	n := 0
	for i := 0; i < runes && n < len(s); i++ {
		_, size := utf8.DecodeRuneInString(s[n:])
		n += size
	}
	return n
}

// codePads is a code run's horizontal and vertical padding, in px (STYLE-TEXT-5). The em
// products use the run's own size, so body code (13.6 sp) pads 0.4 × 13.6 across. Inside a
// heading the padding is 0.2 em across and none up or down. Other runs have no padding.
func (f *flow) codePads(ts TextStyle, code bool) (padX, padY int) {
	if !code {
		return 0, 0
	}
	size := pxOf(f.gtx, ts.Size)
	if f.fp.inHeading {
		return int(math.Round(codePadXHeadingEm * size)), 0
	}
	return int(math.Round(codePadXEm * size)), int(math.Round(codePadYEm * size))
}

// place draws the glyphs of s, a fragment of run r, at the cursor on the current line, and records
// their marks. A code run's fragment is padded on both sides, and its background is painted
// before its glyphs (STYLE-TEXT-5). A struck fragment gets its line across its whole advance,
// padding included (STYLE-TEXT-4). The fragment's advance includes the padding.
func (f *flow) place(r doc.Run, ts TextStyle, s string, glyphs []text.Glyph, padX, padY int) {
	if len(glyphs) == 0 {
		return
	}
	code := r.Style&doc.Code != 0
	lo, hi := math.MaxInt, math.MinInt
	ascent, descent := 0, 0
	for _, g := range glyphs {
		lo = min(lo, g.X.Floor())
		hi = max(hi, (g.X + g.Advance).Ceil())
		ascent = max(ascent, g.Ascent.Ceil())
		descent = max(descent, g.Descent.Ceil())
	}
	baseY := f.top + f.baseOff
	originX := f.cursor + padX - lo
	glyphRect := image.Rect(originX+lo, baseY-ascent, originX+hi, baseY+descent)

	if code {
		bg := image.Rect(glyphRect.Min.X-padX, glyphRect.Min.Y-padY, glyphRect.Max.X+padX, glyphRect.Max.Y+padY)
		f.paintBackground(bg)
		f.tr.add(Mark{Kind: MarkCodeBg, Rect: bg, Colour: f.fp.palette.CodeBg})
	}

	stack := op.Offset(image.Pt(originX, baseY-int(glyphs[0].Y))).Push(f.gtx.Ops)
	paint.ColorOp{Color: ts.Colour}.Add(f.gtx.Ops)
	clipStack := clip.Outline{Path: f.fp.shaper.Shape(glyphs)}.Op().Push(f.gtx.Ops)
	paint.PaintOp{}.Add(f.gtx.Ops)
	clipStack.Pop()
	stack.Pop()

	f.tr.add(Mark{Kind: MarkGlyphs, Rect: glyphRect, Colour: ts.Colour, Text: fragmentText(s)})

	boxStart := f.cursor
	f.cursor += 2*padX + hi - lo
	if r.Style&doc.Strike != 0 {
		f.strike(ts, baseY, boxStart, f.cursor)
	}
	f.hasContent = true
}

// strike draws the line through a struck fragment that spans x from start to end (STYLE-TEXT-4):
// its top edge is 0.3 em above the baseline, and it is max(1 dp, size ÷ 16) thick.
func (f *flow) strike(ts TextStyle, baseY, start, end int) {
	size := pxOf(f.gtx, ts.Size)
	top := baseY - int(math.Round(strikeRiseEm*size))
	thick := max(f.gtx.Dp(strikeMin), int(math.Round(size/strikeDivisor)))
	rect := image.Rect(start, top, end, top+thick)
	f.fillRect(rect, ts.Colour)
	f.tr.add(Mark{Kind: MarkStrike, Rect: rect, Colour: ts.Colour})
}

// fillRect fills r with colour c.
func (f *flow) fillRect(r image.Rectangle, c color.NRGBA) {
	stack := clip.Rect(r).Push(f.gtx.Ops)
	paint.ColorOp{Color: c}.Add(f.gtx.Ops)
	paint.PaintOp{}.Add(f.gtx.Ops)
	stack.Pop()
}

// paintBackground fills r with the code background colour, with rounded corners (STYLE-TEXT-5).
func (f *flow) paintBackground(r image.Rectangle) {
	stack := clip.UniformRRect(r, f.gtx.Dp(codeRadius)).Push(f.gtx.Ops)
	paint.ColorOp{Color: f.fp.palette.CodeBg}.Add(f.gtx.Ops)
	paint.PaintOp{}.Add(f.gtx.Ops)
	stack.Pop()
}

// fragmentText is s with each invalid byte shown as U+FFFD (DOC-15).
func fragmentText(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			b.WriteRune(utf8.RuneError)
		} else {
			b.WriteString(s[i : i+size])
		}
		i += size
	}
	return b.String()
}

// closeLine ends the current line and starts the next one below it.
func (f *flow) closeLine() {
	f.widest = max(f.widest, f.cursor)
	f.top += f.lineH
	f.cursor = 0
	f.hasContent = false
	f.lines++
}
