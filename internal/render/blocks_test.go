package render

import (
	"image"
	"image/color"
	"strings"
	"testing"
)

// hasRect reports whether ms has a mark of rectangle r and colour c.
func hasRect(ms []Mark, r image.Rectangle, c color.NRGBA) bool {
	for _, m := range ms {
		if m.Rect == r && m.Colour == c {
			return true
		}
	}
	return false
}

// TestHeadingRule: levels 1 and 2 have a 1 dp rule in BorderMuted, across the content width and
// directly under the padding below the text; levels 3 to 6 have none (STYLE-TEXT-2). Level 6 is
// FgMuted. Heights are the line, the padding below, the rule, and the 45 px padding either side.
func TestHeadingRule(t *testing.T) {
	tests := []struct {
		src    string
		rule   image.Rectangle // the zero rectangle when the level has no rule
		height int
		colour color.NRGBA
	}{
		{"# T", image.Rect(45, 95, 755, 96), 141, Light.Fg},
		{"## T", image.Rect(45, 82, 755, 83), 128, Light.Fg},
		{"### T", image.Rectangle{}, 115, Light.Fg},
		{"#### T", image.Rectangle{}, 110, Light.Fg},
		{"##### T", image.Rectangle{}, 108, Light.Fg},
		{"###### T", image.Rectangle{}, 107, Light.FgMuted},
	}
	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			e := newViewEnv(t, tt.src, Light, metric1)
			dims, tr := e.layout(0, 800)
			if dims.Size.Y != tt.height {
				t.Fatalf("%s: Size.Y = %d, want %d", tt.src, dims.Size.Y, tt.height)
			}
			if g := glyphMark(t, tr, "T"); g.Colour != tt.colour {
				t.Fatalf("%s: text colour %v, want %v", tt.src, g.Colour, tt.colour)
			}
			rules := marks(tr, MarkHeadingRule)
			if tt.rule == (image.Rectangle{}) {
				if len(rules) != 0 {
					t.Fatalf("%s: %d HeadingRule marks, want none", tt.src, len(rules))
				}
				return
			}
			if len(rules) != 1 {
				t.Fatalf("%s: %d HeadingRule marks, want 1", tt.src, len(rules))
			}
			if rules[0].Rect != tt.rule {
				t.Fatalf("%s: rule at %v, want %v", tt.src, rules[0].Rect, tt.rule)
			}
			if rules[0].Colour != Light.BorderMuted {
				t.Fatalf("%s: rule colour %v, want BorderMuted %v", tt.src, rules[0].Colour, Light.BorderMuted)
			}
		})
	}

	// Ten words wrap to whole 40 px lines above the rule, which stays 15 px from the edges.
	e := newViewEnv(t, "# "+strings.TrimSpace(strings.Repeat("word ", 10)), Light, metric1)
	dims, tr := e.layout(0, 300)
	rules := needMarks(t, tr, MarkHeadingRule, "HeadingRule")
	if len(rules) != 1 {
		t.Fatalf("wrapped heading: %d HeadingRule marks, want 1", len(rules))
	}
	if r := rules[0].Rect; r.Min.X != 15 || r.Max.X != 285 || r.Max.Y != dims.Size.Y-15 {
		t.Fatalf("wrapped heading rule %v in a %d px block, want x 15 to 285 and 15 px above the end", r, dims.Size.Y)
	}
	if n := dims.Size.Y - 30 - 11; n%40 != 0 || n < 80 {
		t.Fatalf("wrapped heading is %d px tall, want whole 40 px lines and at least two", dims.Size.Y)
	}

	// At 2 px per dp the rule, the padding and the line all double (DOC-LAYOUT-1).
	e = newViewEnv(t, "# T", Light, metric2)
	dims, tr = e.layout(0, 1600)
	rules = needMarks(t, tr, MarkHeadingRule, "HeadingRule")
	if len(rules) != 1 || rules[0].Rect != image.Rect(90, 189, 1510, 191) {
		t.Fatalf("at 2 px per dp: rule %v, want (90,189)-(1510,191)", rules)
	}
	if dims.Size.Y != 281 {
		t.Fatalf("at 2 px per dp: Size.Y = %d, want 281", dims.Size.Y)
	}
}

