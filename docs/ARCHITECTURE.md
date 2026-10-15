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
| go-qa-ci                   | commit `803b122ce5266d58c5fa296985a4e1fde206cd0f` | QA; no tag is published yet, so the pin is a commit (`qa/go-qa-ci.ref`)    |

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
func ResolveLink(dir, dest string) Target            // DOC-5
func Load(path string, now func() time.Time) (*Document, LoadStats, error) // DOC-13; sets Name and Dir
type LoadStats struct{ Read, Parse time.Duration }   // for --time (SPEED-2)

// Load errors, each a distinct type so the CLI chooses the message (CLI-8)
// without matching strings:
type ReadError     struct{ Path string; Err error } // Err is the *fs.PathError's Err
type DirectoryError struct{ Path string }
type TooLargeError struct{ Path string; Size int64 }
type BinaryError   struct{ Path string }
const MaxFileSize = 64 << 20
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
  Text runs outside code resolve backslash escapes and references one at a time with goldmark's
  `util.UnescapePunctuations`, `ResolveNumericReferences` and `ResolveEntityNames` (DOC-3), so
  `\&amp;` stays `&amp;`; `CodeSpan` text is raw.
- Node kinds handled: `ast.Document`, `Heading`, `Paragraph`, `TextBlock`, `CodeBlock`,
  `FencedCodeBlock`, `Blockquote`, `List`, `ListItem`, `ThematicBreak`, `HTMLBlock`, `Text`,
  `String`, `CodeSpan`, `Emphasis` (`Level` 1 is Emphasis, 2 is Strong), `Link`, `AutoLink`
  (`URL(src)`, `Label(src)`, `AutoLinkType`), `Image`, `RawHTML`, `LinkReferenceDefinition`
  (a node goldmark inserts; it produces no block, DOC-2); from `github.com/yuin/goldmark/extension/ast`:
  `Table`, `TableHeader`, `TableRow`, `TableCell`, `Strikethrough`, `TaskCheckBox`
  (`IsChecked`), with alignment from `Table.Alignments` (`AlignLeft`, `AlignRight`,
  `AlignCenter`, `AlignNone`).

### internal/render

Turns a `*doc.Document` into Gio layout calls. It has two layers, so most of it is tested as
plain values:

1. **Styles** (`style.go`, `palette.go`): pure functions that give each block and run its
   numbers from SPEC section 3. No Gio state.
2. **Layout** (`flow.go`, `blocks.go`, `list.go`, `table.go`, `image.go`, `view.go`): Gio
   layout functions that use the styles, shape text with a `*text.Shaper` and record what they
   drew into a `Trace`.

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
	Font       font.Font   // Typeface "Go" or "Go Mono", Style, Weight
	Size       unit.Sp
	LineHeight float32     // multiple of Size: 1.5, 1.25 or 1.45
	Colour     color.NRGBA
}

// BlockBox is a block's margins and decoration, in dp.
type BlockBox struct{ MarginTop, MarginBottom unit.Dp /* ... */ }

func HeadingStyle(level int, p Palette) (TextStyle, HeadingRule)   // STYLE-TEXT-2
func RunStyle(base TextStyle, s doc.Style, link bool, p Palette, inHeading bool) TextStyle
func Margins(b doc.Block, nested bool) (top, bottom unit.Dp)        // DOC-LAYOUT-4
func Gap(prevBottom, nextTop unit.Dp) unit.Dp                        // DOC-LAYOUT-3
func Padding(windowWidth unit.Dp) unit.Dp                            // DOC-LAYOUT-1
func OrderedMarker(n, depth int) string                              // STYLE-LIST-3
func BulletShape(depth int) Bullet                                   // STYLE-LIST-2
func ColumnWidths(natural []int, width int, minCol int) []int        // STYLE-TABLE-5

// Trace records what layout drew, in pixels relative to the block's top-left corner.
// The view keeps one per visible block; tests read it to assert structure.
type Trace struct{ Marks []Mark }
type Mark struct {
	Kind   MarkKind        // Glyphs, CodeBg, CodeBlockBg, HeadingRule, QuoteBar, RuleBar,
	                       // Bullet, Number, TaskBox, TableLine, RowFill, ImageBox,
	                       // ImagePixels, Placeholder, Strike, LinkArea
	Rect   image.Rectangle
	Colour color.NRGBA
	Text   string          // for Glyphs and Number: the text drawn in Rect
	Link   *doc.Target     // for LinkArea
}

