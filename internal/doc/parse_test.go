package doc

import (
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
)

// elementsPath is the fixture that every supported element appears in once (ARCHITECTURE
// "Fixtures in testdata/"). Tests in this package read it relative to the package directory.
const elementsPath = "../../testdata/elements.md"

// blockCase is one row of a table: Markdown source and the blocks Parse must return for it.
type blockCase struct {
	src  string
	want []Block
}

// parseDoc parses src as a document in /doc and returns the whole document.
func parseDoc(t *testing.T, src string) *Document {
	t.Helper()
	d, err := Parse([]byte(src), "/doc")
	if err != nil {
		t.Fatalf("Parse(%q) error = %v, want nil", src, err)
	}
	return d
}

// parseBlocks parses src as a document in /doc and returns its blocks.
func parseBlocks(t *testing.T, src string) []Block {
	t.Helper()
	return parseDoc(t, src).Blocks
}

// checkBlocks asserts that Parse(src).Blocks equals want.
func checkBlocks(t *testing.T, src string, want []Block) {
	t.Helper()
	if got := parseBlocks(t, src); !reflect.DeepEqual(got, want) {
		t.Errorf("Parse(%q).Blocks =\n%#v\nwant\n%#v", src, got, want)
	}
}

// checkCases runs each row as a subtest named by its source.
func checkCases(t *testing.T, cases []blockCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(fmt.Sprintf("%q", c.src), func(t *testing.T) {
			checkBlocks(t, c.src, c.want)
		})
	}
}

func txt(s string, st Style) Run { return Run{Kind: TextRun, Text: s, Style: st} }
func brk() Run                   { return Run{Kind: BreakRun} }
func para(runs ...Run) Paragraph { return Paragraph{Runs: runs} }
func hd(level int, runs ...Run) Heading {
	return Heading{Level: level, Runs: runs}
}
func li(task Task, bs ...Block) Item { return Item{Task: task, Blocks: bs} }

// lnk returns a text run with the given text, style and link target (DOC-4).
func lnk(text string, st Style, tg Target) Run {
	return Run{Kind: TextRun, Text: text, Style: st, Link: &tg}
}

// cell returns a table cell holding the given runs (DOC-10).
func cell(runs ...Run) Cell { return Cell{Runs: runs} }

func TestHeadings(t *testing.T) {
	checkCases(t, []blockCase{
		{"# a", []Block{hd(1, txt("a", 0))}},
		{"## a", []Block{hd(2, txt("a", 0))}},
		{"### a", []Block{hd(3, txt("a", 0))}},
		{"#### a", []Block{hd(4, txt("a", 0))}},
		{"##### a", []Block{hd(5, txt("a", 0))}},
		{"###### a", []Block{hd(6, txt("a", 0))}},
		{"a\n===", []Block{hd(1, txt("a", 0))}},
		{"Foo\nbar\n===", []Block{hd(1, txt("Foo bar", 0))}},
		{"a\n---", []Block{hd(2, txt("a", 0))}},
		{"# a ##", []Block{hd(1, txt("a", 0))}},
		{"## *a* b", []Block{hd(2, txt("a", Emphasis), txt(" b", 0))}},
		{"####### a", []Block{para(txt("####### a", 0))}},
	})
}

func TestParagraphRuns(t *testing.T) {
	checkCases(t, []blockCase{
		{"*a*", []Block{para(txt("a", Emphasis))}},
		{"_a_", []Block{para(txt("a", Emphasis))}},
		{"**a**", []Block{para(txt("a", Strong))}},
		{"__a__", []Block{para(txt("a", Strong))}},
		{"***a***", []Block{para(txt("a", Emphasis|Strong))}},
		{"~~a~~", []Block{para(txt("a", Strike))}},
		{"`a`", []Block{para(txt("a", Code))}},
		{"**`a`**", []Block{para(txt("a", Strong|Code))}},
		{"`**a**`", []Block{para(txt("**a**", Code))}},
		{"x *a* **b** `c` ~~d~~ y", []Block{para(
			txt("x ", 0), txt("a", Emphasis), txt(" ", 0), txt("b", Strong),
			txt(" ", 0), txt("c", Code), txt(" ", 0), txt("d", Strike), txt(" y", 0),
		)}},
		{"*a **b** c*", []Block{para(
			txt("a ", Emphasis), txt("b", Emphasis|Strong), txt(" c", Emphasis),
		)}},
		{"a\nb", []Block{para(txt("a b", 0))}},
		{"*a\nb*", []Block{para(txt("a b", Emphasis))}},
	})
}

