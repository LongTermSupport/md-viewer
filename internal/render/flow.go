package render

import (
	"fmt"
	"image"
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
		if err := f.textRun(r); err != nil {
			return layout.Dimensions{}, err
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

// textRun places the text of one run, breaking it across lines as needed.
func (f *flow) textRun(r doc.Run) error {
	ts := RunStyle(f.fp.base, r.Style, r.Link != nil, f.fp.palette, f.fp.inHeading)
	rest := r.Text
	for rest != "" {
		glyphs, n, err := f.shape(ts, rest, text.WrapWords, f.width-f.cursor)
		if err != nil {
			return err
		}
		if n == 0 {
			if f.hasContent {
				f.closeLine()
				continue
			}
			// Step 2 has no line-start policy: the word is placed whole.
			glyphs, n, err = f.shape(ts, rest, text.WrapWords, unboundedWidth)
			if err != nil {
				return err
			}
		}
		f.place(ts, rest[:n], glyphs)
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

// place draws the glyphs of s at the cursor, on the current line, and records a Glyphs mark.
func (f *flow) place(ts TextStyle, s string, glyphs []text.Glyph) {
	if len(glyphs) == 0 {
		return
	}
	lo, hi := math.MaxInt, math.MinInt
	ascent, descent := 0, 0
	for _, g := range glyphs {
		lo = min(lo, g.X.Floor())
		hi = max(hi, (g.X + g.Advance).Ceil())
		ascent = max(ascent, g.Ascent.Ceil())
		descent = max(descent, g.Descent.Ceil())
	}
	baseY := f.top + f.baseOff
	originX := f.cursor - lo
	glyphRect := image.Rect(originX+lo, baseY-ascent, originX+hi, baseY+descent)

	stack := op.Offset(image.Pt(originX, baseY-int(glyphs[0].Y))).Push(f.gtx.Ops)
	paint.ColorOp{Color: ts.Colour}.Add(f.gtx.Ops)
	clipStack := clip.Outline{Path: f.fp.shaper.Shape(glyphs)}.Op().Push(f.gtx.Ops)
	paint.PaintOp{}.Add(f.gtx.Ops)
	clipStack.Pop()
	stack.Pop()

	f.tr.add(Mark{Kind: MarkGlyphs, Rect: glyphRect, Colour: ts.Colour, Text: fragmentText(s)})
	f.cursor += hi - lo
	f.hasContent = true
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