// TestCodeBlock: a code block is a background across the content width in BgMuted, with its text
// inset 16 px on every side, in Fg (STYLE-CODE-1). A line wraps inside the box (STYLE-CODE-2).
// Lines are 20 px, and a text with n newlines shows n + 1 lines (STYLE-CODE-2).
func TestCodeBlock(t *testing.T) {
	t.Run("short", func(t *testing.T) {
		e := newViewEnv(t, "```\nshort\n```", Light, metric1)
		dims, tr := e.layout(0, 800)
		bgs := needMarks(t, tr, MarkCodeBlockBg, "CodeBlockBg")
		if len(bgs) != 1 {
			t.Fatalf("%d CodeBlockBg marks, want 1", len(bgs))
		}
		if bgs[0].Rect != image.Rect(45, 45, 755, 97) || bgs[0].Colour != Light.BgMuted {
			t.Fatalf("background %v in %v, want (45,45)-(755,97) in BgMuted", bgs[0].Rect, bgs[0].Colour)
		}
		g := glyphMark(t, tr, "short")
		if g.Rect.Min.X != 61 {
			t.Fatalf("text starts at x=%d, want 61 (45 + 16 padding)", g.Rect.Min.X)
		}
		if !within(image.Rect(61, 61, 739, 81), g.Rect) {
			t.Fatalf("text %v is not inside (61,61)-(739,81)", g.Rect)
		}
		if g.Colour != Light.Fg {
			t.Fatalf("text colour %v, want Fg %v", g.Colour, Light.Fg)
		}
		if dims.Size.Y != 142 {
			t.Fatalf("Size.Y = %d, want 142 (45 + 52 + 45)", dims.Size.Y)
		}
	})

	t.Run("blank line between lines", func(t *testing.T) {
		e := newViewEnv(t, "```\na\n\nb\n```", Light, metric1)
		_, tr := e.layout(0, 800)
		bgs := needMarks(t, tr, MarkCodeBlockBg, "CodeBlockBg")
		if len(bgs) != 1 || bgs[0].Rect.Max.Y != 137 {
			t.Fatalf("three lines: background %v, want its bottom at 137", bgs)
		}
		a := glyphMark(t, tr, "a")
		b := glyphMark(t, tr, "b")
		if d := b.Rect.Min.Y - a.Rect.Min.Y; d != 40 {
			t.Fatalf("b is %d px below a, want 40 (the blank line is a line)", d)
		}
	})

	t.Run("leading spaces", func(t *testing.T) {
		e := newViewEnv(t, "```\n    x\n```", Light, metric1)
		_, tr := e.layout(0, 800)
		g := glyphMark(t, tr, "    x")
		if g.Rect.Min.X != 61 {
			t.Fatalf("indented text starts at x=%d, want 61", g.Rect.Min.X)
		}
	})

	t.Run("text with a trailing newline", func(t *testing.T) {
		e := newViewEnv(t, "```\na\n\n```", Light, metric1)
		_, tr := e.layout(0, 800)
		bgs := needMarks(t, tr, MarkCodeBlockBg, "CodeBlockBg")
		if len(bgs) != 1 || bgs[0].Rect.Dy() != 72 {
			t.Fatalf("text a\\n: background %v, want 72 px tall (two lines)", bgs)
		}
	})

	t.Run("empty", func(t *testing.T) {
		e := newViewEnv(t, "```\n```", Light, metric1)
		_, tr := e.layout(0, 800)
		bgs := needMarks(t, tr, MarkCodeBlockBg, "CodeBlockBg")
		if len(bgs) != 1 || bgs[0].Rect.Dy() != 52 {
			t.Fatalf("empty fence: background %v, want 52 px tall (one empty line)", bgs)
		}
		if n := len(marks(tr, MarkGlyphs)); n != 0 {
			t.Fatalf("empty fence: %d Glyphs marks, want none", n)
		}
	})

	t.Run("wrap", func(t *testing.T) {
		long := strings.Repeat("x", 100)
		e := newViewEnv(t, "```\nab "+long+"\n```", Light, metric1)
		_, tr := e.layout(0, 400)
		bgs := needMarks(t, tr, MarkCodeBlockBg, "CodeBlockBg")
		if len(bgs) != 1 || bgs[0].Rect.Max.X != 385 {
			t.Fatalf("background %v, want its right edge at 385 (400 - 15)", bgs)
		}
		gs := needMarks(t, tr, MarkGlyphs, "Glyphs")
		if starts := lineStarts(gs); len(starts) < 2 {
			t.Fatalf("%d lines, want at least 2", len(starts))
		} else if !strings.HasPrefix(starts[0].Text, "ab x") {
			t.Fatalf("first line %q, want it to start \"ab x\"", starts[0].Text)
		}
		for _, m := range gs {
			if m.Rect.Max.X > 369 {
				t.Fatalf("glyphs %q end at x=%d, want at most 369 (inner width 338 from 31)", m.Text, m.Rect.Max.X)
			}
		}
		if got := joinGlyphText(tr); got != "ab "+long {
			t.Fatalf("wrapped text is %q, want the source line", got)
		}
	})

	t.Run("dark", func(t *testing.T) {
		e := newViewEnv(t, "```\nshort\n```", Dark, metric1)
		_, tr := e.layout(0, 800)
		bgs := needMarks(t, tr, MarkCodeBlockBg, "CodeBlockBg")
		if len(bgs) != 1 || bgs[0].Colour != Dark.BgMuted {
			t.Fatalf("dark background %v, want BgMuted %v", bgs, Dark.BgMuted)
		}
		if g := glyphMark(t, tr, "short"); g.Colour != Dark.Fg {
			t.Fatalf("dark text colour %v, want Fg %v", g.Colour, Dark.Fg)
		}
	})
}

