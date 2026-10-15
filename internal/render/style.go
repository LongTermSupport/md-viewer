package render

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

	"gioui.org/font"
	"gioui.org/unit"

	"github.com/LongTermSupport/md-viewer/internal/doc"
)

// TextStyle is what a run is drawn with. LineHeight is a multiple of Size.
type TextStyle struct {
	Font       font.Font
	Size       unit.Sp
	LineHeight float32
	Colour     color.NRGBA
}

// HeadingRule is the rule under a heading (STYLE-TEXT-2). The zero value is no padding and no
// rule.
type HeadingRule struct {
	PaddingBelow, Thickness unit.Dp
	Colour                  color.NRGBA
}

// Body text (STYLE-TEXT-1).
const (
	bodySize       unit.Sp = 16  // STYLE-TEXT-1: 16 sp
	lineHeightBody float32 = 1.5 // STYLE-TEXT-1: 1.5 × font size
)

// codeScale is inline code's size as a share of the surrounding size (STYLE-TEXT-5), outside a
// heading. In a heading the code is 100% of the heading size.
const codeScale = 0.85 // STYLE-TEXT-5

// Font weight and heading constants. Go Bold declares weight 600, which Gio names font.SemiBold
// (STYLE-FONT-3), and GitHub asks for 600 too.
const boldWeight font.Weight = font.SemiBold // STYLE-FONT-3

const (
	lineHeightHeading float32 = 1.25 // STYLE-TEXT-2: line height of every level
	headingRuleWidth  unit.Dp = 1    // STYLE-TEXT-2: rule thickness, 1 dp
)

// RunStyle is the style of a run inside base. Flags add to the base: Strong sets the bold
// weight, Emphasis the italic style, Code the Go Mono family (STYLE-FONT-3, STYLE-FONT-4). A
// link takes the palette's Accent colour (STYLE-TEXT-3). The line height is always the base's
// (F6, STYLE-TEXT-5).
func RunStyle(base TextStyle, s doc.Style, link bool, p Palette, inHeading bool) TextStyle {
	ts := base
	if s&doc.Strong != 0 {
		ts.Font.Weight = boldWeight
	}
	if s&doc.Emphasis != 0 {
		ts.Font.Style = font.Italic
	}
	if s&doc.Code != 0 {
		ts.Font.Typeface = typefaceMono
		// Go Mono has no italic in this set, so code is upright (STYLE-FONT-4).
		ts.Font.Style = font.Regular
		if !inHeading {
			ts.Size = base.Size * codeScale
		}
	}
	if link {
		ts.Colour = p.Accent
	}
	return ts
}

// BodyStyle is the body text's style (STYLE-TEXT-1).
func BodyStyle(p Palette) TextStyle {
	return TextStyle{
		Font:       font.Font{Typeface: typefaceText, Style: font.Regular, Weight: font.Normal},
		Size:       bodySize,
		LineHeight: lineHeightBody,
		Colour:     p.Fg,
	}
}

// headingSpec is one row of STYLE-TEXT-2's table. A level with no rule has no padding either.
// The numbers are written out, not computed from em: float32 cannot give .3em of 24 exactly,
// so the table is the spec's literal values (F4).
type headingSpec struct {
	size    unit.Sp
	padding unit.Dp // padding below the text, before the rule
	rule    bool    // a rule of headingRuleWidth in BorderMuted
	muted   bool    // colour FgMuted instead of Fg
}

// headingSpecs holds levels 1 to 6, at index level-1.
var headingSpecs = [...]headingSpec{
	{size: 32, padding: 9.6, rule: true}, // level 1: 2em, padding .3em
	{size: 24, padding: 7.2, rule: true}, // level 2: 1.5em, padding .3em
	{size: 20},                           // level 3: 1.25em
	{size: 16},                           // level 4: 1em
	{size: 14},                           // level 5: .875em
	{size: 13.6, muted: true},            // level 6: .85em, FgMuted
}

// HeadingStyle is the style of heading level 1 to 6 (STYLE-TEXT-2). Any other level is an
// error (F3).
func HeadingStyle(level int, p Palette) (TextStyle, HeadingRule, error) {
	if level < 1 || level > len(headingSpecs) {
		return TextStyle{}, HeadingRule{}, fmt.Errorf("render: no heading style for level %d", level)
	}
	spec := headingSpecs[level-1]
	ts := TextStyle{
		Font:       font.Font{Typeface: typefaceText, Style: font.Regular, Weight: boldWeight},
		Size:       spec.size,
		LineHeight: lineHeightHeading,
		Colour:     p.Fg,
	}
	if spec.muted {
		ts.Colour = p.FgMuted
	}
	var rule HeadingRule
	if spec.rule {
		rule = HeadingRule{PaddingBelow: spec.padding, Thickness: headingRuleWidth, Colour: p.BorderMuted}
	}
	return ts, rule, nil
}

