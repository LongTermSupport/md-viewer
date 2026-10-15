# md-viewer architecture

How md-viewer is built to meet [SPEC.md](SPEC.md). Expectation numbers (CLI-1, DOC-3, …) refer
to that file. The order of work is in [MILESTONES.md](MILESTONES.md).

## Pinned dependencies

| Module                     | Version                                           | Why                                                                        |
| -------------------------- | ------------------------------------------------- | -------------------------------------------------------------------------- |
| Go toolchain               | 1.26.9                                            | `go 1.26.9` in `go.mod`; the same as go-qa-ci's `go.mod` and the ccy image |
| `gioui.org`                | v0.10.3                                           | window, input, layout, text shaping, painting                              |
| `github.com/yuin/goldmark` | v1.8.6                                            | CommonMark parser with the GFM extensions                                  |
| `golang.org/x/image`       | v0.26.0                                           | the Go font files (`font/gofont/...`); the version Gio v0.10.3 requires    |
| go-qa-ci                   | commit `7fc4547e92f2fb0cf67c0c91f1f2068774be5d5a` | QA; no tag is published yet, so the pin is a commit (`qa/go-qa-ci.ref`)    |

No other direct dependency is added without the owner's agreement: no `gioui.org/x`, no test
assertion library (tests use the standard library), no CLI flag library.

The Gio and goldmark calls named below were checked against the source of these exact
versions.

## Packages

```text
cmd/md-viewer/      thin main: start the clock, call app.Prepare, start the window, exit
internal/doc/       file loading, goldmark AST -> typed model, link resolution
internal/render/    model -> Gio layout: palettes, styles, fonts, text flow, blocks, images
internal/app/       arguments, version, timing, the viewer (input, scroll, zoom, links)
internal/window/    the one package that imports gioui.org/app: the native window loop
testdata/           fixtures shared by the packages' tests and benchmarks
```

