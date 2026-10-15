package doc

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/yuin/goldmark/ast"
	extast "github.com/yuin/goldmark/extension/ast"
)

// blockCase is one row of a table: Markdown source and the blocks Parse must return for it.
type blockCase struct {
	src  string
	want []Block
}

// parseBlocks parses src as a document in /doc and returns its blocks.
func parseBlocks(t *testing.T, src string) []Block {
	t.Helper()
	d, err := Parse([]byte(src), "/doc")
	if err != nil {
		t.Fatalf("Parse(%q) error = %v, want nil", src, err)
	}
	return d.Blocks
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

func TestHeadings(t *testing.T) {
	checkCases(t, []blockCase{
		{"# a", []Block{hd(1, txt("a", 0))}},
		{"## a", []Block{hd(2, txt("a", 0))}},
		{"### a", []Block{hd(3, txt("a", 0))}},
		{"#### a", []Block{hd(4, txt("a", 0))}},
		{"##### a", []Block{hd(5, txt("a", 0))}},
		{"###### a", []Block{hd(6, txt("a", 0))}},
		{"a\n===", []Block{hd(1, txt("a", 0))}},
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
	})
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
			_, err := convertBlock(c.node, nil)
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

// TestEveryKindIsMapped parses the M1 constant and checks that it reaches every M1 kind.
func TestEveryKindIsMapped(t *testing.T) {
	d, err := Parse([]byte(m1Kinds), "/doc")
	if err != nil {
		t.Fatalf("Parse(m1Kinds) error = %v, want nil", err)
	}
	kinds := map[string]bool{}
	tasks := map[Task]bool{}
	var styles Style
	hasBreak := false
	walkBlocks(d.Blocks, func(b Block) {
		kinds[reflectName(b)] = true
		if l, ok := b.(List); ok {
			for _, it := range l.Items {
				tasks[it.Task] = true
			}
		}
		for _, r := range runsOf(b) {
			styles |= r.Style
			if r.Kind == BreakRun {
				hasBreak = true
			}
		}
	})
	wantKinds := map[string]bool{
		"Heading": true, "Paragraph": true, "CodeBlock": true,
		"Quote": true, "List": true, "Rule": true,
	}
	if !reflect.DeepEqual(kinds, wantKinds) {
		t.Errorf("block kinds = %v, want %v", kinds, wantKinds)
	}
	if !tasks[Open] || !tasks[Done] {
		t.Errorf("task states = %v, want Open and Done both present", tasks)
	}
	if !hasBreak {
		t.Errorf("no BreakRun found, want one")
	}
	for _, f := range []struct {
		name string
		s    Style
	}{{"Emphasis", Emphasis}, {"Strong", Strong}, {"Strike", Strike}, {"Code", Code}} {
		if styles&f.s == 0 {
			t.Errorf("style %s never found", f.name)
		}
	}
}

// reflectName returns the type name of a block, for example "Heading".
func reflectName(b Block) string {
	return reflect.TypeOf(b).Name()
}

// TestParseDeterministic parses the M1 constant twice and expects equal models (DOC-14).
func TestParseDeterministic(t *testing.T) {
	first, err := Parse([]byte(m1Kinds), "/doc")
	if err != nil {
		t.Fatalf("first Parse(m1Kinds) error = %v, want nil", err)
	}
	second, err := Parse([]byte(m1Kinds), "/doc")
	if err != nil {
		t.Fatalf("second Parse(m1Kinds) error = %v, want nil", err)
	}
	if len(first.Blocks) == 0 {
		t.Errorf("Parse(m1Kinds).Blocks is empty, want blocks")
	}
	if !reflect.DeepEqual(first, second) {
		t.Errorf("two parses differ:\n%#v\nand\n%#v", first, second)
	}
}

// TestInvalidUTF8: invalid bytes are not an error and pass through unchanged (DOC-15).
func TestInvalidUTF8(t *testing.T) {
	checkCases(t, []blockCase{
		{"a\xffb", []Block{para(txt("a\xffb", 0))}},
	})
}
