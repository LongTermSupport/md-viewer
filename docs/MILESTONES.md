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
no display of its own, but it can reach a live Wayland compositor: a socket under the user's
`XDG_RUNTIME_DIR`, which Gio prefers to X11. The owner decides whether the container may reach
that compositor at all. Until then, every command that starts the binary, and every test that
does, hides it:

```bash
env -u WAYLAND_DISPLAY -u WAYLAND_SOCKET -u DISPLAY XDG_RUNTIME_DIR="$(mktemp -d)" ...
```

(`xvfb-run -a` sets `DISPLAY` itself, so it is unset here too). Without the empty runtime
directory, libwayland falls back to `$XDG_RUNTIME_DIR/wayland-0`, and the window opens on the
compositor. So:

- everything builds, including the window code and the binary;
- every unit test runs without a window, and the binary tests run the binary with the
  environment above: one with no display, which must fail, and one under `xvfb-run -a`;
- the binary can be run under a virtual display, `xvfb-run -a`, which proves it opens a window
  and draws a frame, but its timings are information only (SPEC section 9);
- the speed targets are measured by the owner on the desktop (M11).

The binary tests need `xvfb-run` on `PATH`; the container and CI have it, and a missing `xvfb-run`
fails the test rather than skipping it, so the owner's desktop needs the same package.

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
go build ./...    # non-test code compiles on its own (vet and test compile test files too)
go vet ./...
go test -race -count=1 ./...
```

Where `go-qa` is installed it adds to these (`go-qa run`); it is never required. From M10,
also the **smoke run**:

```bash
go build -trimpath -o dist/md-viewer ./cmd/md-viewer
env -u WAYLAND_DISPLAY -u WAYLAND_SOCKET -u DISPLAY XDG_RUNTIME_DIR="$(mktemp -d)" xvfb-run -a dist/md-viewer --time testdata/elements.md
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
- [x] M4 Inline text flow
- [x] M5 The view and simple blocks
- [x] M6 Lists
- [x] M7 Tables
- [x] M8 Images
- [x] M9 The viewer: scroll, zoom, links, quit
- [x] M10 The window and the binary
- [x] M11 Speed: fixtures, benchmarks, measurement
- [ ] M12 Hardening: the label gate on tracked files, non-blocking opens

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
- `qa/go-qa-ci.ref`: the single line `7fc4547e92f2fb0cf67c0c91f1f2068774be5d5a`, the commit
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
  `md-viewer: not implemented yet` to stderr and exits 1; M10 replaces that line (M2 keeps it).
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
      - run: go build ./...
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

The tests are in `trace_test.go` and `flow_test.go`, 23 in all: `TestTraceAdd`, then
`TestFlowLineHeight`, `TestFlowWrapsToWidth`, `TestFlowLongWord`, `TestFlowBreakRun`,
`TestFlowCodeBackground`, `TestFlowCodeInHeading`, `TestFlowStrike`, `TestFlowLinkArea`,
`TestFlowLinkClick`, `TestFlowLinkCursor`, `TestFlowInvalidUTF8`, `TestFlowUnknownRunKind`,
`TestFlowNilTrace`, `TestFlowEmptyRuns`, `TestFlowLeadingSpace`, `TestFlowTinyWidth`,
`TestFlowRunBoundary`, `TestFlowTrailingSpace`, `TestFlowStrikeCode`, `TestFlowLinkWrap`,
`TestFlowBidi` and `TestFlowZeroGlyphs`.

- `TestFlowWrapsToWidth`: a 60-word paragraph at 800 px and at 200 px: the second is taller,
  and at each width every `Glyphs` mark's `Rect.Max.X` is ≤ the width.
- `TestFlowLongWord`: a 300-character word at 200 px breaks across lines, none wider than 200.
  After `ab ` and the same word, the first line is exactly `ab ` (a word boundary breaks first,
  and the long word breaks only on a line of its own).
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
- `TestFlowTinyWidth`: at width 0 and 1, `ab` lays out as `a` then `b` on separate lines, with
  no error and the text `ab`.
