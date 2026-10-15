package render

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"
	"unicode/utf8"

	"gioui.org/gesture"
	"gioui.org/io/pointer"
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
	return &links{clicks: make(map[*doc.Target]*gesture.Click)}
}

// click returns the gesture of link target t, made on first use.
func (l *links) click(t *doc.Target) *gesture.Click {
	if l.clicks == nil {
		l.clicks = make(map[*doc.Target]*gesture.Click)
	}
	c, ok := l.clicks[t]
	if !ok {
		c = &gesture.Click{}
		l.clicks[t] = c
		l.order = append(l.order, t)
	}
	return c
}

// clicked drains the link gestures and returns the first click, if any (LINK-1). It is polled
// once per frame, and the events of every gesture are consumed, so a click is reported once.
func (l *links) clicked(gtx layout.Context) (doc.Target, bool) {
	var first doc.Target
	found := false
	for _, t := range l.order {
		c := l.clicks[t]
		for {
			e, ok := c.Update(gtx.Source)
			if !ok {
				break
			}
			if e.Kind == gesture.KindClick && !found {
				first = *t
				found = true
			}
		}
	}
	return first, found
}

// wrapMode is how a flow wraps its lines. The zero value is prose.
type wrapMode uint8

const (
	wrapProse wrapMode = iota // WRAP-1: at word breaks, then at characters
	wrapCode                  // STYLE-CODE-2: at the character that overflows, leading spaces kept
)

// textAlign is how each line of a flow sits in its box (STYLE-TABLE-4). The zero value is start.
type textAlign uint8

const (
	alignStart  textAlign = iota // a line sits at the box's left edge
	alignCentre                  // a line is centred in the box, the spare pixels rounded down on its right
	alignEnd                     // a line sits at the box's right edge
)

// flowParams are the inputs to layoutFlow that do not change within one call.
type flowParams struct {
	shaper    *text.Shaper
	base      TextStyle // the block's base style; its size times its line height is each line's height
	palette   Palette
	inHeading bool // RunStyle's code rule (STYLE-TEXT-5)
	links     *links
	wrap      wrapMode  // how lines wrap; wrapCode for a code block (STYLE-CODE-2)
	align     textAlign // how each line sits in its box (STYLE-TABLE-4)
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
	strikeRiseEm          = 0.3 // STYLE-TEXT-4: 0.3 em above the baseline
	strikeDivisor         = 16  // STYLE-TEXT-4: size ÷ 16
	strikeMin     unit.Dp = 1   // STYLE-TEXT-4: at least 1 dp
)

// truncator is the text shaping puts where a line was cut. It is U+200B, a zero-width space, so
// it takes no room (a shaper with the default truncator reserves room for it, and the glyph of
// the truncator is skipped). It is written as an escape so that it is visible in the source.
const truncator = "\u200b"

// unboundedWidth is a shaping width that no line reaches, so that a call with no limit can
// measure a single rune. It is internal: no spec value depends on it.
const unboundedWidth = 1 << 20

// probeText is the text shaped once per layout to find the base face's ascent and descent. It
// is internal: its glyph is measured, not drawn.
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

	line     op.MacroOp // the macro of the current line, while lineOpen (STYLE-TABLE-4)
	lineOpen bool       // the current line is recorded, because the flow is aligned
	lineMark int        // the trace's mark count when the current line began
}