// View lays out one document. It owns the per-link click state and the image cache,
// so it lives as long as the window.
type View struct{ /* unexported */ }
func NewView(d *doc.Document, sh *text.Shaper, p Palette, img *Images) *View
func (v *View) Len() int                                             // number of top-level blocks
func (v *View) Layout(gtx layout.Context, i int, tr *Trace) layout.Dimensions
// Layout draws block i, including the gap above it (and the content padding for the first
// and last block), at gtx.Constraints.Max.X wide. tr may be nil.
func (v *View) Clicked(gtx layout.Context) (doc.Target, bool)        // LINK-1, polled once per frame

// Images probes headers before the window and decodes pixels lazily (SPEED-5).
type Images struct{ /* unexported */ }
func NewImages(d *doc.Document, warn io.Writer) *Images // reads headers now, warns per STYLE-IMG-3
func (im *Images) Bind(invalidate func())               // the window's Invalidate, set once it exists
```

**Text flow** (`flow.go`). Paragraph, heading and cell runs are laid out by md-viewer's own
inline flow, modelled on the algorithm of `gioui.org/x/styledtext` (Unlicense OR MIT; cite it
in the file's doc comment) but written here because md-viewer must also paint code
backgrounds, strike lines and link areas, and record them. For each run it calls
`(*text.Shaper).LayoutString` with `text.Parameters{Font, PxPerEm, MaxWidth, WrapPolicy, LineHeightScale, MaxLines: 1 per line attempt}`, iterates glyphs with `NextGlyph`, and paints
each line's glyph outlines with `Shaper.Shape(glyphs)` → `clip.Outline{Path: …}.Op()` and
`paint.FillShape`. Line breaking: fill the line run by run; a run that does not fit is split at
the last word break that fits (`text.WrapWords`); a word wider than a whole line breaks by
grapheme. `BreakRun` ends the line. Each placed fragment records a `Glyphs` mark, and
`CodeBg`, `Strike` and `LinkArea` marks as its style asks. Link fragments also push a
`clip.Rect`, add `pointer.CursorPointer` and the link's `gesture.Click`.

**Lists, tables, images** are laid out by `list.go`, `table.go` and `image.go` from the same
primitives. Images use `paint.NewImageOp` with `op.Affine` scaling, or `widget.Image`
with `Fit: widget.ScaleDown` and `Scale: 1` (one image pixel per dp before zoom).

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
| `elements.md`      | every supported element once (committed with the plan); the smoke run's input                                                                                                                       |
| `large.md`         | `elements.md` repeated 100 times, separated by a blank line; for benchmarks and `--time`; `TestLargeFixtureIsCurrent` fails if it is not exactly that, and rewrites it when `MDV_UPDATE_FIXTURES=1` |
| `img/red-4x3.png`  | a 4 × 3 PNG, every pixel `#ff0000`                                                                                                                                                                  |
| `img/blue-8x8.jpg` | an 8 × 8 JPEG, every pixel near `#0000ff`, quality 90                                                                                                                                               |
| `img/notes.gif`    | a 1 × 1 GIF, for the unsupported-format placeholder                                                                                                                                                 |
| `img/broken.png`   | the bytes `not a png`, for the decode-error placeholder                                                                                                                                             |

The images are written by `TestImageFixtures` in `internal/render` when `MDV_UPDATE_FIXTURES=1`
and otherwise checked to decode to exactly those sizes and colours. Unit tests of single
elements use inline Markdown strings, not files.

## Benchmarks

In `internal/doc`: `BenchmarkParseLarge` (Parse of `large.md`'s bytes, read once outside the
loop). In `internal/render`: `BenchmarkLoadFonts` (`LoadFaces` plus `NewShaper`) and `BenchmarkFirstViewportLarge` (a new `View` of
`large.md`, then the blocks that fill a 980 × 900 dp viewport at zoom 100). They report
`b.ReportAllocs()`. Run with `go test -run '^$' -bench . -benchtime 20x ./internal/...`.

## QA

The gate is the plain Go toolchain: `gofmt -l .` printing nothing, `go vet ./...` and
`go test -race -count=1 ./...`. go-qa-ci is an add-on, never a requirement. CI's `go-qa` job
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