// TestRule: a rule is a 4 dp bar in Border across the content width, with 24 px above and below
// (STYLE-RULE-1, DOC-LAYOUT-4).
func TestRule(t *testing.T) {
	e := newViewEnv(t, "a\n\n---\n\nb", Light, metric1)
	dims, tr := e.layout(1, 800)
	rules := needMarks(t, tr, MarkRuleBar, "RuleBar")
	if len(rules) != 1 || rules[0].Rect != image.Rect(45, 24, 755, 28) {
		t.Fatalf("rule %v, want one bar at (45,24)-(755,28)", rules)
	}
	if rules[0].Colour != Light.Border {
		t.Fatalf("rule colour %v, want Border %v", rules[0].Colour, Light.Border)
	}
	if dims.Size.Y != 28 {
		t.Fatalf("rule element is %d px tall, want 28 (24 + 4)", dims.Size.Y)
	}
	if last := e.dims(2, 800).Size.Y; last != 93 {
		t.Fatalf("element after the rule is %d px tall, want 93 (24 + 24 + 45)", last)
	}

	e = newViewEnv(t, "---", Light, metric1)
	dims, tr = e.layout(0, 800)
	rules = needMarks(t, tr, MarkRuleBar, "RuleBar")
	if len(rules) != 1 || rules[0].Rect != image.Rect(45, 45, 755, 49) {
		t.Fatalf("lone rule %v, want one bar at (45,45)-(755,49)", rules)
	}
	if dims.Size.Y != 94 {
		t.Fatalf("lone rule is %d px tall, want 94 (45 + 4 + 45)", dims.Size.Y)
	}

	e = newViewEnv(t, "---", Dark, metric1)
	_, tr = e.layout(0, 800)
	rules = needMarks(t, tr, MarkRuleBar, "RuleBar")
	if len(rules) != 1 || rules[0].Colour != Dark.Border {
		t.Fatalf("dark rule %v, want Dark.Border %v", rules, Dark.Border)
	}
}

