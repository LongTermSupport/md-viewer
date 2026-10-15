# md-viewer specification

This is the contract md-viewer is built to. Every numbered expectation is testable, and each
gets a test that is seen to fail before the code that makes it pass is written
([MILESTONES.md](MILESTONES.md) says which milestone builds which expectation). The package
layout and public types are in [ARCHITECTURE.md](ARCHITECTURE.md).

Words used with a fixed meaning:

- **dp** and **sp** are Gio's device-independent units (`unit.Dp`, `unit.Sp`). A CSS `px` in
  GitHub's stylesheet is taken as 1 dp (lengths) or 1 sp (font sizes). At zoom 100% and a
  display scale of 1, 1 dp is 1 pixel.
- **em** is the font size of the element in question, as in CSS.
- **content width** is the window width minus the left and right content padding (DOC-LAYOUT-1).
- **stderr** and **stdout** are the process's standard error and standard output. Diagnostics go
  to stderr; stdout carries only the `--help`, `--version` and `--time` output.

## 1. Command line

**CLI-1.** The program is invoked as `md-viewer [FLAGS] FILE`. Exactly one FILE is required.

**CLI-2.** Flags are recognised anywhere among the arguments, before or after FILE. An
argument `--` ends flag parsing: every later argument is a FILE operand, even if it begins
with `-`. A lone `-` is a FILE operand (a file called `-`), not standard input.

**CLI-3.** The flags are exactly these. Each is matched by its whole spelling; there are no
abbreviations, no `=value` forms, and single-dash long forms (`-dark`) are unknown flags.

| Flag           | Effect                                                                       |
| -------------- | ---------------------------------------------------------------------------- |
| `--dark`       | use the dark palette (STYLE-PAL-2)                                           |
| `--time`       | print startup timings to stdout after the first frame, then exit 0 (SPEED-2) |
| `--help`, `-h` | print the usage text (CLI-4) to stdout and exit 0; FILE is not needed        |
| `--version`    | print the version line (CLI-5) to stdout and exit 0; FILE is not needed      |

If both `--help` and `--version` are given, `--help` wins. Repeating a flag is allowed and has
the same effect as giving it once. With `--help` or `--version`, FILE is optional: one FILE
operand is recorded and ignored, and other operands are not an error (CLI-7 still applies to
unknown flags).

**CLI-4.** The usage text is exactly:

```text
Usage: md-viewer [--dark] [--time] FILE
       md-viewer --help | --version

Shows one Markdown file, rendered, in one window.

  --dark     use the dark palette
  --time     print startup timings after the first frame, then exit
  --help     show this help
  --version  show the version

Scroll: wheel, arrows, Page Up/Down, Space, Home/End.
Zoom: Ctrl and + or -, Ctrl+0 to reset, Ctrl and the wheel.
Quit: q or Esc.
```

**CLI-5.** The version line is `md-viewer VERSION`, where VERSION is the main module version
from `runtime/debug.ReadBuildInfo`, printed as the Go toolchain stamped it: `v1.2.3` for a
`go install …@v1.2.3` build, and for a normal `go build` with Go 1.24 or later a VCS
pseudo-version such as `v0.0.0-20261015095027-53092c9efe88`, with `+dirty` appended for a
modified tree. When the main module version is `(devel)` and the build info carries a
`vcs.revision` setting, VERSION is `(devel) ` followed by the first 12 characters of the
revision, and ` dirty` is appended when `vcs.modified` is `true`; an empty `vcs.revision`
gives just `(devel)`. An empty main module version gives `unknown`, and so does a build with
no build info.

**CLI-6. Exit codes.**

| Code | Meaning                                                                        |
| ---- | ------------------------------------------------------------------------------ |
| 0    | success: the window was closed by the user, or `--help`, `--version`, `--time` |
| 1    | a runtime failure: the file cannot be read or is refused, no window opened, or a frame failed (below) |
| 2    | a usage error: missing or extra FILE, an unknown flag                          |

A frame failure is a layout error (`View.Err`), a failed write to stderr during a frame, or a
failed write of the `--time` line to stdout. The window closes, md-viewer prints `md-viewer: `
followed by the error to stderr, and exits 1. The `--time` line is written only after a frame
has succeeded, so a failed frame prints none. Once the window is closing, the frames the window
still delivers are drawn without calling the viewer, so a second error or link action cannot
happen.

**CLI-7. Usage errors** print one line to stderr, then the line
`Run 'md-viewer --help' for usage.`, and exit 2. The first line is exactly:

| Condition         | Message                                           |
| ----------------- | ------------------------------------------------- |
| no FILE           | `md-viewer: missing FILE argument`                |
| two or more FILEs | `md-viewer: expected one FILE, got N` (N counted) |
| unknown flag      | `md-viewer: unknown flag: ARG` (ARG as typed)     |

When several usage errors apply, the first unknown flag (in argument order) is reported;
otherwise the FILE count error. An unknown flag is reported even with `--help` or `--version`;
the FILE count is not checked with either of them (CLI-3).

**CLI-8. File errors** print one line to stderr and exit 1. PATH is the FILE argument exactly
as typed.

| Condition                                   | Message                                                           |
| ------------------------------------------- | ----------------------------------------------------------------- |
| the file cannot be opened or read           | `md-viewer: cannot read PATH: REASON`                             |
| PATH is a directory                         | `md-viewer: cannot read PATH: is a directory`                     |
| PATH is not a regular file                  | `md-viewer: cannot read PATH: not a regular file`                 |
| the file is larger than 64 MiB (67108864 B) | `md-viewer: PATH is too large: SIZE bytes, the limit is 67108864` |
| the file contains a NUL byte                | `md-viewer: PATH is not a text file: it contains a NUL byte`      |