func TestSoftAndHardBreaks(t *testing.T) {
	checkCases(t, []blockCase{
		{"a\nb", []Block{para(txt("a b", 0))}},
		{"a  \nb", []Block{para(txt("a", 0), brk(), txt("b", 0))}},
		{"a\\\nb", []Block{para(txt("a", 0), brk(), txt("b", 0))}},
		{"`a\nb`", []Block{para(txt("a b", Code))}},
		{"*a  \nb*", []Block{para(txt("a", Emphasis), brk(), txt("b", Emphasis))}},
	})
}

func TestWhitespace(t *testing.T) {
	checkCases(t, []blockCase{
		{"a   b", []Block{para(txt("a b", 0))}},
		{"a\t\tb", []Block{para(txt("a b", 0))}},
		{"`a   b`", []Block{para(txt("a   b", Code))}},
		{"`a\tb`", []Block{para(txt("a b", Code))}},
		{"   a   ", []Block{para(txt("a", 0))}},
	})
}

// TestNormaliseRuns checks the run rules of DOC-3 on hand-built runs: merge, collapse, trim.
func TestNormaliseRuns(t *testing.T) {
	cases := []struct {
		name string
		in   []Run
		want []Run
	}{
		{
			name: "trims the ends and keeps the inner spaces of a later run",
			in:   []Run{txt(" a ", 0), txt(" b ", Strong)},
			want: []Run{txt("a ", 0), txt(" b", Strong)},
		},
		{
			name: "merges adjacent runs with equal style",
			in:   []Run{txt("a", 0), txt("b", 0)},
			want: []Run{txt("ab", 0)},
		},
		{
			name: "drops a run emptied by the trim",
			in:   []Run{txt("   ", 0)},
			want: nil,
		},
		{
			name: "does not trim or collapse Code runs at the end",
			in:   []Run{txt("a  b", 0), txt("  ", Code)},
			want: []Run{txt("a b", 0), txt("  ", Code)},
		},
		{
			name: "turns a tab in Code into one space",
			in:   []Run{txt("a\tb", Code)},
			want: []Run{txt("a b", Code)},
		},
		{
			name: "keeps breaks",
			in:   []Run{txt("a", 0), brk(), txt("b", 0)},
			want: []Run{txt("a", 0), brk(), txt("b", 0)},
		},
		{
			name: "trims the start into the next run when the first is emptied",
			in:   []Run{txt(" ", 0), txt(" a", Strong)},
			want: []Run{txt("a", Strong)},
		},
		{
			name: "trims the end into the previous run when the last is emptied",
			in:   []Run{txt("a ", Strong), txt(" ", 0)},
			want: []Run{txt("a", Strong)},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := normaliseRuns(c.in); !reflect.DeepEqual(got, c.want) {
				t.Errorf("normaliseRuns(%#v) =\n%#v\nwant\n%#v", c.in, got, c.want)
			}
		})
	}
}

func TestCodeBlocks(t *testing.T) {
	checkCases(t, []blockCase{
		{"```go\nfunc f() {}\n```", []Block{CodeBlock{Lang: "go", Text: "func f() {}"}}},
		{"```go extra\nx\n```", []Block{CodeBlock{Lang: "go", Text: "x"}}},
		{"```\na\n\n```", []Block{CodeBlock{Text: "a\n"}}},
		{"    a\n    b\n", []Block{CodeBlock{Text: "a\nb"}}},
		{"    \ta\n", []Block{CodeBlock{Text: strings.Repeat(" ", 8) + "a"}}},
		{"- a\n\n  ```\n  x\n  ```", []Block{List{Tight: false, Start: 1, Items: []Item{
			li(NotTask, para(txt("a", 0)), CodeBlock{Text: "x"}),
		}}}},
		{"```\na\tb\n```", []Block{CodeBlock{Text: "a" + strings.Repeat(" ", 7) + "b"}}},
		{"```\n\tx\n```", []Block{CodeBlock{Text: strings.Repeat(" ", 8) + "x"}}},
		{"```\na\tbc\td\n```", []Block{CodeBlock{
			Text: "a" + strings.Repeat(" ", 7) + "bc" + strings.Repeat(" ", 6) + "d",
		}}},
		{"```\né\tx\n```", []Block{CodeBlock{Text: "é" + strings.Repeat(" ", 7) + "x"}}},
		{"```\n```", []Block{CodeBlock{Text: ""}}},
	})
}