// Block margins in dp (DOC-LAYOUT-4).
const (
	marginRem        unit.Dp = 16 // DOC-LAYOUT-4: 1rem, the bottom margin of most blocks
	marginHeadingTop unit.Dp = 24 // DOC-LAYOUT-4: 1.5rem above a heading
	marginRule       unit.Dp = 24 // DOC-LAYOUT-4: hr { margin: 1.5rem 0 }
)

// Margins gives a block's top and bottom margins (DOC-LAYOUT-4). nested is true for a list
// inside a list item, the only block whose margins depend on nesting (F7). A block with no row
// is an error (F3). Margins does not check Heading.Level: HeadingStyle rejects levels outside
// 1 to 6.
func Margins(b doc.Block, nested bool) (top, bottom unit.Dp, err error) {
	switch b := b.(type) {
	case doc.Heading:
		return marginHeadingTop, marginRem, nil
	case doc.Rule:
		return marginRule, marginRule, nil
	case doc.List:
		if nested {
			return 0, 0, nil // ul ul, ul ol, ol ol, ol ul: margin 0
		}
		return 0, marginRem, nil
	case doc.Paragraph, doc.Quote, doc.CodeBlock, doc.Table, doc.Image:
		return 0, marginRem, nil
	default:
		return 0, 0, fmt.Errorf("render: no margins for block %T", b)
	}
}

// Gap is the space between two blocks: the larger of the first's bottom margin and the second's
// top margin, as CSS collapses them (DOC-LAYOUT-3).
func Gap(prevBottom, nextTop unit.Dp) unit.Dp {
	if prevBottom > nextTop {
		return prevBottom
	}
	return nextTop
}

// Content padding (DOC-LAYOUT-1).
const (
	paddingBreakpoint unit.Dp = 768 // DOC-LAYOUT-1: the window width at which the wide padding starts
	paddingWide       unit.Dp = 45  // DOC-LAYOUT-1: .markdown-body { padding: 45px }
	paddingNarrow     unit.Dp = 15  // DOC-LAYOUT-1: padding under 768 dp
)

// Padding is the content inset on all four sides for a window width (DOC-LAYOUT-1). The width
// is after zoom (STYLE-ZOOM-3), so the caller passes the zoomed width.
func Padding(windowWidth unit.Dp) unit.Dp {
	if windowWidth >= paddingBreakpoint {
		return paddingWide
	}
	return paddingNarrow
}

// romanMax is the largest number written in roman numerals (STYLE-LIST-3, F8).
const romanMax = 3999 // STYLE-LIST-3

// romanDigits are the roman numeral values, largest first.
var romanDigits = [...]struct {
	value int
	digit string
}{
	{1000, "m"}, {900, "cm"}, {500, "d"}, {400, "cd"}, {100, "c"}, {90, "xc"},
	{50, "l"}, {40, "xl"}, {10, "x"}, {9, "ix"}, {5, "v"}, {4, "iv"}, {1, "i"},
}

// OrderedMarker is the marker text of an ordered item: the number and a period (STYLE-LIST-3).
// Depth 0 is decimal, depth 1 lower-roman and depth 2 or more lower-alpha (after z comes aa).
// A number outside its style's range, such as 0 or a roman numeral above 3999, is decimal (F8).
func OrderedMarker(n, depth int) string {
	switch {
	case depth == 1 && n >= 1 && n <= romanMax:
		return toRoman(n) + "."
	case depth >= 2 && n >= 1:
		return toAlpha(n) + "."
	default:
		return strconv.Itoa(n) + "."
	}
}

// toRoman writes n, which must be from 1 to romanMax, in lower-case roman numerals.
func toRoman(n int) string {
	var b strings.Builder
	for _, d := range romanDigits {
		for n >= d.value {
			b.WriteString(d.digit)
			n -= d.value
		}
	}
	return b.String()
}

// toAlpha writes n, which must be at least 1, in bijective base 26: 1 is a, 26 is z, 27 is aa.
func toAlpha(n int) string {
	var buf []byte
	for n > 0 {
		n--
		buf = append([]byte{byte('a' + n%26)}, buf...)
		n /= 26
	}
	return string(buf)
}

// ColumnWidths gives each table column's width (STYLE-TABLE-5). When the natural widths sum to
// at most width, each keeps its width. Otherwise each is floor(N × width ÷ ΣN), and at least
// minCol. The minimums can push the sum above width; the caller clips that (M7). A negative width
// fits nothing, so with natural widths summing to 0 there is no share to take and each column is
// minCol. The input is not modified.
func ColumnWidths(natural []int, width, minCol int) []int {
	sum := 0
	for _, n := range natural {
		sum += n
	}
	out := make([]int, len(natural))
	if sum <= width {
		copy(out, natural)
		return out
	}
	for i, n := range natural {
		share := 0
		if sum > 0 {
			share = n * width / sum
		}
		out[i] = max(share, minCol)
	}
	return out
}