// TestQuote: a quote has a 4 dp bar in Border as tall as the quote, and its children start 20 px
// right of the bar, in FgMuted (STYLE-QUOTE-1). Nested quotes nest, and headings, rules and code
// keep their own colours inside a quote.
func TestQuote(t *testing.T) {
	e := newViewEnv(t, "> a\n>\n> > b", Light, metric1)
	dims, tr := e.layout(0, 800)
	bars := needMarks(t, tr, MarkQuoteBar, "QuoteBar")
	if len(bars) != 2 {
		t.Fatalf("%d QuoteBar marks, want 2 (outer and nested)", len(bars))
	}
	if !hasRect(bars, image.Rect(45, 45, 49, 109), Light.Border) {
		t.Fatalf("no outer bar at (45,45)-(49,109) in Border: %v", bars)
	}
	if !hasRect(bars, image.Rect(65, 85, 69, 109), Light.Border) {
		t.Fatalf("no nested bar at (65,85)-(69,109) in Border: %v", bars)
	}
	if g := glyphMark(t, tr, "a"); g.Rect.Min.X != 65 || g.Colour != Light.FgMuted {
		t.Fatalf("a at x=%d in %v, want x 65 in FgMuted", g.Rect.Min.X, g.Colour)
	}
	if g := glyphMark(t, tr, "b"); g.Rect.Min.X != 85 || g.Colour != Light.FgMuted {
		t.Fatalf("b at x=%d in %v, want x 85 in FgMuted", g.Rect.Min.X, g.Colour)
	}
	if dims.Size.Y != 154 {
		t.Fatalf("quote is %d px tall, want 154 (45 + 64 + 45)", dims.Size.Y)
	}

	// Inline code and links: the text is FgMuted, the link Accent, the code FgMuted (STYLE-QUOTE-1).
	e = newViewEnv(t, "> a [l](x.md) `c`", Light, metric1)
	_, tr = e.layout(0, 800)
	if c := glyphMark(t, tr, "a ").Colour; c != Light.FgMuted {
		t.Fatalf("quoted text colour %v, want FgMuted %v", c, Light.FgMuted)
	}
	if c := glyphMark(t, tr, "l").Colour; c != Light.Accent {
		t.Fatalf("quoted link colour %v, want Accent %v", c, Light.Accent)
	}
	if c := glyphMark(t, tr, "c").Colour; c != Light.FgMuted {
		t.Fatalf("quoted inline code colour %v, want FgMuted %v", c, Light.FgMuted)
	}

	// A heading in a quote is FgMuted, keeps its rule, and is as tall as the same heading outside.
	e = newViewEnv(t, "> # h", Light, metric1)
	qDims, tr := e.layout(0, 800)
	qg := glyphMark(t, tr, "h")
	if qg.Colour != Light.FgMuted {
		t.Fatalf("quoted heading colour %v, want FgMuted %v", qg.Colour, Light.FgMuted)
	}
	rules := needMarks(t, tr, MarkHeadingRule, "HeadingRule")
	if len(rules) != 1 || rules[0].Colour != Light.BorderMuted {
		t.Fatalf("quoted heading rule %v, want one in BorderMuted", rules)
	}
	top := newViewEnv(t, "# h", Light, metric1)
	topDims, topTr := top.layout(0, 800)
	if tg := glyphMark(t, topTr, "h"); qg.Rect.Min.Y != tg.Rect.Min.Y || qDims.Size.Y != topDims.Size.Y {
		t.Fatalf("quoted heading at y=%d in a %d px quote, want y %d in %d px as outside the quote", qg.Rect.Min.Y, qDims.Size.Y, tg.Rect.Min.Y, topDims.Size.Y)
	}

	// A rule in a quote is 20 px in from the bar, and keeps Border.
	e = newViewEnv(t, "> ---", Light, metric1)
	dims, tr = e.layout(0, 800)
	rules2 := needMarks(t, tr, MarkRuleBar, "RuleBar")
	if len(rules2) != 1 || rules2[0].Rect != image.Rect(65, 45, 739, 49) || rules2[0].Colour != Light.Border {
		t.Fatalf("quoted rule %v, want one Border bar at (65,45)-(739,49)", rules2)
	}
	if dims.Size.Y != 94 {
		t.Fatalf("quote with a rule is %d px tall, want 94", dims.Size.Y)
	}

	// Fenced code in a quote keeps its own background and colour.
	e = newViewEnv(t, "> ```\n> code\n> ```", Light, metric1)
	_, tr = e.layout(0, 800)
	bgs := needMarks(t, tr, MarkCodeBlockBg, "CodeBlockBg")
	if len(bgs) != 1 || bgs[0].Rect != image.Rect(65, 45, 739, 97) || bgs[0].Colour != Light.BgMuted {
		t.Fatalf("quoted code background %v, want (65,45)-(739,97) in BgMuted", bgs)
	}
	if g := glyphMark(t, tr, "code"); g.Colour != Light.Fg {
		t.Fatalf("quoted code colour %v, want Fg %v", g.Colour, Light.Fg)
	}

	// Sixty words at 300 px wrap inside the quote: 35 px to 269 px (the bar, the inset, the right
	// inset of 16 px and the 15 px padding).
	e = newViewEnv(t, "> "+strings.TrimSpace(strings.Repeat("word ", 60)), Light, metric1)
	_, tr = e.layout(0, 300)
	for _, m := range needMarks(t, tr, MarkGlyphs, "Glyphs") {
		if m.Rect.Min.X < 35 || m.Rect.Max.X > 269 {
			t.Fatalf("quoted text %q spans x %d to %d, want within 35 to 269", m.Text, m.Rect.Min.X, m.Rect.Max.X)
		}
	}

	// An empty quote is only its padding, and draws nothing.
	e = newViewEnv(t, ">", Light, metric1)
	dims, tr = e.layout(0, 800)
	if dims.Size.Y != 90 {
		t.Fatalf("empty quote is %d px tall, want 90", dims.Size.Y)
	}
	if len(tr.Marks) != 0 {
		t.Fatalf("empty quote recorded %d marks, want none", len(tr.Marks))
	}

	e = newViewEnv(t, "> a", Dark, metric1)
	_, tr = e.layout(0, 800)
	bars = needMarks(t, tr, MarkQuoteBar, "QuoteBar")
	if len(bars) != 1 || bars[0].Colour != Dark.Border {
		t.Fatalf("dark quote bar %v, want one in Dark.Border %v", bars, Dark.Border)
	}
	if g := glyphMark(t, tr, "a"); g.Colour != Dark.FgMuted {
		t.Fatalf("dark quoted text colour %v, want FgMuted %v", g.Colour, Dark.FgMuted)
	}
}