func TestQuotesAndRules(t *testing.T) {
	checkCases(t, []blockCase{
		{"> a\n>\n> > b", []Block{Quote{Blocks: []Block{
			para(txt("a", 0)),
			Quote{Blocks: []Block{para(txt("b", 0))}},
		}}}},
		{"---", []Block{Rule{}}},
		{"***", []Block{Rule{}}},
		{"___", []Block{Rule{}}},
		{"a\n\n---", []Block{para(txt("a", 0)), Rule{}}},
	})
}

func TestLists(t *testing.T) {
	checkCases(t, []blockCase{
		{"- a\n- b", []Block{List{Tight: true, Start: 1, Items: []Item{
			li(NotTask, para(txt("a", 0))),
			li(NotTask, para(txt("b", 0))),
		}}}},
		{"3. a\n4. b", []Block{List{Ordered: true, Tight: true, Start: 3, Items: []Item{
			li(NotTask, para(txt("a", 0))),
			li(NotTask, para(txt("b", 0))),
		}}}},
		{"- a\n  - b\n    - c", []Block{List{Tight: true, Start: 1, Items: []Item{
			li(NotTask,
				para(txt("a", 0)),
				List{Tight: true, Start: 1, Items: []Item{
					li(NotTask,
						para(txt("b", 0)),
						List{Tight: true, Start: 1, Items: []Item{
							li(NotTask, para(txt("c", 0))),
						}},
					),
				}},
			),
		}}}},
		{"- a\n\n- b", []Block{List{Tight: false, Start: 1, Items: []Item{
			li(NotTask, para(txt("a", 0))),
			li(NotTask, para(txt("b", 0))),
		}}}},
		{"- [ ] a\n- [x] b\n- [X] c", []Block{List{Tight: true, Start: 1, Items: []Item{
			li(Open, para(txt("a", 0))),
			li(Done, para(txt("b", 0))),
			li(Done, para(txt("c", 0))),
		}}}},
		{"- [ ] a\n- b", []Block{List{Tight: true, Start: 1, Items: []Item{
			li(Open, para(txt("a", 0))),
			li(NotTask, para(txt("b", 0))),
		}}}},
		{"- [x] a\n\n- [ ] b", []Block{List{Tight: false, Start: 1, Items: []Item{
			li(Done, para(txt("a", 0))),
			li(Open, para(txt("b", 0))),
		}}}},
		{"-", []Block{List{Tight: true, Start: 1, Items: []Item{li(NotTask)}}}},
		{"- [ ]", []Block{List{Tight: true, Start: 1, Items: []Item{li(Open, para())}}}},
		{"0. a", []Block{List{Ordered: true, Tight: true, Start: 0, Items: []Item{
			li(NotTask, para(txt("a", 0))),
		}}}},
	})
}

func TestUnsupportedSyntaxIsText(t *testing.T) {
	checkCases(t, []blockCase{
		{"Text[^1].\n\n[^1]: A footnote.", []Block{
			para(txt("Text[^1].", 0)),
			para(txt("[^1]: A footnote.", 0)),
		}},
		{"Hi :smile:", []Block{para(txt("Hi :smile:", 0))}},
		{"Term\n: definition", []Block{para(txt("Term : definition", 0))}},
		{"# Title {#id}", []Block{hd(1, txt("Title {#id}", 0))}},
		{"\"q\" -- a...", []Block{para(txt("\"q\" -- a...", 0))}},
	})
}