REASON is the text of the operating system error underneath the `*fs.PathError` (for a missing
file, `no such file or directory`; for a permission failure, `permission denied`). The size is
checked with `Stat` before reading, so a huge file is refused without being read. A path that
`Stat` shows is neither a regular file nor a directory (a device, FIFO or socket) is refused
without being opened. The read is capped at the limit plus one byte, so a file that grows after
its `Stat` is refused too, with SIZE the bytes read.

**CLI-9.** If the window cannot be opened (no display, no GPU driver), or Gio reports an error
before the window closes, md-viewer prints `md-viewer: cannot open a window: REASON` to stderr
and exits 1, where REASON is the error Gio reports. Gio tries Wayland first, then X11, and
reports the Wayland error when neither works, even if X11 was the one meant.

**CLI-10.** The file extension is not checked: any text file is shown as Markdown.

**CLI-11.** md-viewer reads no configuration file, no environment variable of its own, and no
standard input.

## 2. Document model

The document model (`internal/doc`) is a typed tree produced from goldmark's AST. It holds no
goldmark types, so the render layer depends on md-viewer's own types only. Types are listed in
[ARCHITECTURE.md](ARCHITECTURE.md#internaldoc).

**DOC-1.** Parsing uses goldmark with the GFM extension set: tables, strikethrough, linkify and
task lists (`extension.GFM`). No other extension is enabled, so footnotes, definition lists,
emoji shortcodes, typographer quotes and heading attributes are not interpreted; their source
text appears as ordinary text, unless that text is also valid CommonMark: `[^1]: note` is a
link reference definition (DOC-2), so it produces no text and `[^1]` becomes a link.

**DOC-2.** Every block in the goldmark AST maps to exactly one model block, except as stated in
DOC-9 (HTML) and DOC-11 (images). A link reference definition (`[label]: destination`) produces
no block. A goldmark node kind with no mapping is a programming error: the conversion returns
an error naming the kind, and a test covers every kind that GFM can produce, so that error is
unreachable in practice.

**DOC-3. Inline text is flattened into runs.** A paragraph, heading, or table cell holds a list
of runs. A run is a string with a style set and an optional link:

- style flags: `Emphasis`, `Strong`, `Strike`, `Code`; nested markup combines flags
  (`***x***` is `Emphasis|Strong`);
- adjacent text with identical style and link is merged into one run;
- a soft line break becomes one space;
- a hard line break (two trailing spaces or a trailing backslash) becomes a run of kind
  `Break`, which carries no text, has style 0 and no link, even inside emphasis or a link;
- in runs without `Code`, backslash escapes and entity and numeric character references are
  resolved as goldmark's HTML writer resolves them (`\*` is `*`, `&amp;` is `&`, `\&amp;` is
  `&amp;`); `Code` runs are raw. Resolution happens before the spacing rules;
- in runs without `Code`, every sequence of spaces and tabs collapses to one space (as HTML
  does); in `Code` runs spaces are kept as written, and a tab becomes one space. Collapsing
  happens within a run after adjacent runs of equal style and link are merged, not across runs;
- leading and trailing spaces of a paragraph, heading or cell are removed: the spaces at the
  start of the first run and at the end of the last run, when that run is not `Code`. A run
  emptied by the removal is dropped, and the removal continues into the next run while that run
  is not `Code`.

**DOC-4. Links.** A link run's link is its destination, with backslash escapes and references
resolved, classified by DOC-5. `[text](dest)` gives the runs of `text` with that link; an
autolink `<https://…>` and a bare URL found by linkify give one run whose text is the URL as
written and whose link is the URL. A `www.` link found by linkify keeps its written text
(`www.e.org`) and its link is `http://www.e.org` (P1). An email autolink, `<a@b.c>`, and a bare
address found by linkify give the text as written and the link `mailto:` plus the address; that
`mailto:` target goes through DOC-5 like any other destination (P2).

**DOC-5. Link targets.** `doc.ResolveLink(docDir, dest)` classifies a destination. Backslash
escapes and entity and numeric references in `dest` are resolved first, as DOC-3 does (`x\_y.md`
is `x_y.md`), and `Target.Dest` holds the resolved text (P6). An empty destination (`[a]()`) is
the document's directory, as `LocalFile` (P8). A `file` URL with a host other than empty or
`localhost`, an opaque `file:` URL (`file:x.md`), and a destination with no scheme that has a
host (`//e.org/x`) are `Refused` (P7).

| Destination                                            | Target kind | Target value                                                                                                                                       |
| ------------------------------------------------------ | ----------- | -------------------------------------------------------------------------------------------------------------------------------------------------- |
| scheme `http`, `https` or `mailto` (case-insensitive)  | `External`  | the destination unchanged                                                                                                                          |
| scheme `file`                                          | `LocalFile` | the URL's path, cleaned                                                                                                                            |
| no scheme and no host, not starting with `#`           | `LocalFile` | `docDir` joined with the path part, cleaned; an absolute path is kept as is; the `#fragment` and `?query` are dropped; percent-escapes are decoded |
| starting with `#`                                      | `Fragment`  | the destination unchanged                                                                                                                          |
| any other scheme, or a destination that fails to parse | `Refused`   | the destination unchanged                                                                                                                          |

**DOC-6. Headings.** ATX and setext headings both map to `Heading` with level 1 to 6 and runs.

**DOC-7. Code blocks.** Fenced and indented code blocks both map to `CodeBlock`. Its text is
the lines exactly as goldmark gives them, with tabs expanded to the next multiple of 8 columns
(a column is one rune; an invalid byte counts as one; a newline starts a new count) and the
final newline removed. Before goldmark sees the source, `Parse` turns each `\r\n` and each lone
`\r` into `\n`, so no text carries a CR. The fence's info string is kept as `Lang` (first word only) but
changes nothing in rendering.