Dependencies point one way: `cmd -> app, window`; `app -> window, render, doc`;
`render -> doc`; `window` imports only Gio; `doc` imports neither. Nothing below `cmd` calls
`os.Exit` (go-qa's `gqaci_exit_outside_main` enforces it).

Only `internal/window` and `cmd/md-viewer` need a display at run time. Everything else is
tested headless: Gio's `layout`, `op`, `text`, `widget` and `io/input` packages run without a
window or GPU (checked: they build and run with no display).

### cmd/md-viewer

`main.go`, the whole of it in outline:

```go
func main() {
	start := time.Now()
	prep, code := app.Prepare(os.Args[1:], os.Stdout, os.Stderr, app.SystemClock{})
	if !prep.OpenWindow {
		os.Exit(int(code))
	}
	go func() {
		if err := window.Run(prep.Window(start)); err != nil {
			fmt.Fprintf(os.Stderr, "md-viewer: cannot open a window: %v\n", err) // CLI-9
			os.Exit(int(app.ExitFailure))
		}
		os.Exit(int(app.ExitOK))
	}()
	window.Main()
}
```

`window.Main` calls `gioui.org/app.Main`, which must run on the main goroutine and never
returns; the goroutine ends the process. The function literal is inside `main`, which
`gqaci_exit_outside_main` allows.

### internal/doc

The model. Every type is a plain value; there are no goldmark types in it.

```go
type Document struct {
	Name        string  // base name of the file (WIN-1)
	Dir         string  // absolute directory of the file (DOC-5)
	Blocks      []Block
	DroppedHTML int     // HTML blocks and inline HTML fragments not shown (DOC-9)
}

// Block is one of: Heading, Paragraph, CodeBlock, Quote, List, Table, Rule, Image.
// The interface is sealed by an unexported method; type switches over it must be
// exhaustive (each switch has a default case that returns an error naming the type).
type Block interface{ block() }

type Heading   struct{ Level int; Runs []Run }          // Level 1..6
type Paragraph struct{ Runs []Run }
type CodeBlock struct{ Lang, Text string }
type Quote     struct{ Blocks []Block }
type Rule      struct{}
type List      struct{ Ordered, Tight bool; Start int; Items []Item }
type Item      struct{ Task Task; Blocks []Block }
type Table     struct{ Align []Align; Header []Cell; Rows [][]Cell }
type Cell      struct{ Runs []Run }
type Image     struct{ Alt, Dest string; Target Target; Link *Target }

type Task  uint8 // NotTask, Open, Done
type Align uint8 // AlignNone, AlignLeft, AlignCentre, AlignRight

type RunKind uint8 // TextRun, BreakRun
type Style   uint8 // bit set: Emphasis, Strong, Strike, Code
type Run struct {
	Kind  RunKind
	Text  string  // empty for BreakRun
	Style Style
	Link  *Target // nil when the run is not a link
}

type TargetKind uint8 // External, LocalFile, Fragment, Refused
type Target struct {
	Kind  TargetKind
	Value string // URL, cleaned absolute path, or the destination as written
	Dest  string // the destination as written, for messages
}

func Parse(src []byte, dir string) (*Document, error) // DOC-1..12, DOC-14
func ResolveLink(dir, dest string) Target            // DOC-5; dest as written: escapes and references are resolved first
func Load(path string, now func() time.Time) (*Document, LoadStats, error) // DOC-13; sets Name and Dir
type LoadStats struct{ Read, Parse time.Duration }   // for --time (SPEED-2)

// Load errors, each a distinct type so the CLI chooses the message (CLI-8)
// without matching strings:
type ReadError     struct{ Path string; Err error } // Err is the *fs.PathError's Err, or "not a regular file"
type DirectoryError struct{ Path string }
type TooLargeError struct{ Path string; Size int64 }
type BinaryError   struct{ Path string }
const MaxFileSize = 64 << 20

// The error types are returned as pointers (*ReadError and so on). Their Error texts are for
// developers; the CLI-8 wording is written by internal/app (P11).
```

`Load` takes the clock as a parameter, so tests
never read the wall clock (go-qa's `gqaci_test_clock`) and `--time` can report read and parse
separately.

Conversion rules:

- goldmark is built once per `Parse` call with
  `goldmark.New(goldmark.WithExtensions(extension.GFM))`, and the AST comes from
  `md.Parser().Parse(text.NewReader(src))`.
- Text is taken with `(*ast.Text).Value(src)`, `(*ast.String).Value`, segment `Value(src)` and
  `Lines()`. The deprecated `Node.Text(src)` is never used (staticcheck SA1019 fails it).
  Text runs outside code resolve backslash escapes and references in one pass, with goldmark's
  `util.IsPunct`, `ResolveNumericReferences` and `ResolveEntityNames` (DOC-3), so `\&amp;`
  stays `&amp;`; `CodeSpan` text is raw.
- Node kinds handled: `ast.Document`, `Heading`, `Paragraph`, `TextBlock`, `CodeBlock`,
  `FencedCodeBlock`, `Blockquote`, `List`, `ListItem`, `ThematicBreak`, `HTMLBlock`, `Text`,
  `String`, `CodeSpan`, `Emphasis` (`Level` 1 is Emphasis, 2 is Strong), `Link`, `AutoLink`
  (`URL(src)`, `Label(src)`, `AutoLinkType`), `Image`, `RawHTML`, `LinkReferenceDefinition`
  (a node goldmark inserts; it produces no block, DOC-2); from `github.com/yuin/goldmark/extension/ast`:
  `Table`, `TableHeader`, `TableRow`, `TableCell`, `Strikethrough`, `TaskCheckBox`
  (`IsChecked`), with alignment from `Table.Alignments` (`AlignLeft`, `AlignRight`,
  `AlignCenter`, `AlignNone`).
- `ast.String` is created only by the typographer extension, which is not enabled, so no GFM
  input reaches it (P19). The converter keeps its case; the kind-coverage test does not require it.

### internal/render

Turns a `*doc.Document` into Gio layout calls. It has two layers, so most of it is tested as
plain values:

1. **Styles** (`style.go`, `palette.go`): pure functions that give each block and run its
   numbers from SPEC section 3. No Gio state.
2. **Layout** (`flow.go`, `blocks.go`, `list.go`, `table.go`, `image.go`, `view.go`): Gio
   layout functions that use the styles, shape text with a `*text.Shaper` and record what they
   drew into a `Trace`.

The two open owner decisions (MILESTONES "Decisions for the owner", 01 and 02) are **awaiting the
owner**. Both are built to the spec default, behind these seams:

- 01 fonts and emoji: the face list in `fonts.go` and the options given to `NewShaper`. An emoji
  or system font is one more source, or dropping `NoSystemFonts`.
- 02 dark mode: only `--dark` (STYLE-PAL-2). The choice is made in one place, `app.Prepare`
  (M10), from `Options.Dark`.

```go
type Palette struct {
	Fg, Canvas, FgMuted, Accent, Border, BorderMuted, BgMuted, CodeBg, OnAccent color.NRGBA
}
var Light, Dark Palette // STYLE-PAL-1

// Faces holds the six parsed Go font faces (STYLE-FONT-1).
type Faces struct{ Collection []font.FontFace }
func LoadFaces() (Faces, error)                 // parses goregular, goitalic, gobold,
                                                 // gobolditalic, gomono, gomonobold with
                                                 // gioui.org/font/opentype.ParseCollection
func NewShaper(f Faces) *text.Shaper            // text.NewShaper(text.NoSystemFonts(),
                                                 //   text.WithCollection(f.Collection))

// TextStyle is what a run is drawn with.
type TextStyle struct {
	Font       font.Font   // Typeface "Go" or "Go Mono", Style, Weight; Bold is font.SemiBold (F1)
	Size       unit.Sp
	LineHeight float32     // multiple of Size: 1.5, 1.25 or 1.45
	Colour     color.NRGBA
}

// HeadingRule is the rule under a heading. The zero value is no padding and no rule.
type HeadingRule struct {
	PaddingBelow, Thickness unit.Dp
	Colour                  color.NRGBA
}

// BodyStyle is the body text's style (STYLE-TEXT-1). A block's line height is its base style's
// Size × LineHeight (F6).
func BodyStyle(p Palette) TextStyle

// HeadingStyle is the style of heading level 1 to 6; any other level is an error (F3). The sizes
// are STYLE-TEXT-2's literal numbers in a per-level table, not em products (F4).
func HeadingStyle(level int, p Palette) (TextStyle, HeadingRule, error) // STYLE-TEXT-2
// RunStyle keeps base.LineHeight for every run, so a code run never changes the line (F6).
func RunStyle(base TextStyle, s doc.Style, link bool, p Palette, inHeading bool) TextStyle
// Margins: nested affects the List row only (F7); a block type with no row is an error (F3).
func Margins(b doc.Block, nested bool) (top, bottom unit.Dp, err error) // DOC-LAYOUT-4
func Gap(prevBottom, nextTop unit.Dp) unit.Dp                           // DOC-LAYOUT-3
func Padding(windowWidth unit.Dp) unit.Dp                               // DOC-LAYOUT-1
// OrderedMarker: n below 1 or a roman numeral above 3999 prints decimal (F8).
func OrderedMarker(n, depth int) string // STYLE-LIST-3

// Bullet is the shape of an unordered marker. Its String is the Mark text (disc, circle, square).
type Bullet uint8

const (
	BulletDisc Bullet = iota
	BulletCircle
	BulletSquare
)

func BulletShape(depth int) Bullet // STYLE-LIST-2, M6 (F2); depth below 1 is disc
// ColumnWidths: floor(N × W ÷ ΣN) with a minimum of minCol; "fits" is ΣN <= W. table.go passes
// W less the closing border; the table clips the overflow (F9).
func ColumnWidths(natural []int, width int, minCol int) []int // STYLE-TABLE-5

// Trace records what layout drew. Its origin is the top-left corner of what the trace was made
// for: the flow's in a flow trace, the list element's (gap and padding included) in a View
// trace. Nested layout translates by its origin (push and pop), so a sub-layout records in the
// coordinates of the trace it was given. Tests read it to assert structure.
type Trace struct{ Marks []Mark } // shiftFrom(i, d) moves Marks[i:] by d; count() is len(Marks) (M7)
type Mark struct {
	Kind   MarkKind        // MarkGlyphs, MarkCodeBg, MarkCodeBlockBg, MarkHeadingRule, MarkQuoteBar, MarkRuleBar,
	                       // MarkBullet, MarkNumber, MarkTaskBox, MarkTableLine, MarkRowFill, MarkImageBox,
	                       // MarkImagePixels, MarkPlaceholder, MarkStrike, MarkLinkArea. The Mark prefix avoids the
	                       // clash with the type names HeadingRule and Bullet (F2, from M4)
	Rect   image.Rectangle
	Colour color.NRGBA
	Text   string          // Glyphs and Number: the text drawn in Rect; Bullet: the shape; TaskBox: open, done, border or check
	Link   *doc.Target     // for LinkArea
}

// View lays out one document. It owns the per-link click state and the image cache,
// so it lives as long as the window.
type View struct{ /* unexported */ }
func NewView(d *doc.Document, sh *text.Shaper, p Palette, img *Images) *View // img: the document's images (M8)
func (v *View) Len() int                                             // number of top-level blocks
func (v *View) Layout(gtx layout.Context, i int, tr *Trace) layout.Dimensions // a layout.ListElement
func (v *View) Err() error                                           // the first layout error (M5)
// Layout draws block i, including the gap above it (and the content padding for the first
// and last block), at gtx.Constraints.Max.X wide. tr may be nil, and its origin is the list
// element's top-left. Padding is chosen from that width (Metric.PxToDp of Max.X), so M9 decides
// what width a scrollbar leaves. A failing element returns Dimensions{} and records the error
// in Err; the first error is kept and never cleared. The View paints no background: the window
// fills Canvas (STYLE-PAL-3, tested in M10).
func (v *View) Clicked(gtx layout.Context) (doc.Target, bool)        // LINK-1, polled once per frame

// Images probes headers before the window and decodes pixels lazily (SPEED-5).
type Images struct{ /* unexported */ }
func NewImages(d *doc.Document, warn io.Writer) (*Images, error) // reads headers now, warns per STYLE-IMG-3; an error is a failed write or an unknown block type
func (im *Images) Bind(invalidate func())                          // the window's Invalidate, set once it exists
```

**Text flow** (`flow.go`). Paragraph, heading and cell runs are laid out by md-viewer's own
inline flow, modelled on the algorithm of `gioui.org/x/styledtext` (Unlicense OR MIT; cite it
in the file's doc comment) but written here because md-viewer must also paint code
backgrounds, strike lines and link areas, and record them. For each run it calls
`(*text.Shaper).LayoutString` with `text.Parameters{Font, PxPerEm, MaxWidth, MaxLines: 1,
Truncator, WrapPolicy, Locale}`, iterates glyphs with `NextGlyph`, and paints each fragment's
glyph outlines with `Shaper.Shape(glyphs)` → `clip.Outline{Path: …}.Op()` under
`paint.ColorOp` and `paint.PaintOp`. Each run is shaped alone, so the flow places baselines
itself.

Line breaking rules:

- A run fills the current line. A run that does not fit is split at the last word break that
  fits (`text.WrapWords`, WRAP-1), and the rest goes to the next line.
- A word wider than a whole line breaks by grapheme (`text.WrapHeuristically`). That retry is
  made only at a line start. `WrapHeuristically` is not tried first, because on a line that
  already holds `ab` it would fill the line with graphemes of the long word; WRAP-1 keeps
  `ab ` on the first line.
- A zero-rune result on a non-empty line means "next line", and the run is tried again there.
- If not even one grapheme fits a line, the first rune is placed alone past the line's end, so
  that the loop always advances.
- `BreakRun` ends the line; two in a row give an empty line. No runs give one empty line of
  height L.
- A non-code run that starts a line drops its leading spaces.
- Each placed fragment records a `Glyphs` mark, and `CodeBg`, `Strike` and `LinkArea` marks as
  its style asks. Link fragments also push a `clip.Rect`, add `pointer.CursorPointer` and the
  link's `gesture.Click`.

Flow rules:

- **Line box.** Line height is L = round(base.Size px × base.LineHeight). The first baseline is
  line top + round((L − (A + D)) ÷ 2 + A), with A and D the base style's face ascent and
  descent; every run in the line shares that baseline. A block of n lines is n × L tall.
- **Rounding.** Each length is computed in float px and rounded to the nearest whole px once.
- **Fractional sizes.** PxPerEm rounds up to whole px, so 13.6 sp code is drawn with 14 px
  glyphs. The em products (padding, strike) use the style's size, 13.6.
- **Code padding.** Every fragment of a wrapped code run gets its own padding on both edges.
- **Strike.** Top edge at baseline − round(0.3 em), spanning the whole fragment including code
  padding.
- **Mark rectangles** (relative to the trace's origin, see Trace). `Glyphs`: from the fragment's text
  start to its end; y from baseline − ceil(ascent) to baseline + ceil(descent) of the
  fragment's glyphs; `Text` is the fragment's bytes with each invalid byte as U+FFFD (DOC-15).
  `CodeBg`: the `Glyphs` rectangle grown by the horizontal pad on each side and, outside a
  heading, the vertical pad above and below. It is painted directly, before the glyphs, with
  `paint.ColorOp` and `paint.PaintOp` under a rounded clip; no recorded macro is used.
  `Strike`: as above, height max(1 dp, round(size px ÷ 16)). `LinkArea`: the fragment's X span
  × the full line box; `Link` is the run's target.
- **Alignment.** `flowParams.align` is `alignStart` (the zero value), `alignCentre` or `alignEnd`
  (STYLE-TABLE-4); `newFlow` rejects any other value by name. A non-start line is recorded as a
  macro from its first fragment, and when it closes the macro is replayed under an offset of
  `floor((box − line width) ÷ 2)` for centre, or `box − line width` for end, and the marks it
  recorded move by the same offset (`Trace.shiftFrom`). The line width includes its trailing
  space. A line wider than the box gets no offset. Start alignment records nothing extra. Every
  error return of `layoutFlow` stops an open line macro.
- **Signature.** `layoutFlow(gtx layout.Context, fp flowParams, runs []doc.Run, tr *Trace)
  (layout.Dimensions, error)`, with `flowParams{shaper *text.Shaper; base TextStyle; palette
  Palette; inHeading bool; links *links; wrap wrapMode; align textAlign}`. `inHeading` is needed
  because `RunStyle` takes it. `tr` may be nil. An unknown `RunKind` is an error naming the kind.
- **Links.** `links` holds one `*gesture.Click` per `*doc.Target`, shared by every fragment of
  that link, in a fixed order. `clicked(gtx)` drains them once per frame and returns the first
  `KindClick`. M5's `View` owns the `links` and `View.Clicked` delegates to it.
- **Dimensions.** `Size.X` is the widest line, at least `Constraints.Min.X`; `Baseline` is
  `Size.Y` minus the first baseline. Every block with a text line sets `Baseline` the same way, to
  that line. A block with none has no first baseline; a list marker then takes its position from
  the rule of STYLE-LIST-2.
- **Known limitation.** A run boundary can break a word (as in styledtext).

**Lists** are laid out by `list.go` from the same primitives. Each item's content is laid out with
a `blockCtx` whose `depth` counts the lists around the block and whose `tight` is set for a direct
child of a tight list's item; `quoteCtx` clears `tight` and keeps `depth`. **Tables** are laid
out by `table.go` (below). **Images** are laid out by `image.go`: a box at the probed size (scaled
down to the available width, STYLE-IMG-2), then the pixels or a placeholder (STYLE-IMG-3). Pixels
are drawn with `paint.NewImageOp` and `op.Affine`, scaled into the probed box, so a decoded size
that differs from the probed one (the file changed) does not move the layout. `widget.Image` is
not used: it imports `golang.org/x/exp`, which the module does not name. `golang.org/x/image/webp` is
not used either: STYLE-IMG-1 names PNG and JPEG only. The decode worker is one
goroutine over a FIFO queue of images, started when the queue gets its first request and exiting
when it is empty (SPEED-5, SPEED-6). It calls the window's invalidate, bound by `Images.Bind`,
which is safe to call from any goroutine. An image decoded before `Bind` is drawn on the next frame
only, so `Bind` precedes the first layout. Layout never waits for it.

**Tables** (`table.go`). `checkTable` first: no columns, a count of alignments other than the
header's, a body row whose length is not the header's, or an alignment that is not one of the four
is an error naming it. The header row's cells use `headerStyle` (the body style, bold);
`cellAlign` maps each column's alignment (an exhaustive switch, whose default names the value).
For each column, N is the widest cell's natural width (its flow laid out unwrapped into a
scratch op list) plus 2 × 13 dp and 1 dp. `ColumnWidths(N, W − 1 dp, 64 dp)` gives the widths w.
Line i (vertical) is at x_i = Σ w before it, for i = 0..m, and T = x_m + 1 dp. Row r's top line
is y_r, with y_0 = 0 and y_{r+1} = y_r + 1 dp + 2 × 6 dp + h_r, where h_r is its tallest cell's
flow height; H = y_{n+1} + 1 dp for n body rows. A cell's flow is at
(x_i + 1 dp + 13 dp, y_r + 1 dp + 6 dp), wide max(w_i − 1 dp − 26 dp, 0). Each row's cells are
recorded, its fill painted, then the cells are replayed under
`clip.Rect(0, y_r, min(T, W), y_{r+1})`; the lines are painted last. Marks: `MarkTableLine` for
each whole line (vertical `(x_i, 0)-(x_i + 1, H)`, horizontal `(0, y_r)-(T, y_r + 1)`) in
`Border`, and `MarkRowFill` for each row, `(0, y_r + 1)-(T, y_{r+1})`, in `Canvas` for the header
and body rows 1, 3, … and `BgMuted` for body rows 2, 4, …. Only a `MarkTableLine` or
`MarkRowFill` is cut at x = min(T, W), and dropped when nothing is left (`cutWidth`);
`image.Rect` is not used for the cut, since it swaps corners that are out of order. `Size` is
`(min(T, W), H)`, and `Baseline` is H − (1 dp + 6 dp + the header's first baseline). Colours come
from the block's root palette, text from its text palette (STYLE-QUOTE-1).

### internal/app

```go
type ExitCode int
const (ExitOK ExitCode = 0; ExitFailure ExitCode = 1; ExitUsage ExitCode = 2)

type Options struct{ Path string; Dark, Time, Help, Version bool }
type UsageError struct{ Msg string } // the first line of CLI-7
func ParseArgs(args []string) (Options, error)     // CLI-1..3, CLI-7
const Usage = "..."                                // CLI-4, byte for byte
const UsageHint = "Run 'md-viewer --help' for usage." // CLI-7, the second line of a usage error
func VersionLine(info *debug.BuildInfo, ok bool) string // CLI-5

type Clock interface{ Now() time.Time }
type SystemClock struct{}                          // the only reader of time.Now outside main

// Prepared is everything the window needs; Prepare does SPEED-1's work and nothing else.
type Prepared struct {
	OpenWindow bool
	// unexported: options, document, view, palette, timings, stdout, stderr
}
func Prepare(args []string, stdout, stderr io.Writer, c Clock) (Prepared, ExitCode)
func (p Prepared) Window(start time.Time) window.Config

// Viewer is the per-window state: list position, zoom, accumulators, opener.
type Viewer struct{ /* unexported */ }
func NewViewer(v *render.View, p render.Palette, o Opener, stderr io.Writer) *Viewer
func (vw *Viewer) Frame(gtx layout.Context) (quit bool) // handle input, lay out, report quit

type Action uint8 // None, LineDown, LineUp, PageDown, PageUp, Top, Bottom,
                  // ZoomIn, ZoomOut, ZoomReset, Quit
func ActionFor(e key.Event) Action                // SCROLL-2, ZOOM-2, WIN-4 as a pure table

type Zoom struct{ /* level index */ }
func (z Zoom) Percent() int
func (z Zoom) In() Zoom; func (z Zoom) Out() Zoom; func (z Zoom) Reset() Zoom
func (z Zoom) Metric(m unit.Metric) unit.Metric   // STYLE-ZOOM-3

type Opener interface{ Open(ctx context.Context, target string) error }
func SystemOpener() Opener   // opener_linux.go: xdg-open; opener_other.go: an error per GOOS

type Timings struct{ Read, Parse, Fonts, FirstFrame time.Duration }
func (t Timings) Line() string // SPEED-2, byte for byte
```

**M2 notes on `Prepare`.** `Clock` and `SystemClock` live in `prepare.go`; `timing.go` holds
`Timings` from M10. `Prepare` handles the usage errors (stderr, `ExitUsage`) and `--help` and
`--version` (stdout, `ExitOK`), so `main` only calls it (P16). A file that loads gives
`OpenWindow` true and `ExitOK` (P14). A file error prints its CLI-8 line and gives `ExitFailure`;
any other load error prints `md-viewer: ` + the error (P10, P11). If a write to stdout or stderr
fails, `Prepare` returns `ExitFailure`, and when stdout failed it also writes `md-viewer: ` + the
error to stderr, best effort (P12). Until M10, a file that loads makes `main` print
`md-viewer: not implemented yet` to stderr and exit 1, after any DOC-9 line; M10 removes that
line (P13).

**Input routing in `Viewer.Frame`.** Key events are read with `gtx.Event(key.Filter{Name: …, Required: …, Optional: …})` for each key in the action table (a filter with a nil `Focus`
matches whatever has focus). Wheel events are taken by a full-window input area added *after*
the list, so it is on top: inside a `pointer.PassOp` (so presses still reach the scrollbar and
links below it), a `clip.Rect` of the window, and `event.Op(gtx.Ops, tag)`; it reads
`pointer.Filter{Target: tag, Kinds: pointer.Scroll, ScrollY: pointer.ScrollRange{Min: -1<<30, Max: 1<<30}}`. Because it is on top and accepts any distance, every wheel event goes to it
(Gio splits scroll along the hit chain from the top), and the viewer applies it: without Ctrl to
`layout.List.Position.Offset` (SCROLL-3), with Ctrl to the zoom accumulator (ZOOM-4). Keys
change `Position` directly: lines and pages add to `Offset`; Home sets `Position{}`; End sets
`Position{First: v.Len()}`, which `layout.List` clamps so the last block ends at the bottom.

The list is a `widget.List{List: layout.List{Axis: layout.Vertical}}` drawn with
`material.List(th, &list).Layout(gtx, view.Len(), view.Layout)`, where `th` is a
`material.Theme` whose `Palette.Fg` is the palette's `Fg` (the scrollbar indicator colour).

### internal/window

The only importer of `gioui.org/app`. It has no logic worth a unit test; it is proven by the
smoke run (CI and the container, under Xvfb) and by the owner on the desktop.

```go
type Config struct {
	Title    string                         // WIN-1
	Canvas   color.NRGBA                    // painted under every frame
	Frame    func(gtx layout.Context) bool  // the viewer; true means quit
	Bind     func(invalidate func())        // called once with the window's Invalidate
	OnFirst  func()                         // called once, after the first Frame call returns
	ExitAfterFirst bool                     // --time
}
func Run(c Config) error // returns when the window is gone: nil when closed, else Gio's error
func Main()              // gioui.org/app.Main
```

`Run` creates `new(app.Window)`, sets `app.Title`, `app.Size(980, 900)`,
`app.MinSize(320, 240)`, calls `Bind(w.Invalidate)`, and loops on `w.Event()`:

- `app.FrameEvent`: `gtx := app.NewContext(&ops, e)`, `paint.Fill(gtx.Ops, Canvas)`,
  `quit := c.Frame(gtx)`, `e.Frame(gtx.Ops)`; after the first frame call `OnFirst`; if `quit`
  or `ExitAfterFirst`, call `w.Perform(system.ActionClose)`;
- `app.DestroyEvent`: return `e.Err` (nil when the window was closed normally).

`internal/window` cannot import `internal/app` (that would be a cycle), so it reports with an
error and `main` turns it into CLI-9's message and exit code.

## Headless render tests

Render tests run with no window. The pattern:

```go
var ops op.Ops
var r input.Router
gtx := layout.Context{
	Ops:         &ops,
	Source:      r.Source(),
	Metric:      unit.Metric{PxPerDp: 1, PxPerSp: 1},
	Constraints: layout.Constraints{Max: image.Pt(800, 1 << 20)},
}
var tr render.Trace
dims := view.Layout(gtx, 0, &tr)
```

Tests assert **structure, not pixels**: the dimensions returned, and the `Trace` marks — their
kinds, rectangles, colours and texts. Examples: a heading level 1 has a `HeadingRule` mark 1 px
tall at its bottom in `BorderMuted`; a paragraph laid out at 200 px wide is taller than at
800 px and every `Glyphs` mark's right edge is ≤ 200; a code span has a `CodeBg` mark
containing its `Glyphs` mark. No test compares rendered images.

Input is tested through the same `input.Router`: lay out a frame, call `r.Frame(&ops)`, queue
events with `r.Queue(key.Event{…})` or `r.Queue(pointer.Event{…})`, lay out again and assert
the viewer's state (list position, zoom percent, the fake opener's calls, quit).

Text measurements depend on the Go fonts, which are embedded, so they are the same on every
machine; tests may assert exact pixel numbers where the font determines them, but prefer
relations (taller, narrower, inside) so a Gio upgrade breaks fewer tests.

## Fixtures in testdata/

| File               | What                                                                                                                                                                                                |
| ------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `elements.md`      | every supported element once, including a linked image and a reference-style link (committed with the plan); the smoke run's input                                                                                                                       |
| `large.md`         | `elements.md` repeated 100 times, separated by a blank line; for benchmarks and `--time`; `TestLargeFixtureIsCurrent` fails if it is not exactly that, and rewrites it when `MDV_UPDATE_FIXTURES=1` |
| `img/red-4x3.png`  | a 4 × 3 PNG, every pixel `#ff0000`                                                                                                                                                                  |
| `img/blue-8x8.jpg` | an 8 × 8 JPEG, quality 90, each channel of every pixel within 4 of `00 00 ff`                                                                                                                      |
| `img/notes.gif`    | a 1 × 1 GIF, for the unsupported-format placeholder (a GIF is not PNG or JPEG)                                                                                                                      |
| `img/broken.png`   | the bytes `not a png`, for the unsupported-format placeholder (the header sniff fails: `image.ErrFormat`)                                                                                           |

The images are written by `TestImageFixtures` in `internal/render` when `MDV_UPDATE_FIXTURES=1`
and otherwise checked to decode to exactly those sizes and colours. The decoder-error cases (a
header cut short, a valid header with pixel data cut short) and the size-cap cases are generated
by the tests in `t.TempDir()`, not kept as fixtures. Unit tests of single elements use inline
Markdown strings, not files.

## Benchmarks

In `internal/doc`: `BenchmarkParseLarge` (Parse of `large.md`'s bytes, read once outside the
loop). In `internal/render`: `BenchmarkLoadFonts` (`LoadFaces` plus `NewShaper`) and `BenchmarkFirstViewportLarge` (a new `View` of
`large.md`, then the blocks that fill a 980 × 900 dp viewport at zoom 100). They report
`b.ReportAllocs()`. Run with `go test -run '^$' -bench . -benchtime 20x ./internal/...`.

## QA

The gate is the plain Go toolchain: `gofmt -l .` printing nothing, `go build ./...`,
`go vet ./...` and `go test -race -count=1 ./...`. go-qa-ci is an add-on, never a requirement. CI's `go-qa` job
runs `go install` of go-qa at the commit in `qa/go-qa-ci.ref`, then `go-qa run --ci`; go-qa
installs and runs its own checksum-verified tools, pinned in go-qa-ci's `tools.lock`, so
neither this repository nor the image repeats those pins. `qa/go-qa.toml` holds only
two comment lines and no setting; go-qa's defaults (coverage 80%, mutants off) apply.

While go-qa-ci is private, the install step fails and the `go-qa` job is red on purpose.

## Cross-platform seams

md-viewer is Linux first. These are the places another platform plugs in:

| Seam               | Where                                   | Linux now                         | Elsewhere                                                        |
| ------------------ | --------------------------------------- | --------------------------------- | ---------------------------------------------------------------- |
| window and input   | `internal/window`                       | Gio, Wayland or X11               | Gio supports macOS and Windows unchanged                         |
| link opener        | `internal/app/opener_*.go` (build tags) | `xdg-open`                        | `open` (macOS), `rundll32 url.dll,FileProtocolHandler` (Windows) |
| colour scheme      | `--dark` only                           | flag                              | a platform query could set the default                           |
| fonts              | `render.LoadFaces`                      | embedded Go fonts                 | the same; nothing platform-specific                              |
| paths              | `doc.ResolveLink`, `doc.Load`           | `path/filepath`                   | the same                                                         |
| build dependencies | README, CI, `.claude/ccy/Dockerfile`    | Wayland, X11, EGL, Vulkan headers | none on macOS and Windows (Gio uses system frameworks)           |