// TestFootnoteDefinitionCaveat: DOC-1's one-word caveat. A valid link reference definition
// with the label ^1 gives no block, and the reference [^1] becomes a link (DOC-2).
func TestFootnoteDefinitionCaveat(t *testing.T) {
	checkCases(t, []blockCase{
		{"[^1]: note", nil},
		{"a[^1]\n\n[^1]: note", []Block{
			para(txt("a", 0), lnk("^1", 0, Target{LocalFile, "/doc/note", "note"})),
		}},
	})
}

// TestLinkReferenceDefinitionDropped: a link reference definition produces no block (DOC-2).
func TestLinkReferenceDefinitionDropped(t *testing.T) {
	checkCases(t, []blockCase{
		{"[a]: /url\n\nb", []Block{para(txt("b", 0))}},
	})
}

// TestEscapesAndEntities: escapes and references resolve outside Code; Code is raw (DOC-3).
func TestEscapesAndEntities(t *testing.T) {
	checkCases(t, []blockCase{
		{"\\*a\\*", []Block{para(txt("*a*", 0))}},
		{"a &amp; b", []Block{para(txt("a & b", 0))}},
		{"&#35; x", []Block{para(txt("# x", 0))}},
		{"`\\*`", []Block{para(txt("\\*", Code))}},
		{"\\&amp;", []Block{para(txt("&amp;", 0))}},
		{"&#32;*a*", []Block{para(txt("a", Emphasis))}},
	})
}

// TestLinkRuns checks DOC-4's runs and DOC-5's targets for links and autolinks.
func TestLinkRuns(t *testing.T) {
	lx := Target{LocalFile, "/doc/x.md", "x.md"}
	le := Target{External, "https://e.org/", "https://e.org/"}
	checkCases(t, []blockCase{
		{"[a *b*](x.md)", []Block{para(lnk("a ", 0, lx), lnk("b", Emphasis, lx))}},
		{"<https://e.org/>", []Block{para(lnk("https://e.org/", 0, le))}},
		{"https://e.org/", []Block{para(lnk("https://e.org/", 0, le))}},
		{"a https://e.org/ b", []Block{para(txt("a ", 0), lnk("https://e.org/", 0, le), txt(" b", 0))}},
		{"<a@example.com>", []Block{para(lnk("a@example.com", 0, Target{
			External, "mailto:a@example.com", "mailto:a@example.com",
		}))}},
		{"a@example.com", []Block{para(lnk("a@example.com", 0, Target{
			External, "mailto:a@example.com", "mailto:a@example.com",
		}))}},
		{"<https://e.org/a\\_b>", []Block{para(lnk("https://e.org/a\\_b", 0, Target{
			External, "https://e.org/a\\_b", "https://e.org/a\\_b",
		}))}},
		{"www.e.org", []Block{para(lnk("www.e.org", 0, Target{
			External, "http://www.e.org", "http://www.e.org",
		}))}},
		{"[a](x.md)[b](x.md)", []Block{para(lnk("ab", 0, lx))}},
		{"*[a](x.md)*", []Block{para(lnk("a", Emphasis, lx))}},
		{"[`c`](x.md)", []Block{para(lnk("c", Code, lx))}},
		{"[a  \nb](x.md)", []Block{para(lnk("a", 0, lx), brk(), lnk("b", 0, lx))}},
		{"[a](<b c.md>)", []Block{para(lnk("a", 0, Target{LocalFile, "/doc/b c.md", "b c.md"}))}},
		{"[a](x.md \"title\")", []Block{para(lnk("a", 0, lx))}},
		{"[a](x\\_y.md)", []Block{para(lnk("a", 0, Target{LocalFile, "/doc/x_y.md", "x_y.md"}))}},
		{"[a]\n\n[a]: /u", []Block{para(lnk("a", 0, Target{LocalFile, "/u", "/u"}))}},
		{"# [a](x.md)", []Block{hd(1, lnk("a", 0, lx))}},
	})
}

// TestLinkSharesOneTarget: the runs of one link node share one *Target (DOC-4).
func TestLinkSharesOneTarget(t *testing.T) {
	bs := parseBlocks(t, "[a *b*](x.md)")
	if len(bs) != 1 {
		t.Fatalf("Parse blocks = %d, want 1", len(bs))
	}
	runs := runsOf(bs[0])
	if len(runs) != 2 {
		t.Fatalf("runs = %d, want 2", len(runs))
	}
	if runs[0].Link == nil || runs[0].Link != runs[1].Link {
		t.Errorf("runs of one link have links %p and %p, want one shared *Target", runs[0].Link, runs[1].Link)
	}
}