- `TestFlowRunBoundary`: `ab` then a 30-character run at 100 px gives `ab`, a space, then 12, 12
  and 6 characters; `ab` and `cd` in separate runs at 20 px break between them.
- `TestFlowTrailingSpace`: `ab cd` at 30 px gives `ab ` then `cd`, with the text whole.
- `TestFlowStrikeCode`: a struck code run's `Strike` spans its `CodeBg`, padding included.
- `TestFlowLinkWrap`: a click on the second fragment of a wrapped link reports the link.
- `TestFlowBidi`: a Hebrew word and an Arabic word each span from x 0 to their width, and the
  text after them starts at that width.
- `TestFlowZeroGlyphs`: a fragment with no glyphs is an error.

**Verification:** the standard checks.

**Done when:** all listed tests pass and the standard checks pass.

---

## M5 The view and simple blocks

**Covers:** STYLE-TEXT-2 (layout), STYLE-CODE-1, STYLE-CODE-2, STYLE-QUOTE-1, STYLE-RULE-1,
DOC-LAYOUT-1, DOC-LAYOUT-2, DOC-LAYOUT-3 (applied). Not STYLE-PAL-3: the View paints no
canvas, and the window's fill is tested in M10.

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
- Also: `TestTraceShift` (trace origin), `TestCodeBlockStyle`, `TestFlowCodeMode` (code wrapping
  and trailing lines), `TestViewPlaceholders`, `TestViewLayoutError` (`Err`), `TestViewNilTrace`,
  `TestViewClicked`, `TestViewElements` (every block of `elements.md`), `TestViewTinyWidth`
  (no error at zero, negative and tiny widths).

**Verification:** the standard checks.

**Done when:** all listed tests pass and the standard checks pass.

---

## M6 Lists

**Covers:** STYLE-LIST-1, STYLE-LIST-2, STYLE-LIST-3, STYLE-LIST-4, STYLE-LIST-5.

**Files:** `internal/render/list.go` and `list_test.go`; `style.go` (the `Bullet` type) and
`blocks.go` (the list dispatch and the baselines of every text block); the M5 placeholder for
`doc.List` is removed.

**Tests first:**

- `TestBulletShape`: depths 0 to 3 and 9 give disc, circle, square, square, square; -1 and 0
  give disc; the shapes' texts are `disc`, `circle`, `square`.
- `TestLeadsWithText`: which first blocks start with a text line (an empty item counts), and
  the errors for a nil block and for a pointer block.
- `TestListIndent`: item glyphs start 32 px right of the list's left edge; a nested list's
  items 64 px.
