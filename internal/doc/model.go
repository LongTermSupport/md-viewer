// Package doc loads a Markdown file and converts it into a typed model. The model holds no
// goldmark types, so the render layer depends on md-viewer's own types only (SPEC section 2,
// ARCHITECTURE "internal/doc").
package doc

// Document is a parsed file. Name and Dir are set by Load (DOC-13); Parse sets Dir.
type Document struct {
	Name        string // base name of the file (WIN-1)
	Dir         string // absolute directory of the file (DOC-5)
	Blocks      []Block
	DroppedHTML int // HTML blocks and inline HTML fragments not shown (DOC-9)
}

// Block is one of: Heading, Paragraph, CodeBlock, Quote, List, Table, Rule, Image. The
// interface is sealed by the unexported method; every type switch over it has a default case
// that returns an error naming the type.
type Block interface{ block() }

// Heading is an ATX or setext heading (DOC-6). Level is 1 to 6.
type Heading struct {
	Level int
	Runs  []Run
}

// Paragraph is a paragraph, or a tight list item's text (DOC-3, DOC-8).
type Paragraph struct {
	Runs []Run
}

// CodeBlock is a fenced or indented code block (DOC-7). Lang is the first word of the info
// string.
type CodeBlock struct {
	Lang string
	Text string
}

// Quote is a block quote (DOC-12).
type Quote struct {
	Blocks []Block
}

// Rule is a thematic break (DOC-12).
type Rule struct{}

// List is an ordered or unordered list (DOC-8). Start is the first number of an ordered list
// and 1 for an unordered one.
type List struct {
	Ordered, Tight bool
	Start          int
	Items          []Item
}

// Item is one list item: its task state and its child blocks (DOC-8).
type Item struct {
	Task   Task
	Blocks []Block
}

// Table is a GFM table (DOC-10).
type Table struct {
	Align  []Align
	Header []Cell
	Rows   [][]Cell
}

// Cell is one table cell.
type Cell struct {
	Runs []Run
}

// Image is an image laid out as a block of its own (DOC-11). Link is set when the image is a
// link's content.
type Image struct {
	Alt, Dest string
	Target    Target
	Link      *Target
}

func (Heading) block()   {}
func (Paragraph) block() {}
func (CodeBlock) block() {}
func (Quote) block()     {}
func (Rule) block()      {}
func (List) block()      {}
func (Table) block()     {}
func (Image) block()     {}

// Task is the state of a list item's checkbox (DOC-8). The zero value is NotTask.
type Task uint8

const (
	NotTask Task = iota // no checkbox
	Open                // [ ]
	Done                // [x] or [X]
)

// Align is a table column's alignment (DOC-10). The zero value is AlignNone.
type Align uint8

const (
	AlignNone   Align = iota // no marker: body cells left, header cells centred (STYLE-TABLE-4)
	AlignLeft                // :--
	AlignCentre              // :-:
	AlignRight               // --:
)

// RunKind says what a run is (DOC-3). The zero value is TextRun.
type RunKind uint8

const (
	TextRun  RunKind = iota // text, with a style and optional link
	BreakRun                // a hard line break; carries no text
)

// Style is a set of style flags (DOC-3). The zero value is no style. Nested markup combines
// flags.
type Style uint8

// Style flags (DOC-3). Each is a distinct bit.
const (
	Emphasis Style = 1 << iota
	Strong
	Strike
	Code
)

// Run is a string with a style and an optional link (DOC-3). A BreakRun has no text, style or
// link.
type Run struct {
	Kind  RunKind
	Text  string
	Style Style
	Link  *Target
}

// TargetKind classifies a link destination (DOC-5). The zero value is External.
type TargetKind uint8

const (
	External  TargetKind = iota // http, https, mailto: the destination unchanged
	LocalFile                   // a file on disk: the cleaned path
	Fragment                    // #fragment: not followed
	Refused                     // any other scheme, or unparseable
)

// Target is a resolved link destination (DOC-5).
type Target struct {
	Kind  TargetKind
	Value string // URL, cleaned absolute path, or the destination as written
	Dest  string // the destination as written, for messages
}
