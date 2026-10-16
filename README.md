# md-viewer

A fast, lean Markdown viewer for the Linux desktop, written in Go with
[Gio](https://gioui.org). `md-viewer FILE.md` opens one window that shows one file rendered in
GitHub's style, and nothing else. It starts as fast as it can: the window shows the document
in its first frame.

Status: specified, being built. The specification is [docs/SPEC.md](docs/SPEC.md), the design
[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md), and the build order
[docs/MILESTONES.md](docs/MILESTONES.md).

## What it does

- Renders CommonMark with GitHub's extensions: headings, paragraphs, emphasis, strong,
  strikethrough, inline and block code, block quotes, ordered, unordered, nested and task
  lists, tables with alignment, rules, links, autolinks, and local PNG and JPEG images.
- Looks like GitHub: sizes, spacing and colours come from
  [github-markdown-css](https://github.com/sindresorhus/github-markdown-css) v5.9.0, in light
  or dark. Dark follows the desktop's colour scheme by default (`--theme auto`).
- Two settings, theme and body font size, from flags or a configuration file.
- Scrolls with the wheel, the arrows, Page Up/Down, Space, Home and End.
- Zooms with Ctrl and `+`, `-`, `0`, or Ctrl and the wheel, from 50% to 300%.
- Wraps text to the window's width.
- Opens a clicked link with your desktop's default application (`xdg-open`).
- Quits with `q` or Esc.

## What it does not do

No editing. No syntax highlighting. No tabs, no second window, no second file. No live reload,
no search, no table of contents, no text selection or copy. No HTML (it is left out), no
remote images, no network at all. Two settings, theme and font size, and nothing else. The full
list is SPEC section 10.

## Install

md-viewer needs Go 1.26.9 or newer, a C compiler, and Gio's build dependencies
([Gio's Linux install notes](https://gioui.org/doc/install/linux)).

Fedora:

```bash
sudo dnf install gcc pkg-config wayland-devel libX11-devel libxkbcommon-x11-devel \
    mesa-libGLES-devel mesa-libEGL-devel libXcursor-devel vulkan-headers
```

Debian and Ubuntu:

```bash
sudo apt install gcc pkg-config libwayland-dev libx11-dev libx11-xcb-dev \
    libxkbcommon-x11-dev libgles-dev libegl-dev libffi-dev libxcursor-dev libvulkan-dev
```

Then:

```bash
go install github.com/LongTermSupport/md-viewer/cmd/md-viewer@latest
```

The binary lands in `$(go env GOPATH)/bin`. Each CI run on `main` also uploads a Linux amd64
binary as the artifact `md-viewer-linux-amd64`.

## Use

```text
md-viewer [--theme light|dark|auto] [--font-size N] [--time] FILE
md-viewer --help | --version
```

| Flag              | Effect                                                              |
| ----------------- | ------------------------------------------------------------------- |
| `--theme MODE`    | `light`, `dark`, or `auto` to follow the desktop (default `auto`)   |
| `--dark`          | the same as `--theme dark`                                          |
| `--font-size N`   | body text size, 10 to 32 (default 16); headings and code scale too  |
| `--time`          | print start-up timings after the first frame, then exit             |
| `--help`          | show the usage                                                      |
| `--version`       | show the version                                                    |

Settings can also go in `$XDG_CONFIG_HOME/md-viewer/config` (or `~/.config/md-viewer/config`),
one `key = value` per line. A flag overrides the file:

```text
theme = dark
font-size = 18
```

A malformed file stops md-viewer with an error and exit code 1.

Exit codes: 0 success, 1 the file or the configuration file cannot be shown or no window opened,
2 a usage error.

## Speed

Start-up is measured, not guessed. `md-viewer --time FILE` prints one line after the first
frame and exits:

```text
read_ms=0.4 parse_ms=12.3 fonts_ms=6.1 first_frame_ms=88.0
```

`first_frame_ms` runs from the first line of `main` to the first frame. The wall clock,
including starting the process, is measured with
[hyperfine](https://github.com/sharkdp/hyperfine):

```bash
hyperfine --warmup 3 --runs 20 'md-viewer --time testdata/large.md'
```

The parse, layout and font steps have Go benchmarks:

```bash
go test -run '^$' -bench . -benchtime 20x ./internal/...
```

Targets, on a desktop under GNOME on Wayland (SPEC SPEED-3):

| Measure                                  | Target     | Desktop        | Container (Xvfb, information only) |
| ---------------------------------------- | ---------- | -------------- | ---------------------------------- |
| `first_frame_ms`, `testdata/elements.md` | ≤ 120 ms   | to be measured | 107.5 ms (median of 10)            |
| `first_frame_ms`, `testdata/large.md`    | ≤ 150 ms   | to be measured | 121.2 ms (median of 10)            |
| wall clock, `--time testdata/large.md`   | ≤ 250 ms   | to be measured | not measured (see below)           |
| `BenchmarkParseLarge`                    | ≤ 25 ms/op | to be measured | 17.0 ms/op (median of 10)          |
| `BenchmarkFirstViewportLarge`            | ≤ 15 ms/op | to be measured | 5.3 ms/op (median of 10)           |
| `BenchmarkLoadFonts`                     | ≤ 10 ms/op | to be measured | 4.2 ms/op (median of 10)           |

The container column is information only: a virtual display is not a desktop. Wall clock is not
measured there: hyperfine is not in the development container, and Xvfb's own start-up would
dominate the figure.

To fill the desktop column (SPEC SPEED-3 takes the median of each row):

1. `go build -trimpath -o dist/md-viewer ./cmd/md-viewer`
2. First frame: ten `--time` runs per file, the median of their `first_frame_ms` (stderr to
   `/dev/null`, as `large.md` prints a line per image):
   `for i in $(seq 10); do dist/md-viewer --time testdata/large.md 2>/dev/null; done | awk -F'first_frame_ms=' '{print $2}' | sort -n | awk '{a[NR]=$1} END {print (a[5]+a[6])/2}'`
3. Wall clock: `hyperfine --warmup 3 --runs 20 --export-json wall.json 'dist/md-viewer --time testdata/large.md'`,
   and the median in `wall.json` (`results[0].median`, in seconds).
4. Benchmarks: `go test -run '^$' -bench . -benchtime 20x -count 10 ./internal/...`, and the median
   of the ten ns/op values of each one, divided by 1e6 for ms/op.
5. Compare each with its target. A miss is reported with its number.

How it stays fast: it parses before the window opens and does nothing else then; it uses
embedded fonts and never scans the system's; it lays out only the blocks in view; it decodes
an image's pixels only when the image scrolls into view, off the drawing path.

## Development

The gate is the plain Go toolchain:

```bash
gofmt -l .        # must print nothing
go build ./...
go vet ./...
go test -race -count=1 ./...
```

[go-qa-ci](https://github.com/LongTermSupport/go-qa-ci), pinned in `qa/go-qa-ci.ref`, adds
more checks in CI. It is private for now, so CI's `go-qa` job fails at its install step until
it is published.

## Licence

MIT, see [LICENSE](LICENSE). The styling values are taken from github-markdown-css (MIT).