// TestTables checks DOC-10: alignments, header and body cells, padding and extra cells.
func TestTables(t *testing.T) {
	lx := Target{LocalFile, "/doc/x.md", "x.md"}
	checkCases(t, []blockCase{
		{"| a | b | c | d |\n| :-- | :-: | --: | --- |\n| 1 | 2 | 3 | 4 |", []Block{Table{
			Align:  []Align{AlignLeft, AlignCentre, AlignRight, AlignNone},
			Header: []Cell{cell(txt("a", 0)), cell(txt("b", 0)), cell(txt("c", 0)), cell(txt("d", 0))},
			Rows: [][]Cell{{
				cell(txt("1", 0)), cell(txt("2", 0)), cell(txt("3", 0)), cell(txt("4", 0)),
			}},
		}}},
		{"| a | b |\n| - | - |\n| 1 |", []Block{Table{
			Align:  []Align{AlignNone, AlignNone},
			Header: []Cell{cell(txt("a", 0)), cell(txt("b", 0))},
			Rows:   [][]Cell{{cell(txt("1", 0)), {}}},
		}}},
		{"| a |\n| - |\n| 1 | 2 |", []Block{Table{
			Align:  []Align{AlignNone},
			Header: []Cell{cell(txt("a", 0))},
			Rows:   [][]Cell{{cell(txt("1", 0))}},
		}}},
		{"| *a* | `b` |\n| - | - |\n| [c](x.md) | d \\| e |", []Block{Table{
			Align:  []Align{AlignNone, AlignNone},
			Header: []Cell{cell(txt("a", Emphasis)), cell(txt("b", Code))},
			Rows:   [][]Cell{{cell(lnk("c", 0, lx)), cell(txt("d | e", 0))}},
		}}},
		{"| a |\n| - |", []Block{Table{
			Align:  []Align{AlignNone},
			Header: []Cell{cell(txt("a", 0))},
		}}},
		{"> | a |\n> | - |\n> | 1 |", []Block{Quote{Blocks: []Block{Table{
			Align:  []Align{AlignNone},
			Header: []Cell{cell(txt("a", 0))},
			Rows:   [][]Cell{{cell(txt("1", 0))}},
		}}}}},
	})
}

// TestImagesSplitParagraphs checks DOC-11: a paragraph is split at each image, and an image in
// a heading or a cell becomes its alt text.
func TestImagesSplitParagraphs(t *testing.T) {
	img := Image{Alt: "x", Dest: "i.png", Target: Target{LocalFile, "/doc/i.png", "i.png"}}
	le := Target{External, "https://e.org/", "https://e.org/"}
	linked := img
	linked.Link = &le
	j := Image{Alt: "b", Dest: "j.png", Target: Target{LocalFile, "/doc/j.png", "j.png"}}
	remote := Image{
		Alt:    "x",
		Dest:   "https://e.org/i.png",
		Target: Target{External, "https://e.org/i.png", "https://e.org/i.png"},
	}
	checkCases(t, []blockCase{
		{"a ![x](i.png) b", []Block{para(txt("a", 0)), img, para(txt("b", 0))}},
		{"![x](i.png)", []Block{img}},
		{"![a](i.png)![b](j.png)", []Block{
			Image{Alt: "a", Dest: "i.png", Target: img.Target}, j,
		}},
		{"[![x](i.png)](https://e.org/)", []Block{linked}},
		{"[a ![x](i.png) b](https://e.org/)", []Block{
			para(lnk("a", 0, le)), linked, para(lnk("b", 0, le)),
		}},
		{"a\n![x](i.png)\nb", []Block{para(txt("a", 0)), img, para(txt("b", 0))}},
		{"> a ![x](i.png)", []Block{Quote{Blocks: []Block{para(txt("a", 0)), img}}}},
		{"- ![x](i.png)", []Block{List{Tight: true, Start: 1, Items: []Item{li(NotTask, img)}}}},
		{"![x *y*](i.png)", []Block{Image{Alt: "x y", Dest: "i.png", Target: img.Target}}},
		{"![](i.png)", []Block{Image{Alt: "", Dest: "i.png", Target: img.Target}}},
		{"![`c` \\* &amp;](i.png)", []Block{Image{Alt: "c * &", Dest: "i.png", Target: img.Target}}},
		{"![x](https://e.org/i.png)", []Block{remote}},
		{"# h ![x](i.png)", []Block{hd(1, txt("h x", 0))}},
		{"| ![x](i.png) |\n| - |\n| c |", []Block{Table{
			Align:  []Align{AlignNone},
			Header: []Cell{cell(txt("x", 0))},
			Rows:   [][]Cell{{cell(txt("c", 0))}},
		}}},
	})
}