**DOC-8. Lists.** Ordered and unordered lists map to `List` with `Ordered`, `Start` (the first
number of an ordered list; 1 for unordered), `Tight` (goldmark's `IsTight`) and items. Each
item holds its child blocks and a task state: `NotTask`, `Open` (`[ ]`) or `Done` (`[x]` or
`[X]`). The checkbox is removed from the item's text. An empty item has no blocks; a
checkbox-only item (`- [ ]`) has one empty paragraph. In a tight list goldmark's `TextBlock`
maps to `Paragraph`, and the list's `Tight` flag decides the spacing (STYLE-LIST-4).

**DOC-9. HTML is dropped.** HTML blocks (including comments) and inline raw HTML produce no
model content. The text between inline tags is ordinary Markdown text and is kept
(`a <span>b</span> c` gives the text `a b c`). The model counts what it dropped in
`Document.DroppedHTML`, and md-viewer prints one line to stderr when that count is not zero:
`md-viewer: N HTML fragment(s) not shown: HTML is not rendered`. A fragment is one goldmark HTML
block (a block tag or a comment, however many lines it spans), or one inline raw-HTML tag or
comment: `<span>b</span>` is two fragments and a tag split over two lines is one (P22).

**DOC-10. Tables.** A GFM table maps to `Table` with one alignment per column (`Left`,
`Centre`, `Right`, `None`), a header row and body rows, each cell holding runs. Rows shorter
than the header are padded with empty cells; GFM already drops extra cells.

**DOC-11. Images.** An image is laid out as a block of its own. A paragraph, or a tight list
item's text, containing images is split at each image into: the paragraph of the runs before it,
an `Image` block, the paragraph of the runs after it, and so on. Each split paragraph is
normalised as a paragraph of its own (DOC-3), and is omitted when that leaves no runs (P5). An
`Image` holds its `Alt` text, its destination (escapes and references resolved, as in DOC-5),
its `Target` (DOC-5) and, when it is a link's content (`[![alt](img)](href)`), the link. `Alt` is
the image's inline content flattened to plain text: its DOC-3 runs with escapes and references
resolved, code raw, spaces collapsed and trimmed, and a break as one space. That is the text of
goldmark's HTML `alt` attribute (P4). An image inside a heading or a table cell is replaced by
its alt text as a text run, with the style and link around the image, so it merges with its
neighbours (P3).

**DOC-12. Block quotes, rules.** A block quote maps to `Quote` holding its child blocks; a
thematic break maps to `Rule`.

**DOC-13. Loading.** `doc.Load(path, now)` applies CLI-8's checks in that order (stat, directory,
regular file, size, read, NUL) and returns typed errors, as pointers, that the CLI turns into the messages of
CLI-8 (the CLI owns the wording). `Document.Dir` is the absolute directory of the file, and
`Document.Name` its base name. `Load` sets `Name` and passes the absolute directory to `Parse`,
which stores it as `Document.Dir`. An error from `filepath.Abs` or `Parse` is returned as a plain
error, and the CLI prints `md-viewer: ` + the error and exits 1 (P10). `now` is called exactly
three times on success: before the stat, after the NUL check, and after `Parse`. `LoadStats.Read` is the
difference of the first two readings (stat, read and NUL check) and `LoadStats.Parse` that of the
last two (P9).

**DOC-14.** Parsing is deterministic: the same bytes give a model that is `reflect.DeepEqual`
to the last one.

**DOC-15.** Invalid UTF-8 is not an error. goldmark passes the bytes through; the render layer
shows each invalid byte as U+FFFD.

## 3. Rendering in GitHub style

The look follows GitHub's published Markdown stylesheet, **github-markdown-css v5.9.0** by
Sindre Sorhus (MIT), the files `github-markdown-light.css` and `github-markdown-dark.css` at
<https://github.com/sindresorhus/github-markdown-css/tree/v5.9.0>. The CSS selector each value
comes from is cited beside it, so a later version can be compared rule by rule. Where md-viewer
departs from the stylesheet, the expectation says so and why.

### 3.1 Palettes

**STYLE-PAL-1.** The palette has these named colours. `#RRGGBBAA` has alpha last, as in CSS.

| Name          | Light       | Dark        | Source (light file)                                                                      |
| ------------- | ----------- | ----------- | ---------------------------------------------------------------------------------------- |
| `Fg`          | `#1f2328`   | `#f0f6fc`   | `.markdown-body { color }`                                                               |
| `Canvas`      | `#ffffff`   | `#0d1117`   | `.markdown-body { background-color }`                                                    |
| `FgMuted`     | `#59636e`   | `#9198a1`   | `h6 { color }`, `blockquote { color }`                                                   |
| `Accent`      | `#0969da`   | `#4493f8`   | `a { color }`                                                                            |
| `Border`      | `#d1d9e0`   | `#3d444d`   | `table th, table td { border }`, `hr { background-color }`, `blockquote { border-left }` |
| `BorderMuted` | `#d1d9e0b3` | `#3d444db3` | `h1, h2 { border-bottom }`                                                               |
| `BgMuted`     | `#f6f8fa`   | `#151b23`   | `pre { background-color }`, `tr:nth-child(2n)`                                           |
| `CodeBg`      | `#818b981f` | `#656c7633` | `code, tt { background-color }`                                                          |
| `OnAccent`    | `#ffffff`   | `#ffffff`   | md-viewer's own: the check mark on a done task box                                       |

**STYLE-PAL-2.** The light palette is used unless `--dark` is given. Gio v0.10.3 exposes no
system colour-scheme setting (its `app` package has no such event or query), so md-viewer does
not follow the desktop's light or dark preference. See "Decisions for the owner" in
[MILESTONES.md](MILESTONES.md).

**STYLE-PAL-3.** The whole window, outside and inside the content, is filled with `Canvas`.
Gio's window decorations, where Gio draws them, keep Gio's colours.

### 3.2 Fonts

