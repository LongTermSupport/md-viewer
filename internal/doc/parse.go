package doc

import (
	"bytes"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// Values from SPEC, each written once (DOC-n cited beside it).
const (
	codeTabWidth   = 8 // DOC-7: tabs in code expand to the next multiple of this many columns
	unorderedStart = 1 // DOC-8: Start of an unordered list
	emphasisLevel  = 1 // DOC-3: goldmark Emphasis Level 1 is Emphasis
	strongLevel    = 2 // DOC-3: goldmark Emphasis Level 2 is Strong
)

// maxReferenceBytes bounds the scan for the end of a reference. The longest HTML5 entity name
// with its '&' and ';' is 34 bytes. This is a parser bound, not a SPEC value.
const maxReferenceBytes = 34

// converter holds what the conversion of one source needs: the source bytes, the document
// directory that link targets are resolved against (DOC-5), and the count of dropped HTML
// fragments (DOC-9).
type converter struct {
	src         []byte
	dir         string
	droppedHTML int
}

// Parse converts Markdown source into the document model (DOC-1 to DOC-3, DOC-6 to DOC-8,
// DOC-12, DOC-14, DOC-15). dir is stored as Document.Dir.
func Parse(src []byte, dir string) (*Document, error) {
	src = normaliseLineEndings(src)
	md := goldmark.New(goldmark.WithExtensions(extension.GFM))
	root := md.Parser().Parse(text.NewReader(src))
	c := &converter{src: src, dir: dir}
	blocks, err := c.convertBlocks(root)
	if err != nil {
		return nil, err
	}
	return &Document{Dir: dir, Blocks: blocks}, nil
}

// normaliseLineEndings turns each CRLF and each lone CR into LF (DOC-7). It returns a new slice.
func normaliseLineEndings(src []byte) []byte {
	crlf := []byte("\r\n")
	lf := []byte("\n")
	src = bytes.ReplaceAll(src, crlf, lf)
	return bytes.ReplaceAll(src, []byte("\r"), lf)
}

// unmappedKindError reports a goldmark node kind with no model mapping (DOC-2).
type unmappedKindError struct {
	Kind string
}

func (e *unmappedKindError) Error() string {
	return fmt.Sprintf("doc: goldmark kind %q has no model mapping", e.Kind)
}

func unmappedKind(n ast.Node) error {
	return &unmappedKindError{Kind: n.Kind().String()}
}

// convertBlocks converts the block children of parent, in order.
func (c *converter) convertBlocks(parent ast.Node) ([]Block, error) {
	var out []Block
	for n := parent.FirstChild(); n != nil; n = n.NextSibling() {
		bs, err := c.convertBlock(n)
		if err != nil {
			return nil, err
		}
		out = append(out, bs...)
	}
	return out, nil
}

// convertBlock converts one goldmark block node (DOC-2). A link reference definition gives no
// block.
func (c *converter) convertBlock(n ast.Node) ([]Block, error) {
	switch n := n.(type) {
	case *ast.Heading:
		runs, err := c.inlineRuns(n)
		if err != nil {
			return nil, err
		}
		return []Block{Heading{Level: n.Level, Runs: runs}}, nil
	case *ast.Paragraph, *ast.TextBlock:
		runs, err := c.inlineRuns(n)
		if err != nil {
			return nil, err
		}
		return []Block{Paragraph{Runs: runs}}, nil
	case *ast.FencedCodeBlock:
		return []Block{CodeBlock{Lang: string(n.Language(c.src)), Text: codeText(n, c.src)}}, nil
	case *ast.CodeBlock:
		return []Block{CodeBlock{Text: codeText(n, c.src)}}, nil
	case *ast.Blockquote:
		blocks, err := c.convertBlocks(n)
		if err != nil {
			return nil, err
		}
		return []Block{Quote{Blocks: blocks}}, nil
	case *ast.ThematicBreak:
		return []Block{Rule{}}, nil
	case *ast.List:
		list, err := c.convertList(n)
		if err != nil {
			return nil, err
		}
		return []Block{list}, nil
	case *ast.LinkReferenceDefinition:
		return nil, nil
	default:
		return nil, unmappedKind(n)
	}
}

// convertList converts a list and its items (DOC-8).
func (c *converter) convertList(n *ast.List) (List, error) {
	list := List{Ordered: n.IsOrdered(), Tight: n.IsTight, Start: n.Start}
	if !list.Ordered {
		list.Start = unorderedStart
	}
	for ch := n.FirstChild(); ch != nil; ch = ch.NextSibling() {
		item, ok := ch.(*ast.ListItem)
		if !ok {
			return List{}, unmappedKind(ch)
		}
		converted, err := c.convertItem(item)
		if err != nil {
			return List{}, err
		}
		list.Items = append(list.Items, converted)
	}
	return list, nil
}

// convertItem converts a list item: its task state and its child blocks (DOC-8).
func (c *converter) convertItem(n *ast.ListItem) (Item, error) {
	blocks, err := c.convertBlocks(n)
	if err != nil {
		return Item{}, err
	}
	return Item{Task: taskState(n), Blocks: blocks}, nil
}

// taskState reads the checkbox that starts an item's first text block (DOC-8). The checkbox
// itself produces no run (see collectRuns).
func taskState(item ast.Node) Task {
	first := item.FirstChild()
	if first == nil {
		return NotTask
	}
	box, ok := first.FirstChild().(*extast.TaskCheckBox)
	if !ok {
		return NotTask
	}
	if box.IsChecked {
		return Done
	}
	return Open
}

// codeText returns the lines of a code block with tabs expanded and the final newline removed
// (DOC-7).
func codeText(n ast.Node, src []byte) string {
	lines := n.Lines()
	var b strings.Builder
	for i := 0; i < lines.Len(); i++ {
		seg := lines.At(i)
		b.WriteString(expandTabs(string(seg.Value(src))))
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// expandTabs replaces each tab with spaces up to the next multiple of codeTabWidth columns. A
// column is one rune, an invalid byte counts as one, and a newline starts the count again
// (DOC-7).
func expandTabs(s string) string {
	var b strings.Builder
	col := 0
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == '\t':
			n := codeTabWidth - col%codeTabWidth
			b.WriteString(strings.Repeat(" ", n))
			col += n
		case r == '\n':
			b.WriteByte('\n')
			col = 0
		default:
			b.WriteString(s[i : i+size])
			col++
		}
		i += size
	}
	return b.String()
}

// inlineRuns flattens the inline children of a block into normalised runs (DOC-3).
func (c *converter) inlineRuns(n ast.Node) ([]Run, error) {
	var runs []Run
	if err := c.collectRuns(n, 0, nil, &runs); err != nil {
		return nil, err
	}
	return normaliseRuns(runs), nil
}

// collectRuns appends the runs of parent's inline children to out, with the style inherited
// from its markup and the link inherited from the enclosing link, if any. The text is not yet
// normalised.
func (c *converter) collectRuns(parent ast.Node, st Style, link *Target, out *[]Run) error {
	for ch := parent.FirstChild(); ch != nil; ch = ch.NextSibling() {
		switch ch := ch.(type) {
		case *ast.Text:
			*out = append(*out, Run{Kind: TextRun, Text: resolveText(ch.Value(c.src)), Style: st, Link: link})
			if ch.SoftLineBreak() {
				*out = append(*out, Run{Kind: TextRun, Text: " ", Style: st, Link: link})
			}
			if ch.HardLineBreak() {
				*out = append(*out, Run{Kind: BreakRun})
			}
		case *ast.String:
			*out = append(*out, Run{Kind: TextRun, Text: string(ch.Value), Style: st, Link: link})
		case *ast.CodeSpan:
			text, err := c.codeSpanText(ch)
			if err != nil {
				return err
			}
			*out = append(*out, Run{Kind: TextRun, Text: text, Style: st | Code, Link: link})
		case *ast.Emphasis:
			flag, err := emphasisFlag(ch.Level)
			if err != nil {
				return err
			}
			if err := c.collectRuns(ch, st|flag, link, out); err != nil {
				return err
			}
		case *extast.Strikethrough:
			if err := c.collectRuns(ch, st|Strike, link, out); err != nil {
				return err
			}
		case *extast.TaskCheckBox:
			// DOC-8: the checkbox sets the item's task state and produces no run.
		default:
			return unmappedKind(ch)
		}
	}
	return nil
}

// emphasisFlag maps a goldmark emphasis level to its style flag (DOC-3).
func emphasisFlag(level int) (Style, error) {
	switch level {
	case emphasisLevel:
		return Emphasis, nil
	case strongLevel:
		return Strong, nil
	default:
		return 0, fmt.Errorf("doc: emphasis level %d has no style", level)
	}
}

// codeSpanText returns the raw text of a code span. A newline inside it becomes a space (DOC-3).
func (c *converter) codeSpanText(n *ast.CodeSpan) (string, error) {
	var b strings.Builder
	for ch := n.FirstChild(); ch != nil; ch = ch.NextSibling() {
		t, ok := ch.(*ast.Text)
		if !ok {
			return "", unmappedKind(ch)
		}
		b.Write(t.Value(c.src))
	}
	return strings.ReplaceAll(b.String(), "\n", " "), nil
}

// resolveText resolves backslash escapes and entity and numeric references in text outside
// code (DOC-3), one escape or reference at a time, so that "\&amp;" stays "&amp;".
func resolveText(b []byte) string {
	if bytes.IndexByte(b, '\\') < 0 && bytes.IndexByte(b, '&') < 0 {
		return string(b)
	}
	var out []byte
	for i := 0; i < len(b); {
		switch {
		case b[i] == '\\' && i+1 < len(b) && util.IsPunct(b[i+1]):
			out = append(out, b[i+1])
			i += 2
		case b[i] == '&':
			if resolved, n := reference(b[i:]); n > 0 {
				out = append(out, resolved...)
				i += n
			} else {
				out = append(out, b[i])
				i++
			}
		default:
			out = append(out, b[i])
			i++
		}
	}
	return string(out)
}

// reference resolves the entity or numeric reference that begins b. It returns the resolved
// bytes and the number of bytes consumed, or 0 when b does not begin with a reference.
func reference(b []byte) ([]byte, int) {
	limit := min(len(b), maxReferenceBytes)
	end := bytes.IndexByte(b[:limit], ';')
	if end < 0 {
		return nil, 0
	}
	ref := b[:end+1]
	if bytes.IndexByte(ref[1:], '&') >= 0 {
		return nil, 0
	}
	var resolved []byte
	if len(ref) > 1 && ref[1] == '#' {
		resolved = util.ResolveNumericReferences(ref)
	} else {
		resolved = util.ResolveEntityNames(ref)
	}
	if bytes.Equal(resolved, ref) {
		return nil, 0
	}
	return resolved, len(ref)
}

// normaliseRuns applies the run rules of DOC-3: merge equal neighbours, collapse spaces, trim
// the paragraph's ends, and drop runs left empty.
func normaliseRuns(rs []Run) []Run {
	merged := mergeRuns(rs)
	for i := range merged {
		if merged[i].Kind == TextRun {
			merged[i].Text = collapse(merged[i].Text, merged[i].Style&Code != 0)
		}
	}
	return dropEmpty(trimEnd(trimStart(merged)))
}

// mergeRuns joins adjacent text runs with equal style and link. It returns a new slice.
func mergeRuns(rs []Run) []Run {
	var out []Run
	for _, r := range rs {
		if n := len(out); n > 0 && r.Kind == TextRun && out[n-1].Kind == TextRun &&
			out[n-1].Style == r.Style && sameLink(out[n-1].Link, r.Link) {
			out[n-1].Text += r.Text
			continue
		}
		out = append(out, r)
	}
	return out
}

// sameLink reports whether two run links are equal, both nil included.
func sameLink(a, b *Target) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// collapse applies the spacing rule of DOC-3. Outside code every run of spaces and tabs is one
// space. Inside code each tab is one space and spaces are kept.
func collapse(s string, code bool) string {
	if code {
		return strings.ReplaceAll(s, "\t", " ")
	}
	var b strings.Builder
	inSpace := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == ' ' || c == '\t' {
			if !inSpace {
				b.WriteByte(' ')
			}
			inSpace = true
			continue
		}
		inSpace = false
		b.WriteByte(c)
	}
	return b.String()
}

// trimStart removes the spaces at the start of the paragraph: from the first run while it is
// plain text, dropping each run the removal empties (DOC-3).
func trimStart(rs []Run) []Run {
	for len(rs) > 0 && rs[0].Kind == TextRun && rs[0].Style&Code == 0 {
		rs[0].Text = strings.TrimLeft(rs[0].Text, " ")
		if rs[0].Text != "" {
			break
		}
		rs = rs[1:]
	}
	return rs
}

// trimEnd removes the spaces at the end of the paragraph: from the last run while it is plain
// text, dropping each run the removal empties (DOC-3).
func trimEnd(rs []Run) []Run {
	for len(rs) > 0 && rs[len(rs)-1].Kind == TextRun && rs[len(rs)-1].Style&Code == 0 {
		last := &rs[len(rs)-1]
		last.Text = strings.TrimRight(last.Text, " ")
		if last.Text != "" {
			break
		}
		rs = rs[:len(rs)-1]
	}
	return rs
}

// dropEmpty removes text runs with no text. It returns nil when nothing is left.
func dropEmpty(rs []Run) []Run {
	var out []Run
	for _, r := range rs {
		if r.Kind == TextRun && r.Text == "" {
			continue
		}
		out = append(out, r)
	}
	return out
}