- `TestBullets`: depths 0, 1, 2 give `Bullet` marks of the three shapes (record the shape in
  the mark's `Text`: `disc`, `circle`, `square`), 5 px, centred 11 px left of the content
  edge.
- `TestOrderedNumbers`: `Number` marks with texts `3.`, `4.` for `Start: 3`; nested `i.`,
  then `a.`; right edge 4 px left of the content edge.
- `TestTightAndLooseSpacing`: tight items 4 px apart; loose items 16 px apart; a tight
  item's paragraph and image have no margins.
- `TestTaskBoxes`: an `Open` item has a 13 px `TaskBox` mark `open` (`Canvas` fill) and a
  `border` mark (`FgMuted`, the same rect); a `Done` item has a `done` mark (`Accent`) and a
  `check` mark (`OnAccent`); the box's left edge is 22 px left of the content edge. A task item
  draws no bullet or number, and still counts in the numbering.
- `TestBlockBaseline`: every text block's `Baseline` is its first text line's distance from
  the bottom (heading, code block, quote, paragraph).
- `TestListMarkerBaseline`: a marker sits on its item's first text line, found through
  headings, quotes, nested lists and code blocks; with no text line it sits on an empty body
  line at the item's top.
- `TestListInQuote`: markers and numbers in a quote are `FgMuted`, task boxes keep their own
  colours, and a list in a quote in an item is nested (no margins).
- `TestListTinyWidth`: no error and one marker per item at widths from negative to 77 px.
- `TestViewElements` also counts one marker per list item; `TestViewClicked` has a list row
  (the link's area starts at the content edge). `TestViewPlaceholders` loses its list row.

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
  neighbours (a table of m columns and n body rows has m + 1 vertical and n + 2 horizontal
  lines).
- `TestTableHeaderBold` and `TestTableStripes`: header glyphs use the Bold face; body rows 2
  and 4 have a `RowFill` in `BgMuted`, rows 1 and 3 in `Canvas`, and the header has one in
  `Canvas` too.
- `TestTableAlignment`: in a wide column, `Right` text ends 13 px from the cell's right
  edge, `Centre` is centred, `None` body text starts 13 px from the left and `None` header
  text is centred.
- `TestTableRowHeight`: a row is as tall as its tallest cell; short cells' text is at the top.
- `TestTraceShiftFrom` (`trace_test.go`) and `TestFlowAlign` (`flow_test.go`): a flow's centred
  and end-aligned lines, with their marks moved, and an overflowing line not moved.
- `TestCellAlign`, `TestTableEmptyCells`, `TestTableTinyWidth`, `TestTableNested` and
  `TestTableError` (`table_test.go`): the alignment map, empty and short cells, widths from 0
  to 77 px at three metrics, tables in quotes and lists, and each malformed model's error.
- `TestListError` (`list_test.go`): a list's layout errors name the task state and the level
  (a pin of M6 behaviour).
- Changed rows: `TestBlockBaseline` gains a table row, `TestLeadsWithText` makes a table true,
  `TestListMarkerBaseline` gains a table row, `TestViewElements` gets a table case,
  `TestViewClicked` gains a table row, and `TestViewPlaceholders` loses its table row.

**Verification:** the standard checks.

**Done when:** all listed tests pass and the standard checks pass.

---

## M8 Images

**Covers:** STYLE-IMG-1 to STYLE-IMG-5, SPEED-5.

**Files:** `internal/render/image.go` and `image_test.go`; the fixtures of ARCHITECTURE.md
under `testdata/img/`, written by `TestImageFixtures`; the M5 placeholder for `doc.Image` is
removed. `NewImages` is called by `app.Prepare` (add it there with a test), and `NewView`
gains its `img *Images` parameter (M5 built `NewView(d, sh, p)`).

**Tests first:**

- `TestImageFixtures`: with `MDV_UPDATE_FIXTURES=1` writes the four files; without it, checks
  they decode to the stated sizes and colours.
- `TestImageNaturalSize`: `red-4x3.png` at zoom 1 gives an `ImageBox` 4 × 3 px; with
  `Metric{PxPerDp: 2}` 8 × 6.
- `TestImageScaledDown`: a 1000 × 500 image (generated in the test into `t.TempDir()`) at
  600 px content width is 600 × 300.
- `TestImagePlaceholders`: missing file, GIF, a file that is not an image (`broken.png`), a
  remote URL, a fragment, a directory, a truncated header and a too-large header each give a
  `Placeholder` mark with the alt text (or the destination when alt is empty), hugging its text
  (STYLE-IMG-3), and one stderr line of STYLE-IMG-3's form each.
- `TestImageSizeCap`: generated PNG headers (no pixel data) at the STYLE-IMG-1 limits: 8192 × 1
  and 1 × 8192 and 8000 × 5000 (40 million pixels) are drawn; 8193 × 1, 1 × 8193, 8000 × 5001 and
  8192 × 8192 are placeholders with `image too large: WxH`.
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
- `TestActionFor` also covers the modifier rules of SCROLL-2 and WIN-4 (Shift on arrows and
  Ctrl on Home are no action; Shift+Space is Page Up).
- `TestActionString`: each action's name, and an out-of-range value.
- `TestViewerKeysScroll` (router): after a frame, Down moves `Position.Offset` by 40 px at
  zoom 100 and 80 px at zoom 200; Page Down by the viewport height minus 40; Home sets `First`
  and `Offset` to 0 (layout fills `Count`, `Length` and `BeforeEnd`, so they are not compared);
  End shows the last block at the bottom; Up at the top does not move.
- `TestViewerEveryKeyRoutes`: each key of `TestActionFor` reaches the viewer through the router
  and has its effect (keys with the router's filters).
- `TestViewerZoomKeys`: Ctrl+`=`, Ctrl+Shift+`+`, Ctrl+`-` and Ctrl+`0` move the zoom through
  ZOOM-1's levels, and stop at 300 and at 50.
- `TestViewerZoomScalesContent`: a rule-only document's frame height at several window widths
  and zooms (DOC-LAYOUT-1 and STYLE-ZOOM-3).
- `TestViewerWheel`: a `pointer.Scroll` event with `Scroll.Y = 100` moves 100 px; with Ctrl
  held it does not scroll.
- `TestViewerCtrlWheelZoom`: Ctrl+wheel `-100` zooms in one level; `-4` twice does nothing,
  a third `-4` zooms in (accumulator); `+100` zooms out.
- `TestViewerZoomKeepsPosition`: zooming keeps `Position.First` and scales `Offset`.
- `TestViewerScrollbarDrag`: a press on the indicator, then two `pointer.Move` events with the
  primary button held (the first drag event only anchors), then a release, moves the list (the
  wheel area's `PassOp` lets it through). A press and release without a move does not.
- `TestViewerLinks`: a table of clicks: an `External` link calls the fake opener with the URL;
  an existing `LocalFile` (and a directory, and a file with a fragment) with its path; a missing
  file, a `Fragment` and a `Refused` link give no call and LINK-1's stderr line, byte for byte.
- `TestViewerLinkCursor`: the hand cursor over a link, and the default cursor over plain text.
- `TestViewerOpensOffFrame`: the opener runs only in the spawned function, after the frame.
- `TestOpenerFailureReported`: a fake opener whose wait fails prints LINK-2's line, and the next
  frame does not fail.
- `TestExecOpenerStartError`: a missing opener program gives an error that matches
  `fs.ErrNotExist`.
- `TestSystemOpenerCommand` (Linux): the command is `xdg-open` with the target as one argument,
  built without a shell (LINK-2). It does not start the command.
- `TestSystemOpenerOther` (not Linux, compiled by the cross-platform vet): an error naming the OS.
- `TestViewerQuit`: `q`, Shift+`q` and Esc make `Frame` return true; the modifier controls do not.
- `TestViewerNoEditing` (WIN-5): a control-only pin. Ctrl+C, Ctrl+A, Ctrl+V and text input do
  nothing: no clipboard request, no text input state, no scroll or zoom change.
- `TestViewerEmptyDocument`: a document with no blocks takes the keys and the wheel without
  an error or a panic.
- `TestViewerElements`: `testdata/elements.md` at 980 × 900 dp, with Home, End and a wheel
  scroll, lays out without an error.
- `TestViewerRewraps`: the same frame at 800 then 300 px gives a taller first block (WRAP-2).
- `TestViewerLayoutError`: a layout error ends a frame with that error (the window closes); a
  failed stderr write in a frame does too; an ordinary document gives no error.
- `TestViewerHomeEnd`: End puts the last block at the bottom, Down there does not move, and
  Home is the top.
- `TestViewerCtrlWheelKeepsFirst`: a Ctrl+wheel zoom keeps `First` and scales the offset.
- `TestViewerLinkControls`: a press on the link released away, a hover and no input open nothing.
- `TestZoomMetric`: `Metric` multiplies both scales, at 100, 200, 125 and 67%.
- `TestOpenerFailureNextFrame`: a failed stderr write from the opener's function fails the next
  frame, and the frame after it does not.
- `TestViewerGoSpawnWrites`: the real `goSpawn`, with a writer that is not safe for concurrent
  use, writes the opener's line and a frame's line; `-race` finds no conflict.
- `TestViewerLinkStatErrors` (Linux): a path under a regular file gives "cannot open link", and a
  broken symbolic link is not found.
- `TestExecOpenerWait`, with its helper `TestOpenerHelperProcess`: exit 3 is an error, exit 0 is
  nil.
- `TestFrameErrors`: a link write error and a layout error in one frame give one line.
- `TestViewerLocalLinksAbsolute`: every local link reaches the opener as an absolute path.

**Verification:** the standard checks.

**Done when:** all listed tests pass and the standard checks pass.

---

## M10 The window and the binary

**Covers:** WIN-1, WIN-2, WIN-3, WIN-6, CLI-6, CLI-9, STYLE-PAL-2, STYLE-PAL-3 (the window
fills `Canvas`; the view paints none), SPEED-1, SPEED-2, SPEED-6.

**Files:** `internal/window/window.go` (the loop, `Config`, `FrameError`, `Source`, `Loop`; no
cgo, no `gioui.org/app`) and `internal/window/gioapp/gioapp.go` (the only importer of
`gioui.org/app`: `Run` and `Main`), as ARCHITECTURE "internal/window and internal/window/gioapp"
says; `internal/app/prepare.go` completed (palette, fonts, images, view, viewer, timings,
`Prepared.Window`); `internal/app/timing.go`; `internal/app/viewer.go` gains `Bind` for a failed
async write; `cmd/md-viewer/main.go` in the shape of ARCHITECTURE.md (the `not implemented yet`
line is removed); in `.github/workflows/ci.yml`, after the `go build` step of the `build` job, add:

```yaml
      - name: Smoke run under Xvfb (must exit 0; speed is judged on a real desktop)
        run: env -u WAYLAND_DISPLAY -u WAYLAND_SOCKET -u DISPLAY XDG_RUNTIME_DIR="$(mktemp -d)" xvfb-run -a dist/md-viewer --time testdata/elements.md
```

**Tests first:**

- `TestTimingsLine`: `Timings{Read: 400µs, Parse: 12.34ms, Fonts: 6.1ms, FirstFrame: 88ms}`
  gives `read_ms=0.4 parse_ms=12.3 fonts_ms=6.1 first_frame_ms=88.0`.
- `TestPrepareTimings`: with a scripted clock that counts any reading past its script, the
  load takes three readings and the faces two, and `OnFirst` in the window config writes the
  line to stdout only when `--time` is given, and its write error is returned (CLI-6).
- `TestPrepareTitleAndPalette`: the config's title is `elements.md - md-viewer`; `Canvas`
  is the light palette's, and the dark one with `--dark`; the viewer's theme text colour follows
  the palette; `ExitAfterFirst` is set only with `--time`.
- `TestPrepareBindsInvalidate`: `Bind` reaches the images, so one headless frame of a local
  image makes the window invalidate when its pixels are ready (SPEED-5).
- `TestPrepareFontError`: a font error prints `md-viewer: ` and the error, and exits 1 with no
  window.
- `TestViewerAsyncErrorInvalidates` (`internal/app/viewer_test.go`): a failed write from the
  opener's function invalidates the window, and the next frame returns that write error (LINK-2).
- `TestWindowOptions` (`internal/window/gioapp`): the options set the title and the size and
  minimum size in dp (WIN-1, WIN-2).
- `TestBinaryNoDisplay` in `cmd/md-viewer/main_test.go`: run the built binary on
  `testdata/elements.md` with `DISPLAY`, `WAYLAND_DISPLAY` and `WAYLAND_SOCKET` removed from its
  environment and `XDG_RUNTIME_DIR` set to an empty temporary directory: exit 1, empty stdout, and
  stderr is the three lines printed before the window (DOC-9's and the two image lines, byte for
  byte), then one line starting `md-viewer: cannot open a window: ` (CLI-9). Run with `--time` too.
- `TestBinaryTimeUnderXvfb`: run `xvfb-run -a <binary> --time testdata/elements.md` (and with
  `--dark`) in the same environment, with `DISPLAY` removed, so Gio cannot reach a Wayland
  compositor: exit 0, stdout matching the smoke regular expression, and the `md-viewer: ` lines on
  stderr are exactly the three lines printed before the window. Other stderr lines, such as Mesa
  warnings, are ignored. When `xvfb-run` is not on `PATH` the test fails with a message saying
  the image lacks it. It never skips.
- `TestFrameErrorMapping` (`internal/window`, `cmd/md-viewer`): a `Config.Frame` error comes out
  of `Loop` as a `*FrameError` that unwraps to it, and `main` prints `md-viewer: ` and that
  error's text and exits 1 (CLI-6). Without a failed frame, `Loop` returns nil on close.
- `TestNoInitFunctions` (`cmd/md-viewer/init_test.go`): no non-test file of the module declares
  `func init()`, checked with `go/parser` over every non-test `.go` file, with a fixture row that
  must fail (SPEED-6).
- `TestGoStatements` (`cmd/md-viewer/init_test.go`): the `go` statements in non-test files are
  exactly the spawn of image decoding, the spawn of the opener's wait, and the window loop's
  goroutine in `main` (SPEED-6, with its fixture row).

The window tests (`internal/window/window_test.go`) drive `Loop` through a fake `Source`, with
no goroutines: a close, a frame error, the `--time` line, the canvas fill and the binding are
each checked in the order the loop calls them (`TestLoop*`). `gioapp` has one unit test of the
options it applies to `app.Config` (`TestWindowOptions`: WIN-1, WIN-2). `TestOneWindow` pins WIN-3:
the module creates one `app.Window`. WIN-6 has no test: it is Gio's default (the app ID is the base
name of the executable), and the owner checks it on the desktop.

**Verification:** the standard checks and the smoke run. Push and check that CI's `build` job
runs the smoke step green and uploads the artifact; download it with
`gh run download <id> -n md-viewer-linux-amd64`, which drops the executable bit, so run
`chmod +x md-viewer` first, then `./md-viewer --version`.

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
- `TestParseLargeDeterministic`: `large.md` parses to the same model twice (DOC-14) and has
  2600 top-level blocks, 26 per copy, which pins the copy count; the 99 blank-line joins
  (`The end.`, a blank line, then `# Heading level 1`) are counted separately, since the
  block count alone does not see a missing blank line.
- `TestNoSystemFonts`: a shaper from `NewShaper` lays out "日" and "☃" as one missing-glyph
  cluster each (glyph id 0; a control row "a" has a non-zero id). A subprocess, with `HOME`,
  `XDG_CACHE_HOME` and `LocalAppData` set to one empty temporary directory, shapes text, and
  the directory is still empty afterwards: no font cache file (STYLE-FONT-2, SPEED-4).
- `TestFirstViewportLaysOutFewBlocks`: a 980 × 900 dp window of `large.md`, laid out at the
  list's 970 dp width, calls `View.Layout` for at least one and fewer than 60 blocks (count with
  a wrapper around `View.Layout`), and the blocks cover the 900 px height (SCROLL-1).
- `TestNoPlannerLabels` (`cmd/md-viewer`): no Go file, README or doc holds a planner's label.

**Verification:** the standard checks, the smoke run with `testdata/large.md`, and
`go test -run '^$' -bench . -benchtime 20x ./internal/...`. Record the numbers in the README
as above.

The table's double layout (each cell is measured twice, and a whole flow is built for the
offset) was measured in M11 and left as it is: no SPEED target needs a change.

**Done when:** the standard checks pass, the benchmarks run, the README holds the container
numbers, and the owner has been told the host measurement is theirs to run (README "Speed").

---

## M12 Hardening: the label gate on tracked files, non-blocking opens

**Covers:** CLI-8, DOC-13 and STYLE-IMG-3 for a path replaced after its `Stat`; the label gate of
M11.

**Files:** `internal/doc/open.go` (`OpenRegular`, `ErrDirectory`, `ErrNotRegular`) and
`open_test.go`, `open_linux_test.go`; `internal/doc/load.go` and `load_linux_test.go`;
`internal/render/image.go` and `image_linux_test.go`; `cmd/md-viewer/init_test.go`.

**Tests first:**

- `TestTrackedFiles` (`cmd/md-viewer`): in a fixture repository, the list is the tracked files,
  without an untracked file or a tracked file deleted from the work tree; a directory that is not a
  git checkout, or a checkout that does not track `go.mod`, is an error that names `git ls-files`.
- `TestNoPlannerLabels`: its fixture is a git repository, and an untracked file holding a label is
  not reported. A missing `git`, or a tree that is not a checkout, fails the test; it never skips.
- `TestOpenRegular`, `TestOpenFlags`: a regular file opens with its size; a missing file gives the
  operating system's error; a directory gives `ErrDirectory`; the flags hold `O_NONBLOCK` and
  `O_NOCTTY`.
- `TestOpenRegularNonRegular` (Linux): `OpenRegular` refuses a FIFO with no writer, `/dev/zero` and a
  socket with `not a regular file` before any open, and a symbolic link to a regular file opens.
- `TestOpenStatNonRegular`, `TestOpenStatRegular` (Linux): the open-and-check step, given a path
  that is not a regular file when it is opened (a FIFO with and without a writer, a symlink to a
  FIFO, `/dev/null`, `/dev/zero`, a socket, a directory), returns no file without blocking, and a
  byte in the FIFO stays unread; a regular file and a symlink to it open and read in full.
- `TestFileOpens` (`cmd/md-viewer`): the only call in non-test code that opens a file is the
  `os.OpenFile` in `internal/doc/open.go`.
- `TestLoadNonRegular`, `TestImageNonRegular` (Linux): `Load`, the probe and the decode refuse a
  FIFO with no writer and a socket with `not a regular file`; a symlink to a regular file loads, and
  a symlink to a PNG is drawn.

A test that could block (a FIFO with no writer) runs the call in a goroutine, with a one-minute
bound that only a failure reaches; on the bound, the test opens the FIFO for reading and writing,
which releases the call, waits for it, and fails. No test sleeps.

**Verification:** the standard checks, the darwin and windows vets of M10, the smoke run, and:

```bash
go test -race -count=5 -run 'Open|NonRegular|FileOpens|TrackedFiles|PlannerLabels' ./cmd/md-viewer ./internal/doc ./internal/render
```

**Done when:** the standard checks and the smoke run pass, an untracked file holding a label does
not fail `go test`, and no open in md-viewer can block on a FIFO.

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
12. **SPEED-6 carve-out, the new indirect modules, and the SPEED-1 reading.** (Awaiting the
    owner; built to the suggested reading.) M10 reworded SPEED-6 to leave Gio's own platform work
    to Gio, which on X11 includes its IBus connection over D-Bus. md-viewer's code starts only the
    window loop, the image-decoding spawn and the opener's wait (`TestGoStatements` pins them),
    and does no D-Bus itself. Importing `gioui.org/app` adds four indirect modules:
    `gioui.org/shader`, `github.com/godbus/dbus/v5`, `golang.org/x/net` and `golang.org/x/sys`.
    ARCHITECTURE lists them. SPEED-1 is read as allowing the view, the viewer's state and the
    theme to be built before the window, since that is in-memory work; no other I/O or parsing
    happens first. The alternative is to build them after the window opens. The SPEC wording is
    unchanged: the owner accepts it or asks for a change.