**STYLE-FONT-1.** Text uses the Go fonts embedded in the binary (`golang.org/x/image/font/gofont`):
Go Regular, Go Italic, Go Bold and Go Bold Italic for text, and Go Mono and Go Mono Bold for
code. Exactly these six faces are loaded, and no system font is looked up
(`text.NoSystemFonts()`). GitHub's stack (`-apple-system, "Segoe UI", "Noto Sans", …`) needs
system fonts, which cost a font scan at start (SPEED-4); the Go fonts are the substitute.

**STYLE-FONT-2.** Characters the Go fonts lack (emoji, CJK, most symbols) are drawn as the
font's missing-glyph box. This is a known cost of STYLE-FONT-1.

**STYLE-FONT-3.** `Strong` uses the Bold face. GitHub asks for weight 600 (`b, strong`,
`h1`…`h6`); Go Bold declares weight 600, so it is used, requested as `font.SemiBold`. Headings
are Bold.

**STYLE-FONT-4.** `Emphasis` uses the Italic face; `Emphasis|Strong` uses Bold Italic. `Code`
uses Go Mono (Go Mono Bold when also `Strong`; Go Mono has no italic in this set, so `Code`
with `Emphasis` is upright).

### 3.3 Page layout and spacing

**DOC-LAYOUT-1. Content padding.** The content is inset from the window edges by 45 dp on all
four sides when the window is at least 768 dp wide (after zoom, STYLE-ZOOM-3), and 15 dp when
it is narrower. Source: github-markdown-css's README recommends
`.markdown-body { padding: 45px }` with `padding: 15px` under `max-width: 767px`. The width
tested is the list's width, the window less the 10 dp scrollbar gutter (SCROLL-4), read in
zoomed dp (STYLE-ZOOM-3). So at zoom 100 a window 778 dp wide is tested at 768 dp and gets 45 dp
of padding, and a window 777 dp wide is tested at 767 dp and gets 15 dp. The gutter is kept even
when the content fits and no bar is drawn.

**DOC-LAYOUT-2. No maximum width.** The content fills the window's width minus the padding.
GitHub caps the column at 980 px; md-viewer does not, because it wraps to the window
(WRAP-1).

**DOC-LAYOUT-3. Block spacing.** Each block has a top and a bottom margin. The space between
two consecutive blocks is the larger of the first block's bottom margin and the second block's
top margin (CSS margin collapsing). The first block of the document, of a quote and of a list
item has a top margin of 0; the last has a bottom margin of 0 (`.markdown-body>*:first-child`,
`>*:last-child`, `blockquote>:first-child`, `blockquote>:last-child`).

**DOC-LAYOUT-4. Margins.**

| Block                                   | Top   | Bottom | Source                                                                   |
| --------------------------------------- | ----- | ------ | ------------------------------------------------------------------------ |
| Paragraph                               | 0     | 16 dp  | `p, blockquote, ul, ol, dl, table, pre, details { margin-bottom: 1rem }` |
| Heading (all levels)                    | 24 dp | 16 dp  | `h1, …, h6 { margin-top: 1.5rem; margin-bottom: 1rem }`                  |
| Quote                                   | 0     | 16 dp  | as Paragraph                                                             |
| List (top level)                        | 0     | 16 dp  | as Paragraph                                                             |
| List inside another list (at any depth) | 0     | 0      | `ul ul, ul ol, ol ol, ol ul { margin-top: 0; margin-bottom: 0 }`         |
| Code block                              | 0     | 16 dp  | as Paragraph                                                             |
| Table                                   | 0     | 16 dp  | as Paragraph                                                             |
| Rule                                    | 24 dp | 24 dp  | `hr { margin: 1.5rem 0 }`                                                |
| Image                                   | 0     | 16 dp  | it sits in a paragraph, so a paragraph's margins                         |

`1rem` is 16 dp; it does not change with the element's own font size. Only the List row depends
on `nested`.

### 3.4 Text

**STYLE-TEXT-1. Body text** is 16 sp, colour `Fg`, line height 1.5 × font size
(`.markdown-body { font-size: 16px; line-height: 1.5 }`).

**STYLE-TEXT-2. Headings.**

| Level | Size    | Line height | Padding below | Rule below          | Colour    |
| ----- | ------- | ----------- | ------------- | ------------------- | --------- |
| 1     | 32 sp   | 1.25        | 9.6 dp (.3em) | 1 dp, `BorderMuted` | `Fg`      |
| 2     | 24 sp   | 1.25        | 7.2 dp (.3em) | 1 dp, `BorderMuted` | `Fg`      |
| 3     | 20 sp   | 1.25        | 0             | none                | `Fg`      |
| 4     | 16 sp   | 1.25        | 0             | none                | `Fg`      |
| 5     | 14 sp   | 1.25        | 0             | none                | `Fg`      |
| 6     | 13.6 sp | 1.25        | 0             | none                | `FgMuted` |

Sources: `h1 { font-size: 2em; padding-bottom: .3em; border-bottom: 1px solid }`,
`h2 { font-size: 1.5em; … }`, `h3 { 1.25em }`, `h4 { 1em }`, `h5 { .875em }`,
`h6 { .85em; color }`, `h1, …, h6 { line-height: 1.25 }`. The rule spans the full content
width, directly under the padding. An empty heading (no runs) is one empty line of its line
height, with its padding and rule.

**STYLE-TEXT-3. Links** are drawn in `Accent`, with no underline, keeping their other style
flags. The pointer turns into the hand cursor (`pointer.CursorPointer`) over a link
(`a { color; text-decoration: none }`). GitHub underlines on hover; md-viewer does not track
hover.

**STYLE-TEXT-4. Strikethrough** draws a line through the run, in the run's colour, at
0.3 em above the baseline, max(1 dp, size ÷ 16) thick, across the run's width. The line's top
edge is at baseline − 0.3 em, rounded to whole pixels; a code run's line spans its padding too.