// TestHTMLDropped checks DOC-9: HTML produces no content, its text is kept, and DroppedHTML
// counts the fragments (one HTML block or one inline tag or comment each).
func TestHTMLDropped(t *testing.T) {
	cases := []struct {
		src     string
		want    []Block
		dropped int
	}{
		{"<div>\nhi\n</div>", nil, 1},
		{"<!-- c -->", nil, 1},
		{"a <span>b</span> c", []Block{para(txt("a b c", 0))}, 2},
		{"a <b\nc='d'> e", []Block{para(txt("a e", 0))}, 1},
		{"# a <b>c</b>", []Block{hd(1, txt("a c", 0))}, 2},
		{"> <div>x</div>", []Block{Quote{}}, 1},
		{"x\n\n<div>y</div>\n\nz <i>w</i>", []Block{para(txt("x", 0)), para(txt("z w", 0))}, 3},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%q", c.src), func(t *testing.T) {
			d := parseDoc(t, c.src)
			if !reflect.DeepEqual(d.Blocks, c.want) {
				t.Errorf("Parse(%q).Blocks =\n%#v\nwant\n%#v", c.src, d.Blocks, c.want)
			}
			if d.DroppedHTML != c.dropped {
				t.Errorf("Parse(%q).DroppedHTML = %d, want %d", c.src, d.DroppedHTML, c.dropped)
			}
		})
	}
}

// TestUnmappedKindIsError: a kind with no mapping returns an error naming it (DOC-2).
func TestUnmappedKindIsError(t *testing.T) {
	inline := ast.NewParagraph()
	inline.AppendChild(inline, extast.NewFootnoteLink(1))
	cases := []struct {
		name string
		node ast.Node
		kind string
	}{
		{"block", extast.NewFootnoteList(), "FootnoteList"},
		{"inline", inline, "FootnoteLink"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := (&converter{}).convertBlock(c.node)
			if err == nil {
				t.Fatalf("convertBlock(%s) error = nil, want an error naming %q", c.name, c.kind)
			}
			if !strings.Contains(err.Error(), c.kind) {
				t.Errorf("convertBlock(%s) error = %q, want it to name %q", c.name, err, c.kind)
			}
		})
	}
}

// m1Kinds holds one of every M1 construct: every block kind, every style flag, both breaks,
// both checkbox states, a nested list and a loose list. The hard break is the two trailing
// spaces on the "code" line.
const m1Kinds = "# Title\n" +
	"\n" +
	"Setext\n" +
	"======\n" +
	"\n" +
	"Para with *em*, **strong**, ~~strike~~, `code`,  \n" +
	"and a soft\n" +
	"break.\n" +
	"\n" +
	"```go info\n" +
	"func f() {}\n" +
	"```\n" +
	"\n" +
	"    indented\n" +
	"\n" +
	"> quote\n" +
	">\n" +
	"> > nested\n" +
	"\n" +
	"***\n" +
	"\n" +
	"- item one\n" +
	"  1. nested\n" +
	"\n" +
	"- [ ] open\n" +
	"- [x] done\n"

// elementsKinds are the goldmark node kinds that GFM produces and that ARCHITECTURE says Parse
// handles, except String, which only the typographer extension creates. elements.md must
// reach each of them (DOC-2).
var elementsKinds = []string{
	"Document", "Heading", "Paragraph", "TextBlock", "CodeBlock", "FencedCodeBlock",
	"Blockquote", "List", "ListItem", "ThematicBreak", "HTMLBlock", "Text", "CodeSpan",
	"Emphasis", "Link", "AutoLink", "Image", "RawHTML", "LinkReferenceDefinition", "Table",
	"TableHeader", "TableRow", "TableCell", "Strikethrough", "TaskCheckBox",
}