// layoutFlow lays out runs as wrapped lines and records what it drew in tr, which may be nil.
// The line box is the base style's size times its line height (STYLE-TEXT-1), and every
// line is that tall, so the block is lines × L tall. A run's style sets its font, colour and
// marks; the base style sets the baseline.
//
// The algorithm follows gioui.org/x/styledtext v0.10.3 (styledtext.go and iterator.go; the
// module is Unlicense OR MIT, see its LICENSE). Each run is shaped alone with MaxLines 1 and a
// truncator, and the runes it shows fill the line; what does not fit moves to the next line.
// It differs from styledtext in these ways:
//
//   - A word wider than a whole line breaks by grapheme (WrapHeuristically), at a line start
//     only. styledtext's WrapWords leaves the word whole and overflows the line
//     (styledtext.go, layoutSpan).
//   - The baseline comes from the base face, shared by every run in a line, and the line height
//     is fixed, so styledtext's per-line ascent does not apply.
//   - The flow paints code backgrounds, strike lines and link areas, and records each as a mark
//     (ARCHITECTURE "internal/render").
//   - A non-code run drops its leading spaces where it starts a line (WRAP-1). A code block keeps
//     them, and wraps by grapheme at the character that overflows (STYLE-CODE-2).
//   - If not even one grapheme fits, the first rune is placed alone past the line's end, so the
//     loop always advances (WRAP-1).
func layoutFlow(gtx layout.Context, fp flowParams, runs []doc.Run, tr *Trace) (layout.Dimensions, error) {
	f, err := newFlow(gtx, fp, tr)
	if err != nil {
		return layout.Dimensions{}, err
	}
	for _, r := range runs {
		switch r.Kind {
		case doc.TextRun:
			if err := f.textRun(r); err != nil {
				f.abandon()
				return layout.Dimensions{}, err
			}
		case doc.BreakRun:
			if err := f.closeLine(); err != nil {
				f.abandon()
				return layout.Dimensions{}, err
			}
		default:
			f.abandon()
			return layout.Dimensions{}, fmt.Errorf("render: no text flow for run kind %d", r.Kind)
		}
	}
	// A code block closes its last line even after a break, so text ending in a newline shows
	// the empty line after it (STYLE-CODE-2).
	if f.hasContent || f.lines == 0 || f.fp.wrap == wrapCode {
		if err := f.closeLine(); err != nil {
			f.abandon()
			return layout.Dimensions{}, err
		}
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
	if err := checkAlign(fp.align); err != nil {
		return nil, err
	}
	return &flow{
		gtx:     gtx,
		fp:      fp,
		tr:      tr,
		width:   gtx.Constraints.Max.X,
		lineH:   lineH,
		baseOff: baseOff,
	}, nil
}

// pxOf is a size in px before any rounding; a length is rounded once, where it is used
// (ARCHITECTURE "internal/render", text flow rules).
func pxOf(gtx layout.Context, s unit.Sp) float64 {
	return float64(s) * float64(gtx.Metric.PxPerSp)
}

// dpPx is a length in dp in px before any rounding, as pxOf is for sizes in sp. The unit package
// has no float conversion from dp, so the marker geometry uses this.
func dpPx(gtx layout.Context, d unit.Dp) float64 {
	return float64(d) * float64(gtx.Metric.PxPerDp)
}

// pxPerEm is the size the shaper draws a style at: the size in px, rounded up to a whole pixel
// (ARCHITECTURE "internal/render", text flow rules).
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

// textRun places the text of one run, breaking it across lines as needed (WRAP-1). In prose, a
// word that does not fit the rest of its line moves to the next line (WrapWords, words first),
// and a word wider than a whole line breaks at a character (WrapHeuristically), and only then.
// A code block wraps by grapheme at the character that overflows (STYLE-CODE-2, WrapGraphemes).
// A non-code run drops its leading spaces where it starts a line; a code run and a code block
// keep them.
func (f *flow) textRun(r doc.Run) error {
	if r.Link != nil && f.fp.links == nil {
		return fmt.Errorf("render: link run %q with no link state", r.Text)
	}
	ts := RunStyle(f.fp.base, r.Style, r.Link != nil, f.fp.palette, f.fp.inHeading)
	code := r.Style&doc.Code != 0
	padX, padY := f.codePads(ts, code)
	first, err := firstPolicy(f.fp.wrap)
	if err != nil {
		return err
	}
	// Leading spaces are kept in a code run and in a code block, and dropped elsewhere (WRAP-1).
	keepSpaces := code || f.fp.wrap == wrapCode
	rest := r.Text
	for rest != "" {
		if !f.hasContent && !keepSpaces {
			rest = strings.TrimLeft(rest, " ")
			if rest == "" {
				break
			}
		}
		width := f.width - f.cursor - 2*padX
		glyphs, n := f.shape(ts, rest, first, width)
		if n == 0 {
			if f.hasContent {
				if err := f.closeLine(); err != nil {
					return err
				}
				continue
			}
			// The first word is wider than a whole line: break it at a character.
			glyphs, n = f.shape(ts, rest, text.WrapHeuristically, width)
		}
		if n == 0 {
			// Not even one grapheme fits the line: place the first rune alone, past the
			// line's end, so that the loop always advances.
			_, size := utf8.DecodeRuneInString(rest)
			glyphs, n = f.shape(ts, rest[:size], text.WrapWords, unboundedWidth)
			if n == 0 {
				return fmt.Errorf("render: no glyphs for %q", rest[:size])
			}
		}
		if err := f.place(r, ts, rest[:n], glyphs, padX, padY); err != nil {
			return err
		}
		rest = rest[n:]
		if rest != "" {
			if err := f.closeLine(); err != nil {
				return err
			}
		}
	}
	return nil
}

// firstPolicy is the policy a line's text is first shaped with. A code block wraps at the
// character that overflows (STYLE-CODE-2); prose at a word break (WRAP-1).
func firstPolicy(m wrapMode) (text.WrapPolicy, error) {
	switch m {
	case wrapProse:
		return text.WrapWords, nil
	case wrapCode:
		return text.WrapGraphemes, nil
	default:
		return 0, fmt.Errorf("render: no wrap policy for wrap mode %d", m)
	}
}

// shape lays out s as one line at most maxWidth px wide. It returns the glyphs without the
// truncator, and the number of bytes of s that the glyphs show. A width of zero or less shows
// nothing.
func (f *flow) shape(ts TextStyle, s string, policy text.WrapPolicy, maxWidth int) ([]text.Glyph, int) {
	if maxWidth <= 0 {
		return nil, 0 // no room at all: nothing is shown
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
	return glyphs, byteLen(s, runes)
}

// byteLen is the number of bytes that the first runes runes of s take. An invalid byte is one
// rune (DOC-15).
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
// padding included (STYLE-TEXT-4). The fragment's advance includes the padding. A fragment with
// no glyphs is an error: it would place nothing and not advance the text.
func (f *flow) place(r doc.Run, ts TextStyle, s string, glyphs []text.Glyph, padX, padY int) error {
	if len(glyphs) == 0 {
		return fmt.Errorf("render: fragment %q has no glyphs", s)
	}
	if f.fp.align != alignStart && !f.lineOpen {
		f.openLine()
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
	// The outline's origin is the fragment's left edge on the baseline (STYLE-TEXT-7); its glyph
	// box is derived from it, so the box, the decorations and the ink share one baseline.
	origin := image.Pt(f.cursor+padX-lo, baseY)
	glyphRect := image.Rect(origin.X+lo, origin.Y-ascent, origin.X+hi, origin.Y+descent)

	if code {
		bg := image.Rect(glyphRect.Min.X-padX, glyphRect.Min.Y-padY, glyphRect.Max.X+padX, glyphRect.Max.Y+padY)
		f.paintBackground(bg)
		f.tr.add(Mark{Kind: MarkCodeBg, Rect: bg, Colour: f.fp.palette.CodeBg})
	}

	pop := pushGlyphs(f.gtx, f.fp.shaper, glyphs, origin)
	paint.ColorOp{Color: ts.Colour}.Add(f.gtx.Ops)
	paint.PaintOp{}.Add(f.gtx.Ops)
	pop()

	f.tr.add(Mark{Kind: MarkGlyphs, Rect: glyphRect, Origin: origin, Colour: ts.Colour, Text: fragmentText(s)})

	boxStart := f.cursor
	f.cursor += 2*padX + hi - lo
	if r.Style&doc.Strike != 0 {
		f.strike(ts, baseY, boxStart, f.cursor)
	}
	if r.Link != nil {
		f.linkArea(r.Link, image.Rect(boxStart, f.top, f.cursor, f.top+f.lineH))
	}
	f.hasContent = true
	return nil
}

// pushGlyphs makes the outline of glyphs the clip of what is drawn until the returned pop is
// called, with the outline's origin at origin (STYLE-TEXT-7). Shape's outline has its baseline at
// y = 0 and x from the first glyph, and Glyph.Y is not in it (gioui.org/text, Shape), so origin
// is the fragment's left edge on its baseline. The paint and the ink probes both use it.
func pushGlyphs(gtx layout.Context, sh *text.Shaper, glyphs []text.Glyph, origin image.Point) (pop func()) {
	offset := op.Offset(origin).Push(gtx.Ops)
	outline := clip.Outline{Path: sh.Shape(glyphs)}.Op().Push(gtx.Ops)
	return func() {
		outline.Pop()
		offset.Pop()
	}
}

// linkArea makes rect a link's clickable area: the hand cursor shows over it, and a press and
// release in it is the link's click (LINK-1, STYLE-TEXT-3). The area is the link fragment's
// width and the whole line box.
func (f *flow) linkArea(t *doc.Target, rect image.Rectangle) {
	linkArea(f.gtx, f.fp.links, t, rect, f.tr)
}

// linkArea makes rect the clickable area of link t, in a flow or around an image (LINK-1,
// STYLE-TEXT-3, STYLE-IMG-4): the hand cursor shows over it, a press and release in it is the
// link's click, and the area is recorded as a mark. tr may be nil.
func linkArea(gtx layout.Context, links *links, t *doc.Target, rect image.Rectangle, tr *Trace) {
	stack := clip.Rect(rect).Push(gtx.Ops)
	pointer.CursorPointer.Add(gtx.Ops)
	links.click(t).Add(gtx.Ops)
	stack.Pop()
	tr.add(Mark{Kind: MarkLinkArea, Rect: rect, Link: t})
}

// strike draws the line through a struck fragment that spans x from start to end (STYLE-TEXT-4):
// its top edge is 0.3 em above the baseline, and it is max(1 dp, size ÷ 16) thick.
func (f *flow) strike(ts TextStyle, baseY, start, end int) {
	size := pxOf(f.gtx, ts.Size)
	top := baseY - int(math.Round(strikeRiseEm*size))
	thick := max(f.gtx.Dp(strikeMin), int(math.Round(size/strikeDivisor)))
	rect := image.Rect(start, top, end, top+thick)
	fillRect(f.gtx, rect, ts.Colour)
	f.tr.add(Mark{Kind: MarkStrike, Rect: rect, Colour: ts.Colour})
}

// fillRect fills r with colour c, in the current transform.
func fillRect(gtx layout.Context, r image.Rectangle, c color.NRGBA) {
	stack := clip.Rect(r).Push(gtx.Ops)
	paint.ColorOp{Color: c}.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
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
func (f *flow) closeLine() error {
	f.widest = max(f.widest, f.cursor)
	if f.lineOpen {
		shift, err := lineShift(f.fp.align, f.width, f.cursor)
		if err != nil {
			return err
		}
		call := f.line.Stop()
		f.lineOpen = false
		stack := op.Offset(image.Pt(shift, 0)).Push(f.gtx.Ops)
		call.Add(f.gtx.Ops)
		stack.Pop()
		f.tr.shiftFrom(f.lineMark, image.Pt(shift, 0))
	}
	f.top += f.lineH
	f.cursor = 0
	f.hasContent = false
	f.lines++
	return nil
}

// openLine starts recording the current line, so that closeLine can move it (STYLE-TABLE-4). The
// marks the line records are those from the trace's count now.
func (f *flow) openLine() {
	f.line = op.Record(f.gtx.Ops)
	f.lineMark = f.tr.count()
	f.lineOpen = true
}

// abandon stops an open line's recording without replaying it, when layoutFlow fails, so that no
// operations are left inside an unstopped macro.
func (f *flow) abandon() {
	if f.lineOpen {
		f.line.Stop()
		f.lineOpen = false
	}
}

// checkAlign is an error naming an alignment that is not one of the three (STYLE-TABLE-4).
func checkAlign(a textAlign) error {
	switch a {
	case alignStart, alignCentre, alignEnd:
		return nil
	default:
		return fmt.Errorf("render: no line alignment for value %d", a)
	}
}

// lineShift is how far a line of width w moves right in a box of width box (STYLE-TABLE-4): the
// spare pixels rounded down for centre, all of them for end. A line wider than the box, and the
// start alignment, do not move.
func lineShift(a textAlign, box, w int) (int, error) {
	spare := max(box-w, 0)
	switch a {
	case alignStart:
		return 0, nil
	case alignCentre:
		return spare / 2, nil
	case alignEnd:
		return spare, nil
	default:
		return 0, fmt.Errorf("render: no line alignment for value %d", a)
	}
}