**STYLE-TEXT-5. Inline code** is Go Mono at 85% of the surrounding font size (13.6 sp in body
text), colour unchanged, on a `CodeBg` rounded rectangle with a 6 dp corner radius. A code run
keeps the line height of its block (the run does not change it). The rectangle extends 0.2 em
above and below the glyph line box without changing the line height, and 0.4 em to the left and
right. This horizontal padding is part of the run's width. Each length is rounded to whole
pixels once (`code, tt { padding: .2em .4em; font-size: 85%; background-color; border-radius:
6px }`).
Inside a heading the code is 100% of the heading size with 0.2 em horizontal padding and none
vertical (`h1 code, … { padding: 0 .2em; font-size: inherit }`). When a code run wraps across
lines, each line's fragment gets its own rectangle, with the horizontal padding on both edges.

**STYLE-TEXT-6.** Text is aligned to the start (left). Paragraph text does not hyphenate.

### 3.5 Blocks

**STYLE-CODE-1. Code blocks** (fenced and indented, the same) are Go Mono at 13.6 sp (85% of
16), colour `Fg`, line height 1.45, on a `BgMuted` rounded rectangle with a 6 dp radius that
spans the full content width, with 16 dp padding on all sides (`pre { padding: 1rem; font-size: 85%; line-height: 1.45; background-color; border-radius: 6px }`). No syntax
highlighting is done; `Lang` is ignored.

**STYLE-CODE-2. Code lines wrap.** A code line longer than the box's inner width wraps at the
character that would overflow (`text.WrapGraphemes`). The text of a block with n newlines shows
n + 1 lines, so the empty text shows one empty line; the leading spaces of a line are kept. GitHub
instead scrolls a code block sideways (`pre { overflow: auto; word-wrap: normal }`); md-viewer has
no horizontal scrolling, so it wraps.

**STYLE-QUOTE-1. Quotes** draw a 4 dp wide bar (`.25em` of 16) in `Border` down their full
height at the left edge, and lay out their child blocks with 20 dp of left inset (the bar plus
16 dp padding) and 16 dp of right inset (`blockquote { padding: 0 1em; color; border-left:
.25em solid }`). Text inside a quote is drawn in `FgMuted`, except links (`Accent`) and code
blocks (`Fg`), which set their own colour. Inside a quote, headings of levels 1 to 5, inline
code and list markers are `FgMuted` as well; heading rules keep `BorderMuted`, rules and quote
bars keep `Border`, task boxes keep their own colours (STYLE-LIST-5), and table lines and row
fills keep theirs (STYLE-TABLE-1, STYLE-TABLE-3). Nested quotes nest.

**STYLE-RULE-1. A rule** is a 4 dp tall bar (`.25em`) in `Border` across the full content
width (`hr { height: .25em; background-color; border: 0 }`).

### 3.6 Lists

**STYLE-LIST-1.** A list indents its items' content by 32 dp from the list's left edge
(`ul, ol { padding-left: 2em }`); nested lists indent again from their item's content edge.

**STYLE-LIST-2. Unordered markers** are shapes drawn with clip operations, not glyphs. The
shapes are drawn by choice: the Go fonts do have ◦ and ▪. They are centred horizontally 11 dp
left of the item's content edge and vertically at the first line's baseline minus 0.27 × the
item's font size. The marker's rectangle has its top-left corner at the rounded centre minus
half its size, so a 5 dp shape is centred to within half a pixel:

| Nesting depth (lists around this one) | Marker                                                                                                                     |
| ------------------------------------- | -------------------------------------------------------------------------------------------------------------------------- |
| 0                                     | filled circle, 5 dp diameter, colour the item's text colour (`Fg`; `FgMuted` inside a quote)                               |
| 1                                     | circle outline, 5 dp outer diameter, 1 dp stroke inside it, colour the item's text colour (`Fg`; `FgMuted` inside a quote) |
| 2 or more                             | filled square, 5 dp side, colour the item's text colour (`Fg`; `FgMuted` inside a quote)                                   |

These are the browser defaults for `disc`, `circle`, `square`, which github-markdown-css does
not change for unordered lists. Depth counts ordered and unordered lists alike.

The first line's baseline in STYLE-LIST-2, 3 and 5 is that of the item's first text line. The
font is the body style (STYLE-TEXT-1) whatever the first block is. The line is found
recursively through quotes and nested lists, code block text included. A table's first line is
its header row's. When the first block has no text line (for example a rule or an image), the
baseline is that of an empty body line at the item's top, and the item's height is unchanged. An
empty item (`-`) is one empty body line.

**STYLE-LIST-3. Ordered markers** are text in the item's font and colour, the number followed
by `.`, right-aligned so that the marker's right edge is 4 dp left of the item's content edge,
on the first line's baseline. Numbering starts at `Start` and counts up. The numbering style
depends on the depth (lists around this one, ordered or not): depth 0 decimal (`1.`), depth 1
lower-roman (`i.`), depth 2 or more lower-alpha (`a.`; after `z.` comes `aa.`)
(`ol ol, ul ol { list-style-type: lower-roman }`, `ul ul ol, ul ol ol, ol ul ol, ol ol ol { list-style-type: lower-alpha }`).
A number with no CSS style in that range prints in decimal: 0 or less at any depth (`0.`), and
roman numerals above 3999 (`4000.`).

**STYLE-LIST-4. Item spacing.** In a tight list, consecutive items are 4 dp apart
(`li+li { margin-top: .25em }`) and the paragraphs and images that are direct children of an
item have no margins; a paragraph inside a quote in the item keeps its margins. In a loose
list, consecutive items are 16 dp apart and paragraphs inside an item keep their 16 dp bottom
margin (`li>p { margin-top: 1rem }`, collapsed with the paragraph margins).