// walkBlocks calls visit on each block, then on the blocks inside it.
func walkBlocks(bs []Block, visit func(Block)) {
	for _, b := range bs {
		visit(b)
		switch b := b.(type) {
		case Quote:
			walkBlocks(b.Blocks, visit)
		case List:
			for _, it := range b.Items {
				walkBlocks(it.Blocks, visit)
			}
		default:
			// Other kinds hold no child blocks.
		}
	}
}

// runsOf returns the runs of a heading or paragraph, and none for other kinds.
func runsOf(b Block) []Run {
	switch b := b.(type) {
	case Heading:
		return b.Runs
	case Paragraph:
		return b.Runs
	default:
		return nil
	}
}

// coverage records what a model reaches: block kinds, task states, style flags, breaks,
// heading levels, code languages, link target kinds, and whether an image is linked.
type coverage struct {
	kinds     map[string]bool
	tasks     map[Task]bool
	styles    Style
	hasBreak  bool
	levels    map[int]bool
	langs     map[string]bool
	linkKinds map[TargetKind]bool
	linked    bool
}

// coverageOf walks a model and records its coverage.
func coverageOf(bs []Block) coverage {
	c := coverage{
		kinds:     map[string]bool{},
		tasks:     map[Task]bool{},
		levels:    map[int]bool{},
		langs:     map[string]bool{},
		linkKinds: map[TargetKind]bool{},
	}
	walkBlocks(bs, func(b Block) {
		c.kinds[reflectName(b)] = true
		switch b := b.(type) {
		case Heading:
			c.levels[b.Level] = true
		case CodeBlock:
			c.langs[b.Lang] = true
		case List:
			for _, it := range b.Items {
				c.tasks[it.Task] = true
			}
		case Image:
			c.linked = c.linked || b.Link != nil
		default:
			// Other kinds add nothing beyond their runs, checked below.
		}
		for _, r := range runsOf(b) {
			c.styles |= r.Style
			if r.Kind == BreakRun {
				c.hasBreak = true
			}
			if r.Link != nil {
				c.linkKinds[r.Link.Kind] = true
			}
		}
	})
	return c
}

// goldmarkKinds parses src with goldmark and GFM, as Parse does, and returns the names of the
// node kinds in the tree.
func goldmarkKinds(t *testing.T, src string) map[string]bool {
	t.Helper()
	md := goldmark.New(goldmark.WithExtensions(extension.GFM))
	root := md.Parser().Parse(text.NewReader([]byte(src)))
	seen := map[string]bool{}
	err := ast.Walk(root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if entering {
			seen[n.Kind().String()] = true
		}
		return ast.WalkContinue, nil
	})
	if err != nil {
		t.Fatalf("ast.Walk error = %v, want nil", err)
	}
	return seen
}

// readElements returns the text of elements.md.
func readElements(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(elementsPath)
	if err != nil {
		t.Fatalf("read %s: %v", elementsPath, err)
	}
	return string(b)
}

// checkStyles asserts that each of the four style flags occurs.
func checkStyles(t *testing.T, styles Style) {
	t.Helper()
	for _, f := range []struct {
		name string
		s    Style
	}{{"Emphasis", Emphasis}, {"Strong", Strong}, {"Strike", Strike}, {"Code", Code}} {
		if styles&f.s == 0 {
			t.Errorf("style %s never found", f.name)
		}
	}
}