// TestBlockBaseline: a text block's Baseline is its first text line's distance from the bottom,
// so Size.Y less Baseline is the first baseline from the top. A heading's rule and padding are
// below that line; a code block's padding is above it; a quote's first line is its first child's
// (STYLE-TEXT-1, STYLE-TEXT-2, STYLE-CODE-1, STYLE-QUOTE-1).
func TestBlockBaseline(t *testing.T) {
	tests := []struct {
		name   string
		src    string
		height int // Size.Y, or 0 for no check
		first  int // the first baseline from the top
	}{
		{name: "paragraph", src: "a", first: 18},
		{name: "heading 1", src: "# h", height: 51, first: 32},
		{name: "code block", src: "```\nx\n```", height: 52, first: 31},
		{name: "quote of a paragraph", src: "> a", first: 18},
		{name: "quote of a heading", src: "> # h\n>\n> b", height: 91, first: 32},
	}
	ctx := newBlockCtx(t, Light)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dims, tr := layBlock(t, ctx, firstBlock(t, tt.src), 800)
			if len(tr.Marks) == 0 {
				t.Errorf("%q draws no marks", tt.src)
			}
			if tt.height != 0 && dims.Size.Y != tt.height {
				t.Errorf("%q is %d px tall, want %d", tt.src, dims.Size.Y, tt.height)
			}
			if got := dims.Size.Y - dims.Baseline; got != tt.first {
				t.Errorf("%q has its first baseline %d px from the top, want %d", tt.src, got, tt.first)
			}
		})
	}
}