**STYLE-LIST-5. Task items** draw no bullet or number, but they still count in the numbering. A
13 dp square box sits with its left edge 22 dp left of the item's content edge, centred
vertically on the first line's baseline minus 0.35 × font size, with a 3 dp corner radius
(`.task-list-item { list-style-type: none }`, `.task-list-item-checkbox { margin: 0 .2em .25em -1.4em }`;
13 dp is the browser's checkbox size). An `Open` box is filled `Canvas` with a 1 dp
`FgMuted` border. A `Done` box is filled `Accent` with no border and a check mark in `OnAccent`:
a 1.5 dp stroked polyline through the points (3, 6.5), (5.5, 9), (10, 4) dp relative to the
box's top-left corner. Boxes are not interactive. A list item that is a task in a list that also
has non-task items is still drawn as a task item, and the others keep their bullets.

### 3.7 Tables

**STYLE-TABLE-1. Cells** have 6 dp vertical and 13 dp horizontal padding and a 1 dp `Border`
line on every edge; adjacent cells share one line (`table th, table td { padding: 6px 13px; border: 1px solid }`, `border-collapse: collapse`).

**STYLE-TABLE-2. The header row** is Bold. Its background is `Canvas`, filled as a row
(STYLE-TABLE-3), so a table with n body rows has n + 1 fills. A fill is the band between its row's
horizontal lines across the table's width; the lines are painted over the fills.

**STYLE-TABLE-3. Body rows** alternate: the 2nd, 4th, 6th … body row (counting from 1) is
filled `BgMuted`, the others `Canvas` (`table tr { background-color }`,
`table tr:nth-child(2n) { background-color }`; the header row is in its own `thead`, so the
count restarts in `tbody`).

**STYLE-TABLE-4. Alignment.** A column marked `Left`, `Centre` or `Right` aligns every cell's
text that way. A column marked `None` aligns body cells left and header cells centred: the
browser's default for `th`, which github-markdown-css does not override. Alignment is per line: each
line of a wrapped cell is aligned in its box, by the line as laid out, its trailing space included.
A centred line leaves the spare whole pixels, rounded down, on its right. A line wider than its box
is start-aligned.

**STYLE-TABLE-5. Column widths.** Let W be the content width, and for each column let N be its
natural width: the widest of its cells laid out on one line, plus 26 dp padding and 1 dp of
border. Each column owns the border on its left, and the table has a closing 1 dp border on its
right. "Fits" means ΣN + 1 dp ≤ W. If it fits, each column is N wide and the table is ΣN + 1 dp
wide, left-aligned (`table { width: max-content; max-width: 100% }`). Otherwise the columns are
fitted to W − 1 dp: each is max(64 dp, ⌊N × (W − 1 dp) ÷ ΣN⌋) wide, in whole px, and cell text
wraps. Each length (the minimum, the padding and the border) is rounded to whole px once. If the
columns and the closing border are still wider than W, the table is clipped at W. GitHub scrolls
a too-wide table sideways; md-viewer has no horizontal scrolling.

**STYLE-TABLE-6. Rows.** A row is its tallest cell's text, with 6 dp above and below it, and a
1 dp border above it: the next row's border is 1 dp + 6 dp + that text + 6 dp below. Cell text is
vertically aligned to the top of the cell. A cell is laid out on one line, unwrapped, for its
natural width: a hard break (which GFM cannot give a cell) makes it as wide as its widest line.
An empty cell is one empty line. An image in a cell is its alt text (DOC-11), and inline code's
padding counts in its width.

### 3.8 Images

**STYLE-IMG-1. Local PNG and JPEG images** are shown. The destination is resolved with DOC-5;
only `LocalFile` targets whose decoded format is `png` or `jpeg` are drawn. An image wider or
taller than 8192 px, or of more than 40 million pixels, is not drawn: a texture that size can
exceed the GPU's limit, and the graphics layer panics when the GPU refuses one. Its header is
read before the window and gives its size, so such an image is a placeholder with REASON
`image too large: WxH` (STYLE-IMG-3). The cap is a judgement, not a measured limit: at the pixel
cap an image is about 160 MB as decoded (4 bytes per pixel), and up to twice that when its pixels
are converted for drawing.

**STYLE-IMG-2. Size.** One image pixel is one dp, so the image follows the zoom. An image wider
than the width available to it is scaled down, keeping its aspect ratio, to that width
(`img { max-width: 100% }`). The available width is the window's content width less any list
indent or quote inset around the image. Images are never scaled up. An image is left-aligned.
Each side of its box is rounded to whole pixels once, so a box may be 0 px tall.

**STYLE-IMG-3. Placeholder.** An image that is not drawn (remote, missing, unreadable, another
format, too large, or failed to decode) is shown as its alt text, or its destination when the alt
text is empty, in Italic `FgMuted` body text inside a 1 dp `Border` rectangle with 8 dp padding.
The rectangle hugs its text: its width is the widest line plus 2 × 9 dp, and its height is the
text's height plus 2 × 9 dp (the padding and the border on each side). Its text is not a text
link, so it stays `FgMuted` even inside a link (STYLE-IMG-4).

md-viewer prints one line to stderr for each such image occurrence in the Markdown, in document
order, after DOC-9's line and before the window opens:
`md-viewer: image not shown: DEST: REASON`. REASON is:

- `remote images are not loaded`, for an External target, or a Refused one with a host;
- `not a local file`, for a fragment (`#x`) or another Refused target;
- `unsupported format`, for a file that is not a PNG or JPEG, including one the header sniff
  does not recognise;
- `is a directory`, or `not a regular file`, for a path that is not a regular file (the file is
  not opened);
- `image too large: WxH`, from the size rule of STYLE-IMG-1;
- the text of the operating system's error after the path (`no such file or directory`,
  `permission denied`), or the decoder's error for a PNG or JPEG whose header is unreadable.

An image whose pixels fail to decode after the window has opened (SPEED-5) prints its line once,
when it is first laid out, and is a placeholder from then on (STYLE-IMG-5).

**STYLE-IMG-4. Linked images.** An image that is a link's content is clickable like a link
(LINK-1), and shows the hand cursor. The whole placeholder or box is the clickable area.