// TestEveryKindIsMapped parses the M1 constant and elements.md, and checks that each reaches
// every kind it should (DOC-2). M1's expectations for m1Kinds are unchanged.
func TestEveryKindIsMapped(t *testing.T) {
	t.Run("m1Kinds", func(t *testing.T) {
		c := coverageOf(parseDoc(t, m1Kinds).Blocks)
		wantKinds := map[string]bool{
			"Heading": true, "Paragraph": true, "CodeBlock": true,
			"Quote": true, "List": true, "Rule": true,
		}
		if !reflect.DeepEqual(c.kinds, wantKinds) {
			t.Errorf("block kinds = %v, want %v", c.kinds, wantKinds)
		}
		if !c.tasks[Open] || !c.tasks[Done] {
			t.Errorf("task states = %v, want Open and Done both present", c.tasks)
		}
		if !c.hasBreak {
			t.Errorf("no BreakRun found, want one")
		}
		checkStyles(t, c.styles)
	})
	t.Run("elements.md", func(t *testing.T) {
		src := readElements(t)
		d := parseDoc(t, src)
		c := coverageOf(d.Blocks)
		wantKinds := map[string]bool{
			"Heading": true, "Paragraph": true, "CodeBlock": true, "Quote": true,
			"List": true, "Table": true, "Rule": true, "Image": true,
		}
		if !reflect.DeepEqual(c.kinds, wantKinds) {
			t.Errorf("block kinds = %v, want %v", c.kinds, wantKinds)
		}
		if !c.tasks[Open] || !c.tasks[Done] {
			t.Errorf("task states = %v, want Open and Done both present", c.tasks)
		}
		if !c.hasBreak {
			t.Errorf("no BreakRun found, want one")
		}
		checkStyles(t, c.styles)
		for lvl := 1; lvl <= 6; lvl++ {
			if !c.levels[lvl] {
				t.Errorf("heading level %d never found", lvl)
			}
		}
		if !c.langs["go"] || !c.langs[""] {
			t.Errorf("code languages = %v, want \"go\" and \"\" both present", c.langs)
		}
		if !c.linkKinds[LocalFile] || !c.linkKinds[External] {
			t.Errorf("link kinds = %v, want LocalFile and External both present", c.linkKinds)
		}
		if !c.linked {
			t.Errorf("no linked Image found, want one")
		}
		if d.DroppedHTML != 4 {
			t.Errorf("DroppedHTML = %d, want 4", d.DroppedHTML)
		}
		seen := goldmarkKinds(t, src)
		for _, k := range elementsKinds {
			if !seen[k] {
				t.Errorf("goldmark kind %s not in elements.md", k)
			}
		}
	})
}

// reflectName returns the type name of a block, for example "Heading".
func reflectName(b Block) string {
	return reflect.TypeOf(b).Name()
}

// TestParseDeterministic parses each source twice and expects equal models (DOC-14).
func TestParseDeterministic(t *testing.T) {
	t.Run("m1Kinds", func(t *testing.T) {
		checkDeterministic(t, m1Kinds)
	})
	t.Run("elements.md", func(t *testing.T) {
		checkDeterministic(t, readElements(t))
	})
}

// checkDeterministic parses src twice and checks that the models are equal and not empty.
func checkDeterministic(t *testing.T, src string) {
	t.Helper()
	first := parseDoc(t, src)
	second := parseDoc(t, src)
	if len(first.Blocks) == 0 {
		t.Errorf("Parse(src).Blocks is empty, want blocks")
	}
	if !reflect.DeepEqual(first, second) {
		t.Errorf("two parses differ:\n%#v\nand\n%#v", first, second)
	}
}

// TestInvalidUTF8: invalid bytes are not an error and pass through unchanged (DOC-15).
func TestInvalidUTF8(t *testing.T) {
	checkCases(t, []blockCase{
		{"a\xffb", []Block{para(txt("a\xffb", 0))}},
		{"`a\xff`", []Block{para(txt("a\xff", Code))}},
		{"```\n\xffa\tb\n```", []Block{CodeBlock{Text: "\xffa" + strings.Repeat(" ", 6) + "b"}}},
	})
}

// TestLineEndings: CRLF and a lone CR become LF before goldmark sees the source (DOC-7), so
// no text carries a CR.
func TestLineEndings(t *testing.T) {
	checkCases(t, []blockCase{
		{"```\r\na\tb\r\n```", []Block{CodeBlock{Text: "a" + strings.Repeat(" ", 7) + "b"}}},
		{"a  \r\nb", []Block{para(txt("a", 0), brk(), txt("b", 0))}},
		{"a\rb", []Block{para(txt("a b", 0))}},
	})
}
