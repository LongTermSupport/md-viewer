# md-viewer milestones

The order md-viewer is built in. Each milestone is one working session: do it in order, finish
it completely, then stop. [SPEC.md](SPEC.md) says what to build (the numbered expectations),
[ARCHITECTURE.md](ARCHITECTURE.md) says where and with which types and APIs.

## Rules for every milestone

1. **Read first.** Read the expectations the milestone lists, in SPEC.md, and the package's
   section of ARCHITECTURE.md. If the two disagree, or an expectation cannot be met with the
   API that ARCHITECTURE.md names, stop and report it: do not invent a third behaviour.
2. **Tests first, seen red.** For each test listed, write it before the code. Give the code a
   stub that compiles and returns zero values, run the test, and see it fail on its assertion
   (a compile error does not count as red). Then write the code until it passes. The commit
   message lists the tests that were seen red.
3. **Fail fast.** No ignored errors (`_ =` on an error, empty `if err != nil {}`), no
   skip-and-continue. A function that can fail returns an error; only `main` decides the exit
   code. No `panic` for expected conditions.
4. **Strict typing.** Use the named types of ARCHITECTURE.md (`doc.Style`, `doc.TargetKind`,
   `app.ExitCode`, …), not bare `int` or `string`. Every switch over an enum lists every value
   (golangci's `exhaustive` checks it).
5. **DRY.** A number from SPEC.md is written once, as a named constant beside the code that
   uses it, with the expectation number in its comment. Tests refer to the constant only when
   the test is about something else; a test *of* that value writes the number literally, so a
   wrong constant fails.
6. **No suppressions.** Do not add `//nolint`, `//lint:ignore` or a go-qa record entry. If a
   linter finding seems wrong, stop and report it.
7. **Standard library only in tests.** No assertion library.
8. **Finish.** A milestone is done only when all of its verification commands pass, in the
   container. Then mark it done in the status list below, commit, and push to `origin main`.

## Environment

The work happens inside md-viewer's own ccy container, built from `.claude/ccy/Dockerfile`. It
has Go 1.26.9, gcc, Gio's build headers, Xvfb with Mesa's software renderer, and `gh`. It has
no real display. So:

- everything builds, including the window code and the binary;
- every test runs, because tests never open a window;
- the binary can be run under a virtual display, `xvfb-run -a`, which proves it opens a window
  and draws a frame, but its timings are information only (SPEC section 9);
- the speed targets are measured by the owner on the desktop (M11).

The GitHub Actions workflow (`.github/workflows/ci.yml`) repeats the build, the tests and the
smoke run on `ubuntu-24.04` and uploads the binary as an artifact. That run is the proof that
the code builds from a clean checkout.

The gate is the plain Go toolchain; go-qa-ci is an add-on, never a requirement. go-qa-ci is
private, so in CI its `go-qa` job cannot install go-qa until it is public and fails at that
step; that red job is expected and is not a reason to change the workflow.

## Verification commands

Every milestone runs the **standard checks**, from the repository root:

```bash
gofmt -l .        # must print nothing
go vet ./...
go test -race -count=1 ./...
```

Where `go-qa` is installed it adds to these (`go-qa run`); it is never required. From M10,
also the **smoke run**:

```bash
go build -trimpath -o dist/md-viewer ./cmd/md-viewer
xvfb-run -a dist/md-viewer --time testdata/elements.md
```

which must exit 0 and print one line matching
`^read_ms=[0-9]+\.[0-9] parse_ms=[0-9]+\.[0-9] fonts_ms=[0-9]+\.[0-9] first_frame_ms=[0-9]+\.[0-9]$`.

After pushing, check the CI run with `gh run list --limit 3` and `gh run view <id>`: the
`build` job must be green. The `go-qa` job is red only at its install step, for the
private-repository reason, while go-qa-ci is private; any other failure in it is a real failure.

## Status

- [x] M0 Scaffolding, CI and the command line
- [x] M1 Document model: blocks and inline runs
- [x] M2 Document model: links, tables, images, HTML, loading
- [x] M3 Render styles, palettes and fonts
- [ ] M4 Inline text flow
- [ ] M5 The view and simple blocks
- [ ] M6 Lists
- [ ] M7 Tables
- [ ] M8 Images
- [ ] M9 The viewer: scroll, zoom, links, quit
- [ ] M10 The window and the binary
- [ ] M11 Speed: fixtures, benchmarks, measurement

---

## M0 Scaffolding, CI and the command line

**Covers:** CLI-1, CLI-2, CLI-3, CLI-4, CLI-5, CLI-7, CLI-11.

**First step: check the environment.** Run `go version` (must say `go1.26.9`),
`pkg-config --exists wayland-client egl xkbcommon x11 && echo ok`, `command -v xvfb-run` and
`gh auth status`. If any fails, stop and report it: the container image is wrong, and the fix
is in `.claude/ccy/Dockerfile` and a rebuild by the owner, not an install by hand.

**Files:**

- `go.mod`: `module github.com/LongTermSupport/md-viewer`, `go 1.26.9`. No requirements yet
  beyond what the code imports (`go mod tidy`).
- `.gitignore`: `/dist/`, `*.test`, `*.out`, `/untracked/`.
- `qa/go-qa-ci.ref`: the single line `f125f711e6f6f59fbe5db1443bb4f41df22224d7`, the commit
  where go-qa installs its own tools. No installer script and no tool
  directory live in this repository: the Go gate needs only the Go toolchain.
- `qa/go-qa.toml`: only these comment lines (the defaults apply):
  `# go-qa configuration; see go-qa-ci's README, "Configuration".` and
  `# Empty on purpose: go-qa's defaults apply. A change to them is a reviewed diff here.`
- `.github/workflows/ci.yml`, exactly as below.
- `internal/app/cli.go`: `Options`, `UsageError`, `ParseArgs`, `Usage`, `ExitCode` and its
  constants.
- `internal/app/version.go`: `VersionLine(info *debug.BuildInfo, ok bool) string`.
- `cmd/md-viewer/main.go`: parses the arguments; prints usage (stdout, exit 0), the version
  line (stdout, exit 0), or a usage error (CLI-7, exit 2). With a valid FILE it prints
  `md-viewer: not implemented yet` to stderr and exits 1; M10 replaces that line (M2 keeps it, P13).
- `README.md` already exists; change nothing in it except to correct an error you find.

`.github/workflows/ci.yml`:

```yaml
name: CI

# build: vet, race tests and a release build on Ubuntu with Gio's build dependencies; the
#   binary is uploaded as an artifact. From M10 it also runs the binary under Xvfb.
# go-qa: go-qa-ci over the module, an add-on to the plain Go gate in `build`. go-qa installs
#   its own pinned tools. go-qa-ci is private, so until it is public this job fails at its
#   install step. It must not be skipped or made optional.
on:
  push:
    branches: ["**"]
  pull_request:
  workflow_dispatch:

permissions:
  contents: read

concurrency:
  group: ci-${{ github.ref }}
  cancel-in-progress: true

env:
  GOTOOLCHAIN: local
  # Gio's Linux build dependencies (see .claude/ccy/Dockerfile for why each is needed), and
  # Xvfb with Mesa's software renderer for the smoke run.
  APT_PACKAGES: >-
    gcc libc6-dev pkg-config libwayland-dev libxkbcommon-dev libxkbcommon-x11-dev
    libx11-dev libx11-xcb-dev libxcursor-dev libxfixes-dev libegl-dev libgles-dev
    libvulkan-dev libffi-dev xvfb xauth libegl-mesa0 libgl1-mesa-dri

jobs:
  build:
    name: Build, test, artifact
    runs-on: ubuntu-24.04
    timeout-minutes: 20
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          persist-credentials: false
      - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
        with:
          go-version-file: go.mod
      - name: Install Gio build dependencies
        run: |
          read -ra pkgs <<< "${APT_PACKAGES}"
          sudo apt-get update
          sudo apt-get install -y --no-install-recommends "${pkgs[@]}"
      - name: gofmt -l prints nothing
        run: test -z "$(gofmt -l .)"
      - run: go vet ./...
      - run: go test -race -count=1 ./...
      - run: go build -trimpath -o dist/md-viewer ./cmd/md-viewer
      - uses: actions/upload-artifact@cf430e030ddbb5b0abf93d22962f4752f3646cd9 # v7.0.2
        with:
          name: md-viewer-linux-amd64
          path: dist/md-viewer
          if-no-files-found: error

  go-qa:
    name: go-qa (go-qa-ci)
    runs-on: ubuntu-24.04
    timeout-minutes: 30
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1
        with:
          # go-qa's record ratchet compares against the merge base, which needs the history.
          fetch-depth: 0
          persist-credentials: false
      - uses: actions/setup-go@b7ad1dad31e06c5925ef5d2fc7ad053ef454303e # v7.0.0
        with:
          go-version-file: go.mod
      - name: Install Gio build dependencies
        run: |
          read -ra pkgs <<< "${APT_PACKAGES}"
          sudo apt-get update
          sudo apt-get install -y --no-install-recommends "${pkgs[@]}"
      - name: Install go-qa at the pinned go-qa-ci commit (fails while go-qa-ci is private)
        env:
          GOPRIVATE: github.com/LongTermSupport/go-qa-ci
        run: go install "github.com/LongTermSupport/go-qa-ci/cmd/go-qa@$(tr -d '[:space:]' < qa/go-qa-ci.ref)"
      - name: go-qa run --ci
        run: go-qa run --ci
```

**Tests first** (`internal/app/cli_test.go`, `version_test.go`), table-driven:

- `TestParseArgsFile`: `["a.md"]`, `["--dark", "a.md"]`, `["a.md", "--dark"]`,
  `["--time", "a.md", "--dark"]` give the right `Options`.
- `TestParseArgsDoubleDash`: `["--", "--dark"]` gives `Path: "--dark"`, `Dark: false`;
  `["-"]` gives `Path: "-"`.
- `TestParseArgsHelpVersion`: `["--help"]`, `["-h"]`, `["--version"]`, `["--help", "--version"]` (Help wins), each without FILE, no error.
- `TestParseArgsRepeated`: `["--dark", "--dark", "a.md"]` is `Dark: true`.
- `TestParseArgsUsageErrors`: `[]` → `md-viewer: missing FILE argument`; `["a", "b"]` →
  `md-viewer: expected one FILE, got 2`; `["--nope", "a"]` and `["-dark", "a"]` and
  `["--dark=1", "a"]` → `md-viewer: unknown flag: …`; `["--x", "a", "b"]` reports the flag
  (CLI-7's order). Each error is a `*UsageError`, checked with `errors.As`.
- `TestUsageText`: `Usage` equals CLI-4's text byte for byte (copy it into the test).
- `TestVersionLine`: a tagged version; `(devel)` with a revision and `vcs.modified=true`;
  `(devel)` without settings; `ok == false` gives `md-viewer unknown`.
- `TestMainUsageExit` in `cmd/md-viewer/main_test.go`: builds the binary once with
  `go build` into `t.TempDir()` and runs it with `exec.CommandContext(t.Context(), …)` (go-qa's
  `gqaci_exec_context` requires the context) for `--help` (exit 0,
  stdout is the usage), no arguments (exit 2, stderr is CLI-7's two lines) and `--version`
  (exit 0, stdout starts with `md-viewer `).

**Verification:** the environment check above, then the standard checks. Push, and check the
CI run: `build` green, `go-qa` red only at its install step (go-qa-ci is private).

**Done when:** the standard checks pass in the container, CI's `build` job is green, and CI's
`go-qa` job fails only at its install step with the private-repository error.

---

## M1 Document model: blocks and inline runs

**Covers:** DOC-1, DOC-2, DOC-3, DOC-6, DOC-7, DOC-8, DOC-12, DOC-14, DOC-15.

**Files:** `internal/doc/model.go` (all model types of ARCHITECTURE.md, including those M2
fills in), `internal/doc/parse.go`, `internal/doc/parse_test.go`, `internal/doc/model_test.go`
(written before `model.go`). Add goldmark v1.8.6 to `go.mod` (`go get github.com/yuin/goldmark@v1.8.6`).

**Tests first** (each parses an inline Markdown string with `Parse(src, "/doc")` and compares
`Blocks` with `reflect.DeepEqual`, printing both with `%#v` on failure):

- `TestHeadings`: `#` to `######`, and setext `===` and `---`, give the right levels and runs.
- `TestParagraphRuns`: `*a*`, `_a_`, `**a**`, `***a***`, `~~a~~`, `` `a` ``, and a mix in one
  paragraph give the right runs and style flags; adjacent equal runs are merged.
- `TestSoftAndHardBreaks`: a soft break is a space; two trailing spaces and a trailing
  backslash each give a `BreakRun`.
- `TestWhitespace`: `a   b` gives `a b`; `` `a   b` `` keeps three spaces; leading and trailing
  spaces are trimmed.
- `TestCodeBlocks`: a fenced block with `go` (and `go extra` info) keeps `Lang: "go"` and the
  text without the final newline; an indented block; a tab expands to the next multiple of 8.
- `TestQuotesAndRules`: a quote holding a paragraph and a nested quote; `---`, `***`, `___`
  each give `Rule`.
- `TestLists`: unordered, ordered with `Start: 3`, nested two levels, tight and loose, task
  items `[ ]`, `[x]`, `[X]` (checkbox text removed).
- `TestUnsupportedSyntaxIsText`: a footnote reference `[^1]` and its definition, and
  `:smile:`, come out as plain text.
- `TestStyleFlagsDisjoint` (`model_test.go`): the four style flags are non-zero and disjoint.
- `TestUnmappedKindIsError`: an unmapped block kind and an unmapped inline kind each return an
  error naming the kind.
- `TestNormaliseRuns`: the run merge, collapse and trim rules on hand-built runs.
- `TestLinkReferenceDefinitionDropped`: `[a]: /url` followed by text gives only the text.
- `TestEscapesAndEntities`: backslash escapes and entity references resolve outside code.
- `TestEveryKindIsMapped`: parse an inline constant holding every M1 construct; no error, and
  every M1 block kind and style flag occurs. (M2 adds the `testdata/elements.md` version.)
- `TestParseDeterministic`: parsing that constant twice gives `DeepEqual` models.
- `TestInvalidUTF8`: `"a\xffb"` parses without error and the bytes pass through.

**Verification:** the standard checks.

**Done when:** all listed tests pass, and the standard checks pass.

---

## M2 Document model: links, tables, images, HTML, loading

**Covers:** DOC-4, DOC-5, DOC-9, DOC-10, DOC-11, DOC-13, CLI-8, CLI-10.

**Files:** `internal/doc/link.go`, `internal/doc/load.go`, additions to `parse.go`, tests
`link_test.go`, `load_test.go`, additions to `parse_test.go`; `internal/app/prepare.go` with
the first part of `Prepare` (arguments, then `doc.Load`, printing CLI-8's messages and the
DOC-9 line); `cmd/md-viewer/main.go` uses `Prepare`.

**Before the tests:** `testdata/elements.md` needs a real indented code block and a setext
heading (it has neither now), and its misleading fence (a fenced block where an indented one
is meant) renamed or replaced. M1's `TestEveryKindIsMapped` and `TestParseDeterministic` run
on this file in this milestone.

**Tests first:**

- `TestResolveLink` (table): every row of DOC-5's table, plus `HTTPS://X`, `other.md#part`,
  `../up.md`, `/abs/x.md`, `a%20b.md`, `file:///tmp/x.md`, `javascript:alert(1)`, `#top`,
  a destination that fails `url.Parse`, and `javascript&colon;alert(1)`, `file://localhost/x`,
  `file:///x%20y`, `C:\x.md` and `?q`.
- `TestLinkRuns`: `[a *b*](x.md)` gives two runs sharing one link; `<https://e.org/>`,
  a bare `https://e.org/`, `<a@example.com>` give DOC-4's runs.
- `TestTables`: alignments `:--`, `:-:`, `--:`, `---`; header and body cells with runs; a
  short row is padded.
- `TestImagesSplitParagraphs`: `a ![x](i.png) b` gives Paragraph, Image, Paragraph;
  `![x](i.png)` alone gives one Image; `[![x](i.png)](https://e.org/)` gives an Image with a
  `Link`; an image in a heading becomes its alt text.
- `TestHTMLDropped`: an HTML block, a comment and `a <span>b</span> c` give no HTML content,
  the text `a b c`, and `DroppedHTML` equal to the number of fragments.
- `TestFootnoteDefinitionCaveat`: `[^1]: note` alone gives no block and `[^1]` gives a link
  run (DOC-1's one-word caveat). M1 cannot test it, since its Link kind is unmapped until now.
- `TestEveryKindIsMapped` and `TestParseDeterministic` move to `testdata/elements.md`, as M1's
  versions use an inline constant (see M1).
- `TestLoadErrors` (in `t.TempDir()`): a missing file, a directory, `/dev/zero` (not a regular
  file), a file over `MaxFileSize` (create it sparse with `os.Truncate`, so it is not written out), and a file
  with a NUL byte; each gives its error type with the right fields. (A permission failure is
  not tested: the container runs as root, which reads mode-0 files; the missing-file case
  already proves REASON is passed through.)
- `TestLoadOK`: `Load` of `testdata/elements.md` with a fake clock that advances 1 ms per
  call gives `Name: "elements.md"`, an absolute `Dir`, and `LoadStats` with `Read == Parse == 1 ms` (DOC-13).
- `TestPrepareFileErrors` (`internal/app`): CLI-8's four messages, byte for byte, exit 1.
- `TestPrepareHTMLLine`: a file with HTML prints DOC-9's line to stderr.
- `TestPrepareArgs` (`internal/app`): help, version and usage output, which move into `Prepare`.

**Verification:** the standard checks.

**Done when:** all listed tests pass, `go run ./cmd/md-viewer nosuchfile.md` prints
`md-viewer: cannot read nosuchfile.md: no such file or directory` and exits 1, and the standard
checks pass.

---

## M3 Render styles, palettes and fonts

**Covers:** STYLE-PAL-1, STYLE-FONT-1, STYLE-FONT-3, STYLE-FONT-4, STYLE-TEXT-1,
STYLE-TEXT-2 (the numbers), DOC-LAYOUT-1, DOC-LAYOUT-3, DOC-LAYOUT-4, STYLE-LIST-3
(`OrderedMarker`), STYLE-TABLE-5 (`ColumnWidths`), SPEED-4.

**Files:** `internal/render/palette.go`, `style.go`, `fonts.go`, and their `_test.go` files.
Add `gioui.org v0.10.3` and `golang.org/x/image v0.26.0` to `go.mod`.

**Tests first:**

- `TestPalettes`: every colour of STYLE-PAL-1's table, light and dark, written as literal
  `color.NRGBA{…}` values in the test (convert the hex by hand; `#d1d9e0b3` is
  `{0xd1, 0xd9, 0xe0, 0xb3}`).
- `TestHeadingStyle`: for each level, size, line height, padding, rule and colour from
  STYLE-TEXT-2.
- `TestRunStyle`: body text 16 sp, 1.5; each style flag picks the face of STYLE-FONT-3/4;
  `Code` is 85% (13.6 sp) in body and 100% in a heading; links are `Accent`.
- `TestMargins` and `TestGap`: DOC-LAYOUT-4's table, nested lists, and the collapse rule
  (`Gap(16, 24) == 24`, `Gap(24, 0) == 24`).
- `TestPadding`: 768 dp and wider → 45; 767 → 15.
- `TestOrderedMarker`: depth 0: 1 → `1.`, 12 → `12.`; depth 1: 1 → `i.`, 4 → `iv.`,
  9 → `ix.`, 14 → `xiv.`; depth 2: 1 → `a.`, 26 → `z.`, 27 → `aa.`.
- `TestColumnWidths`: fits (natural kept), too wide (scaled), minimum 64 applied.
- `TestLoadFaces`: six faces; the typefaces are `Go` and `Go Mono` with the weights and
  styles of STYLE-FONT-1; a `NewShaper` lays out `"Hello"` to a positive width.
- `TestNewShaper`, "no system fonts" (SPEED-4): with an empty collection and `NoSystemFonts`,
  `"Hello"` is not laid out from a system font. M11 keeps its CJK test (`TestNoSystemFonts`).
- `BenchmarkLoadFonts`: `LoadFaces` then `NewShaper`, with `b.ReportAllocs()`.

**Verification:** the standard checks, and
`go test -run '^$' -bench LoadFonts -benchtime 20x ./internal/render` runs.

**Done when:** all listed tests pass and the standard checks pass.

---

## M4 Inline text flow

**Covers:** WRAP-1, STYLE-TEXT-3, STYLE-TEXT-4, STYLE-TEXT-5, STYLE-TEXT-6, DOC-3 (breaks
in layout), DOC-15 (U+FFFD shown).

**Files:** `internal/render/trace.go` (`Trace`, `Mark`, `MarkKind`), `internal/render/flow.go`
and `flow_test.go`. Read `gioui.org/x/styledtext/styledtext.go` at v0.10.3 (download with
`go mod download -json gioui.org/x@v0.10.3`; do not add it to `go.mod`) for the algorithm,
and cite it in `flow.go`'s doc comment.

The flow's entry point (unexported is fine) takes the runs, a base `TextStyle`, the palette,
the link click states, and `gtx`; it returns `layout.Dimensions` and appends marks to the
trace. Use the headless test pattern of ARCHITECTURE.md with `Metric{PxPerDp: 1, PxPerSp: 1}`.

**Tests first:**

The tests are in `trace_test.go` and `flow_test.go`, 16 in all: `TestTraceAdd`, then
`TestFlowLineHeight`, `TestFlowWrapsToWidth`, `TestFlowLongWord`, `TestFlowBreakRun`,
`TestFlowCodeBackground`, `TestFlowCodeInHeading`, `TestFlowStrike`, `TestFlowLinkArea`,
`TestFlowLinkClick`, `TestFlowLinkCursor`, `TestFlowInvalidUTF8`, `TestFlowUnknownRunKind`,
`TestFlowNilTrace`, `TestFlowEmptyRuns` and `TestFlowLeadingSpace`.

- `TestFlowWrapsToWidth`: a 60-word paragraph at 800 px and at 200 px: the second is taller,
  and at each width every `Glyphs` mark's `Rect.Max.X` is ≤ the width.
- `TestFlowLongWord`: a 300-character word at 200 px breaks across lines, none wider than 200.
- `TestFlowLineHeight`: one line of body text is 24 px tall (16 × 1.5); two lines are 48.
- `TestFlowBreakRun`: `a`, `BreakRun`, `b` gives two lines even at 800 px.
- `TestFlowCodeBackground`: a code run gives one `CodeBg` mark in `CodeBg` colour whose
  rectangle contains the run's `Glyphs` mark, wider than it by 0.4 em each side; when the run
  wraps, one `CodeBg` mark per line.
- `TestFlowCodeInHeading`: code in a heading has 0.2 em horizontal padding and none vertical.
- `TestFlowStrike`: a struck run gives a `Strike` mark across its width, 1 px tall at 16 sp.
- `TestFlowLinkArea`: a link run gives a `LinkArea` mark with the run's target, and its
  `Glyphs` mark has colour `Accent`.
- `TestFlowLinkClick`: lay out a link, `r.Frame(&ops)`, queue a primary press and release
  inside its `LinkArea` with `r.Queue(pointer.Event{…})`, lay out again: the flow reports the
  link clicked once.
- `TestFlowLinkCursor`: over a link the pointer cursor is the hand (`CursorPointer`), over
  plain text it is `CursorDefault`.
- `TestFlowInvalidUTF8`: `"a\xffb"` lays out with three glyph clusters and no error; each
  invalid byte is one U+FFFD.
- `TestFlowUnknownRunKind`: a run kind the flow does not know is an error that names it.
- `TestFlowNilTrace`: a nil `Trace` gives the same dimensions as a recording one.
- `TestFlowEmptyRuns`: no runs give one empty line of 24 px and no `Glyphs` marks.
- `TestFlowLeadingSpace`: a non-code run that starts a line drops its leading space; a code
  run keeps it.

**Verification:** the standard checks.

**Done when:** all listed tests pass and the standard checks pass.

---

## M5 The view and simple blocks

**Covers:** STYLE-TEXT-2 (layout), STYLE-CODE-1, STYLE-CODE-2, STYLE-QUOTE-1, STYLE-RULE-1,
DOC-LAYOUT-1, DOC-LAYOUT-2, DOC-LAYOUT-3 (applied), STYLE-PAL-3 (inside the view).

**Files:** `internal/render/view.go`, `blocks.go`, and their tests. `View.Layout` dispatches
on the block type; lists, tables and images return an error-free placeholder of height 0 until
M6 to M8 (each of those milestones replaces its placeholder, and a test in that milestone
proves it).

**Tests first:**

- `TestViewLen`: `elements.md` gives the number of top-level blocks of its model.
- `TestViewFirstAndLastPadding`: block 0 starts 45 px down at 800 px wide (`Glyphs` mark at
  y ≥ 45) and the last block ends 45 px above its bottom; at 700 px wide, 15.
- `TestViewGaps`: paragraph then heading: the heading's glyphs start 24 px after the
  paragraph's last line (collapse of 16 and 24).
- `TestHeadingRule`: h1 and h2 have a `HeadingRule` mark 1 px tall, full content width, colour
  `BorderMuted`, directly under the padding; h3 has none.
- `TestCodeBlock`: a `CodeBlockBg` mark in `BgMuted` spanning the content width, with the
  glyphs inset 16 px; a long line wraps inside the box (STYLE-CODE-2).
- `TestQuote`: a `QuoteBar` mark 4 px wide in `Border` as tall as the quote; child glyphs
  start 20 px right of the bar's left edge and are `FgMuted`.
- `TestRule`: a `RuleBar` 4 px tall in `Border`, full content width, with 24 px above and
  below.
- `TestNoMaxWidth`: at 2000 px wide a long paragraph's glyphs reach past 980 px.

**Verification:** the standard checks.

**Done when:** all listed tests pass and the standard checks pass.

---

## M6 Lists

**Covers:** STYLE-LIST-1, STYLE-LIST-2, STYLE-LIST-3, STYLE-LIST-4, STYLE-LIST-5.

**Files:** `internal/render/list.go` and `list_test.go`; the M5 placeholder for `doc.List` is
removed.

**Tests first:**

- `TestListIndent`: item glyphs start 32 px right of the list's left edge; a nested list's
  items 64 px.
- `TestBullets`: depths 0, 1, 2 give `Bullet` marks of the three shapes (record the shape in
  the mark's `Text`: `disc`, `circle`, `square`), 5 px, centred 11 px left of the content
  edge.
- `TestOrderedNumbers`: `Number` marks with texts `3.`, `4.` for `Start: 3`; nested `i.`,
  then `a.`; right edge 4 px left of the content edge.
- `TestTightAndLooseSpacing`: tight items 4 px apart; loose items 16 px apart.
- `TestTaskBoxes`: an `Open` item has a 13 px `TaskBox` mark with `Canvas` fill and no
  bullet; a `Done` item's box is `Accent`; the box's left edge is 22 px left of the content
  edge.

**Verification:** the standard checks.

**Done when:** all listed tests pass and the standard checks pass.

---

## M7 Tables

**Covers:** STYLE-TABLE-1 to STYLE-TABLE-6.

**Files:** `internal/render/table.go` and `table_test.go`; the M5 placeholder for `doc.Table`
is removed.

**Tests first:**

- `TestTableNaturalWidth`: a narrow table at 800 px is narrower than 800 and left-aligned.
- `TestTableTooWide`: a table of long cells at 400 px is at most 400 wide and its cell text
  wraps (taller rows).
- `TestTableLines`: `TableLine` marks, 1 px, `Border`, on every cell edge, shared between
  neighbours (an n × m table has m + 1 vertical and n + 2 horizontal lines, counting the
  header row).
- `TestTableHeaderBold` and `TestTableStripes`: header glyphs use the Bold face; body rows 2
  and 4 have a `RowFill` in `BgMuted`, rows 1 and 3 in `Canvas`.
- `TestTableAlignment`: in a wide column, `Right` text ends 13 px from the cell's right
  edge, `Centre` is centred, `None` body text starts 13 px from the left and `None` header
  text is centred.
- `TestTableRowHeight`: a row is as tall as its tallest cell; short cells' text is at the top.

**Verification:** the standard checks.

**Done when:** all listed tests pass and the standard checks pass.

---

## M8 Images

**Covers:** STYLE-IMG-1 to STYLE-IMG-5, SPEED-5.

**Files:** `internal/render/image.go` and `image_test.go`; the fixtures of ARCHITECTURE.md
under `testdata/img/`, written by `TestImageFixtures`; the M5 placeholder for `doc.Image` is
removed. `NewImages` is called by `app.Prepare` (add it there with a test).

**Tests first:**

- `TestImageFixtures`: with `MDV_UPDATE_FIXTURES=1` writes the four files; without it, checks
  they decode to the stated sizes and colours.
- `TestImageNaturalSize`: `red-4x3.png` at zoom 1 gives an `ImageBox` 4 × 3 px; with
  `Metric{PxPerDp: 2}` 8 × 6.
- `TestImageScaledDown`: a 1000 × 500 image (generated in the test into `t.TempDir()`) at
  600 px content width is 600 × 300.
- `TestImagePlaceholders`: missing file, GIF, broken PNG and a remote URL each give a
  `Placeholder` mark with the alt text (or the destination when alt is empty) and one stderr
  line of STYLE-IMG-3's form each.
- `TestImageLazyDecode`: before the block is laid out, no pixels are decoded; after
  `Layout` the decode starts, the bound invalidate function is called once when it is done,
  and the next layout has an `ImagePixels` mark. Before that, the box is `BgMuted` at its
  final size. Synchronise with a channel the test controls, never with a sleep.
- `TestLinkedImage`: an image with a `Link` has a `LinkArea` mark covering it.

**Verification:** the standard checks.

**Done when:** all listed tests pass and the standard checks pass.

---

## M9 The viewer: scroll, zoom, links, quit

**Covers:** SCROLL-1 to SCROLL-4, ZOOM-1, ZOOM-2, STYLE-ZOOM-3, ZOOM-4, ZOOM-5, WRAP-2,
LINK-1, LINK-2, LINK-3, WIN-4 (the key handling), WIN-5.

**Files:** `internal/app/viewer.go`, `keys.go`, `zoom.go`, `opener.go`, `opener_linux.go`,
`opener_other.go`, and tests. The opener's command is built with `exec.CommandContext` and
started with `Start`; the wait runs in a goroutine.

**Tests first:**

- `TestActionFor` (table): every row of SCROLL-2, ZOOM-2 and WIN-4, with and without
  modifiers; `q` with Ctrl is `None`; a key release is `None`.
- `TestZoomLevels`: `In` from 100 is 110, from 300 is 300; `Out` from 50 is 50; `Reset` is
  100; `Metric` multiplies `PxPerDp` and `PxPerSp`.
- `TestViewerKeysScroll` (router): after a frame, Down moves `Position.Offset` by 40 px at
  zoom 100 and 80 px at zoom 200; Page Down by the viewport height minus 40; Home returns to
  `Position{}`; End shows the last block at the bottom; Up at the top does not move.
- `TestViewerWheel`: a `pointer.Scroll` event with `Scroll.Y = 100` moves 100 px; with Ctrl
  held it does not scroll.
- `TestViewerCtrlWheelZoom`: Ctrl+wheel `-100` zooms in one level; `-4` twice does nothing,
  a third `-4` zooms in (accumulator); `+100` zooms out.
- `TestViewerZoomKeepsPosition`: zooming keeps `Position.First` and scales `Offset`.
- `TestViewerScrollbarDrag`: a press-drag on the scrollbar area moves the list (the wheel
  area's `PassOp` lets it through).
- `TestViewerLinks`: clicks on an `External` link call the fake opener with the URL; on an
  existing `LocalFile` with the path; on a missing file, a `Fragment` and a `Refused` link,
  no call and LINK-1's stderr line.
- `TestOpenerFailureReported`: a fake opener whose wait fails prints LINK-2's line.
- `TestViewerQuit`: `q` and Esc make `Frame` return true.
- `TestViewerRewraps`: the same frame at 800 then 300 px gives a taller first block.

**Verification:** the standard checks.

**Done when:** all listed tests pass and the standard checks pass.

---

## M10 The window and the binary

**Covers:** WIN-1, WIN-2, WIN-3, WIN-6, CLI-6, CLI-9, STYLE-PAL-2, SPEED-1, SPEED-2,
SPEED-6.

**Files:** `internal/window/window.go`; `internal/app/prepare.go` completed (fonts, images,
view, viewer, timings, `Prepared.Window`); `internal/app/timing.go`; `cmd/md-viewer/main.go`
in the shape of ARCHITECTURE.md (the `not implemented yet` line is removed); in
`.github/workflows/ci.yml`, after the `go build` step of the `build` job, add:

```yaml
      - name: Smoke run under Xvfb (information only; speed is judged on a real desktop)
        run: xvfb-run -a dist/md-viewer --time testdata/elements.md
```

**Tests first:**

- `TestTimingsLine`: `Timings{Read: 400µs, Parse: 12.34ms, Fonts: 6.1ms, FirstFrame: 88ms}`
  gives `read_ms=0.4 parse_ms=12.3 fonts_ms=6.1 first_frame_ms=88.0`.
- `TestPrepareTimings`: with a fake clock, `Prepare` records read, parse and fonts from the
  clock, and `OnFirst` in the window config prints the line to stdout only when `--time` is
  given.
- `TestPrepareTitleAndPalette`: the config's title is `elements.md - md-viewer`; `Canvas`
  is the light palette's, and the dark one with `--dark`.
- `TestNoInitFunctions`: parses every non-test `.go` file of the module with `go/parser` and
  fails if any declares `func init()` (SPEED-6).
- `TestBinaryNoDisplay` in `cmd/md-viewer/main_test.go`: run the built binary on
  `testdata/elements.md` with `DISPLAY` and `WAYLAND_DISPLAY` removed from its environment:
  exit 1 and stderr starting with `md-viewer: cannot open a window: `.
- `TestBinaryTimeUnderXvfb`: when `xvfb-run` is on `PATH`, run
  `xvfb-run -a <binary> --time testdata/elements.md`: exit 0 and stdout matching the smoke
  regular expression. When `xvfb-run` is not on `PATH` the test fails with a message saying
  the image lacks it (the container and CI both have it, so it must never skip).

**Verification:** the standard checks and the smoke run. Push and check that CI's `build` job
runs the smoke step green and uploads the artifact; download it with
`gh run download <id> -n md-viewer-linux-amd64` and run `./md-viewer --version` on it.

**Done when:** the standard checks and the smoke run pass in the container, and CI's `build`
job is green with the artifact uploaded.

---

## M11 Speed: fixtures, benchmarks, measurement

**Covers:** SPEED-3, SPEED-4, SPEED-7; DOC-14 on the large fixture.

**Files:** `testdata/large.md` (generated), `internal/doc/large_test.go`
(`TestLargeFixtureIsCurrent`, `BenchmarkParseLarge`), `internal/render/bench_test.go`
(`BenchmarkFirstViewportLarge`); in `.github/workflows/ci.yml`, after the smoke step:

```yaml
      - name: Benchmarks (information only)
        run: go test -run '^$' -bench . -benchtime 20x ./internal/...
```

and the README's "Speed" section's results table, with the container's numbers in a column
headed "container (Xvfb, information only)" and the desktop column left as "to be measured".

**Tests first:**

- `TestLargeFixtureIsCurrent`: `large.md` is exactly 100 copies of `elements.md` joined by
  one blank line; with `MDV_UPDATE_FIXTURES=1` it is rewritten.
- `TestNoSystemFonts`: a shaper from `NewShaper` lays out a CJK character as a single
  missing-glyph cluster, which proves no system font was consulted (STYLE-FONT-2, SPEED-4).
- `TestFirstViewportLaysOutFewBlocks`: laying out a 980 × 900 dp viewport of `large.md` calls
  `View.Layout` for fewer than 60 blocks (count with a wrapper around `View.Layout`), showing
  that only the visible blocks are laid out (SCROLL-1).

**Verification:** the standard checks, the smoke run with `testdata/large.md`, and
`go test -run '^$' -bench . -benchtime 20x ./internal/...`. Record the numbers in the README
as above.

**Done when:** the standard checks pass, the benchmarks run, the README holds the container
numbers, and the owner has been told the host measurement is theirs to run (README "Speed").

---

## Decisions for the owner

These were chosen so that work can start; each is a one-line change of spec if the owner
decides otherwise.

01. **Fonts.** (Awaiting the owner; built to the spec default; seam: `goFaceSources()` and
    `NewShaper`'s options in `internal/render/fonts.go`.) Only the embedded Go fonts are used
    (STYLE-FONT-1), for start-up speed. Emoji and symbols such as ✅ ⬜ 🔄, common in plan files,
    show as missing-glyph boxes. The alternatives are system fonts (a font scan at every start;
    its cost is measured in M11 if wanted) or embedding a monochrome emoji font (a larger binary).
02. **Dark mode.** (Awaiting the owner; built to `--dark` only; seam: `app.Prepare` in M10.)
    Gio v0.10.3 does not report the desktop's colour scheme, so it is `--dark` only. md-viewer
    could read the XDG desktop portal's `org.freedesktop.appearance` `color-scheme` setting over
    D-Bus (Gio already depends on `godbus`), at the cost of one D-Bus round trip at start.
03. **`--time` exits after the first frame**, so it can be scripted and benchmarked. The
    alternative is to print and keep the window open.
04. **HTML is dropped**, with one stderr line counting it. The alternative is to show HTML
    source as text.
05. **No 980 dp column cap**: text wraps to the window width, as asked. GitHub caps it.
06. **Code blocks and wide tables wrap or clip** instead of scrolling sideways.
07. **In-page `#` links are not followed.**
08. **Files over 64 MiB are refused.**
09. **The wheel scrolls by what Gio reports**: 100 px a notch on Wayland, 10 on X11.
10. **go-qa-ci is private**, so CI's `go-qa` job is red until it is public. The alternative is
    a read-only token as a repository secret. The plain Go toolchain gate does not depend on it.
11. **Installing md-viewer on the desktop** (a play, a `.desktop` file, the default for
    `text/markdown`) is fedora-desktop's Plan 00199 Phase 2, not part of this repository.