**STYLE-IMG-5.** Before an image's pixels are decoded (SPEED-5) its box is drawn at its final
size, filled `BgMuted`, so the layout does not move when the pixels arrive. The one exception is
an image whose pixels fail to decode: it becomes a placeholder when it is laid out, so its layout
moves once.

## 4. Links

**LINK-1.** A primary-button click on a link (press and release on the same link) acts on its
target (DOC-5):

| Target kind | Action                                                                                                            |
| ----------- | ----------------------------------------------------------------------------------------------------------------- |
| `External`  | open it with the platform opener (`xdg-open URL` on Linux)                                                        |
| `LocalFile` | if the file exists, open it with the opener; otherwise print `md-viewer: link target not found: PATH`             |
| `Fragment`  | nothing is opened; print `md-viewer: in-page links are not followed: DEST`                                        |
| `Refused`   | nothing is opened; print `md-viewer: link not opened: DEST: only http, https, mailto and local files are allowed` |

A `LocalFile` target exists when `os.Stat` succeeds. A directory counts (`[a]()` opens the
document's directory), and a symbolic link is followed, so a link to a missing file is not found.
When the stat fails for another reason (for example permission denied), nothing is opened and
md-viewer prints `md-viewer: cannot open link: PATH: REASON`, with REASON the operating-system
error text.

**LINK-2.** The opener is started with `exec.CommandContext`, with the target as a separate
argument and no shell. md-viewer does not wait for it before drawing the next frame; it waits
in a goroutine and, if the opener cannot start or exits non-zero, prints
`md-viewer: cannot open link: TARGET: REASON` to stderr. REASON is the text of the error from
starting or waiting (for example `exec: "xdg-open": executable file not found in $PATH`, or
`exit status 4`). The opener's own stdout and stderr are discarded, so only md-viewer's lines
appear. md-viewer never kills the opener, on a click or on exit. A failed link never ends the
program. If the line cannot be written, md-viewer invalidates the window, so the next frame
comes at once, not at the next input event; that frame fails with the error, and the window
closes as CLI-6 describes.

**LINK-3.** A `LocalFile` Markdown link opens through the opener like any other file, so the
desktop's default application for Markdown handles it (which may be md-viewer itself).

## 5. Scrolling

**SCROLL-1.** The document scrolls vertically only. In a frame only the visible blocks are laid out,
and the one block beyond each edge of the view that `layout.List` lays out too (`layout.List`).

**SCROLL-2. Keys.** A key acts on press, not release.

| Key                   | Moves the view                                                |
| --------------------- | ------------------------------------------------------------- |
| Down arrow / Up arrow | down / up by 40 dp                                            |
| Page Down / Page Up   | down / up by the viewport height minus 40 dp (at least 40 dp) |
| Space / Shift+Space   | as Page Down / Page Up                                        |
| Home / End            | to the top / to the bottom                                    |

The distances are in dp, so they scale with the zoom. Scrolling stops at the top and at the
bottom; it cannot move past either.

Modifiers: the arrows, Page Up/Down, Home and End act only with no modifier, so Shift+↓ or Ctrl+End
does nothing. Space acts with no modifier or with Shift. The keypad keys with the same names
(keypad arrows, Home and End) act like the main keys. Gio does not report keypad Page Up and Page
Down with NumLock off, so those never reach md-viewer (a Gio limit).

**SCROLL-3. Wheel.** A vertical scroll event without Ctrl moves the view by the event's
`Scroll.Y` in pixels, rounded to the nearest pixel, as Gio reports it (Gio reports 100 px per
wheel notch on Wayland and 10 px on X11). Horizontal scroll events are ignored.

**SCROLL-4. Scrollbar.** A vertical scrollbar is drawn at the right edge of the window (`material.List`
with `material.Scrollbar`, which reserves its gutter, not zoomed, 10 dp at the window's scale).
Dragging its indicator scrolls the document. A primary click on the track outside the indicator
centres the view on the click point. The indicator is drawn in `Fg` at alpha 150, and at alpha 200
while the pointer is over the bar (the material defaults).

## 6. Zoom

**ZOOM-1. Levels.** The zoom is one of these percentages: 50, 67, 75, 80, 90, 100, 110, 125,
150, 175, 200, 250, 300. It starts at 100.

**ZOOM-2. Controls.**

| Input                               | Effect            |
| ----------------------------------- | ----------------- |
| Ctrl and `+` or `=` (Shift allowed) | next higher level |
| Ctrl and `-`                        | next lower level  |
| Ctrl and `0`                        | 100               |
| Ctrl and a vertical wheel scroll    | see ZOOM-4        |

At 300 a further zoom in, and at 50 a further zoom out, does nothing.

**STYLE-ZOOM-3. Zoom scales everything inside the content area**: every dp and sp of the
document, including the padding (DOC-LAYOUT-1), the scroll distances (SCROLL-2) and images. It
is applied by multiplying the frame's `unit.Metric` (`PxPerDp` and `PxPerSp`) by the zoom
factor. Gio's window decorations and the scrollbar gutter (SCROLL-4) are not zoomed.

**ZOOM-4. Ctrl+wheel.** Each vertical scroll event with Ctrl held adds its `Scroll.Y` to an
accumulator. When the accumulator reaches −10 px or less the zoom goes one level up; at +10 px
or more, one level down; the accumulator is then reset to 0. One event changes the zoom by at
most one level. Ctrl+wheel never scrolls. The accumulator is reset whenever a threshold is
reached, even when the zoom is already at its limit and does not change. Nothing else resets it
(not releasing Ctrl, not a keyboard zoom).

**ZOOM-5. Reading position.** A zoom change keeps the block at the top of the view at the top,
and scales its offset into that block by the ratio of the new zoom to the old, rounded to the
nearest pixel. The list's top and bottom limits then apply, so near the end of the document the
block at the top can change (the view never moves past the end).

## 7. Word wrap

**WRAP-1.** Text wraps at word boundaries to the content width. A word that does not fit the rest
of its line moves to the next line (`text.WrapWords`). A word wider than a whole line breaks at
a character (`text.WrapHeuristically`), and only then. If not even one character fits a line,
the first character is shown alone past the line's end. A non-code run drops its leading spaces
where it starts a line. A break may fall between two runs of one word, as in
`gioui.org/x/styledtext` (`foo**bar**` may break between `foo` and `bar`). No text is clipped
or scrolled sideways, except a too-wide table (STYLE-TABLE-5).

**WRAP-2.** Resizing the window re-wraps all text in the next frame.

## 8. Window

**WIN-1. Title.** The window title is `NAME - md-viewer`, where NAME is the file's base name.

**WIN-2. Size.** The window opens at 980 × 900 dp, and cannot be made smaller than
320 × 240 dp.

**WIN-3. One window, one file.** md-viewer opens exactly one window showing exactly the one
FILE. It never opens another window or another file itself (LINK-3 hands files to the opener).

**WIN-4. Quit.** `q` (no Ctrl, Alt or Super; with or without Shift) and `Esc` (the same modifier
rule) close the window and exit 0. Closing the window by the window manager exits 0.

**WIN-5. No editing.** Nothing in the window changes the file, and there is no text cursor,
selection or clipboard.

**WIN-6. Wayland app ID.** The Wayland `app_id` and X11 class are `md-viewer` when the binary is
installed as `md-viewer` (Gio's default, the executable's base name). A renamed or symlinked
binary, or `go run`, gets its own base name instead.

## 9. Speed

The targets are measured on the owner's desktop, which has a display. The development
container has none, and a virtual display (Xvfb with Mesa's software renderer) is too unlike a
real one to measure against, so container and CI numbers are recorded as information, never
judged against the targets.

**SPEED-1. Work before the window.** Before the window is created, md-viewer parses its
arguments, stats and reads the file, parses it into the document model, reads each local
image's header (`image.DecodeConfig`, for its size), and loads the six font faces. Building the
view, the viewer's state and its theme is in-memory work done in the same preparation. No other
I/O or parsing happens before the window opens. Gio's own work at window creation is not
md-viewer's and is outside this rule.

**SPEED-2. `--time`.** With `--time`, after the first frame has been submitted (the first
`FrameEvent.Frame` call has returned), md-viewer prints exactly one line to stdout:

```text
read_ms=0.4 parse_ms=12.3 fonts_ms=6.1 first_frame_ms=88.0
```

`read_ms` is the read (stat, read and NUL check, DOC-13), `parse_ms` the parse into the document
model (DOC-13), and `fonts_ms` the loading of the six faces and the making of the text shaper.
`first_frame_ms` is the time from the first statement of `main` to the end of the first frame.
Each value has one decimal place, rounded to the nearest tenth of a millisecond; the tie between
two tenths is not specified. md-viewer then closes the window and exits 0. The time before `main` (exec,
dynamic linking, Go runtime start) is not included; the wall-clock measurement (SPEED-7)
includes it.

**SPEED-3. Targets.** Release build (`go build -trimpath`), on the owner's desktop under
GNOME on Wayland, median of 10 runs:

| Measure                                                      | Target     |
| ------------------------------------------------------------ | ---------- |
| `first_frame_ms` for `testdata/elements.md`                  | ≤ 120 ms   |
| `first_frame_ms` for `testdata/large.md`                     | ≤ 150 ms   |
| wall clock of `md-viewer --time testdata/large.md` (SPEED-7) | ≤ 250 ms   |
| `BenchmarkParseLarge` (read excluded)                        | ≤ 25 ms/op |
| `BenchmarkFirstViewportLarge`                                | ≤ 15 ms/op |
| `BenchmarkLoadFonts`                                         | ≤ 10 ms/op |

A target that is missed is reported to the owner with the measured number; it is not quietly
moved.

**SPEED-4. Fonts.** No system font scan, no font cache file, no fontconfig: the six embedded
faces only (STYLE-FONT-1).

**SPEED-5. Images.** An image's pixels are decoded only when the top-level block that holds it
first becomes visible, which includes the one block beyond each edge of the view that `layout.List`
lays out (SCROLL-1), in a goroutine, one image at a time, in the order the images are first
laid out. The window is invalidated when each is ready. The goroutine exits when no image is
waiting, and the next request starts it again. The first frame never waits for image pixels.

**SPEED-6. Nothing else.** No `init` functions in md-viewer's packages, no background goroutines
other than the window loop, image decoding and the link opener's wait, no network, no D-Bus, no
file watching, no configuration reads. Gio's own goroutines and platform work (on X11, its
connection to IBus over D-Bus and its reads of the machine ID and IBus configuration) are
Gio's, and outside this rule.

**SPEED-7. Benchmarks and measurement.** Go benchmarks in the repository measure parse, first
viewport layout and font loading on `testdata/large.md` (ARCHITECTURE "Benchmarks"). The owner
measures wall clock with
`hyperfine --warmup 3 --runs 20 'md-viewer --time testdata/large.md'`.

## 10. Non-goals

md-viewer will not have, and a change adding any of these is out of scope:

- editing, a text cursor, selection, copy to clipboard;
- syntax highlighting;
- tabs, more than one window, more than one file, a file picker, a recent-files list;
- live reload or watching the file for changes;
- search, a table of contents, heading anchors, following in-page `#` links;
- rendering HTML, math, Mermaid or other diagrams, footnotes, GitHub alerts (`> [!NOTE]` is an
  ordinary quote), emoji shortcodes;
- loading remote images or anything else from the network;
- horizontal scrolling;
- printing, export, PDF;
- font, colour or size settings, configuration files;
- reading from standard input;
- following the desktop's dark-mode setting (STYLE-PAL-2), unless the owner decides otherwise.
