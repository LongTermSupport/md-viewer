# md-viewer: project instructions

md-viewer is a Markdown viewer in Go with Gio: `md-viewer FILE.md` opens one window showing
one file rendered in GitHub's style. Linux first, with seams left for other platforms. Start-up
and rendering must be as fast as possible, and that is measured, not guessed.

## The "no" list

No editing, no syntax highlighting, no tabs, no second window or file, no live reload, no
search, no selection or copy, no HTML rendering, no network, no settings. Anything not in
[docs/SPEC.md](docs/SPEC.md) is out of scope; its section 10 lists the non-goals. Do not add a
feature, a flag or a dependency the spec does not name: propose it to the owner instead.

## Where things are

- [docs/SPEC.md](docs/SPEC.md): the numbered, testable expectations. The contract.
- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md): packages, public types, pinned versions, how
  render is tested without a window.
- [docs/MILESTONES.md](docs/MILESTONES.md): the build order, the tests to write first, the
  verification commands, and the status list. Work one milestone at a time, in order.
- [README.md](README.md): what users read.

## Method

- **Spec first.** Behaviour is written in SPEC.md before code. If the spec is wrong or
  unclear, stop and say so; do not code around it.
- **Tests first.** Every expectation gets a test, seen to fail on its assertion before the code
  that makes it pass is written.
- **Fail fast.** Every error is returned or reported; nothing is skipped, ignored or retried
  silently. Only `main` exits.
- **Strict typing.** Named types for every kind, style, exit code and target; exhaustive
  switches.
- **DRY.** Each spec value is one named constant, citing its expectation number.
- **Diagnostics to stderr.** stdout carries only `--help`, `--version` and `--time` output.
- **No suppressions.** No `//nolint`, no go-qa record entries without the owner.

## QA

QA is go-qa-ci, pinned to the commit in `qa/go-qa-ci.ref`. Install its tools with
`scripts/install-qa-tools.sh`, run the gate with `scripts/qa.sh`. Also `go vet ./...` and
`go test -race -count=1 ./...`. All must pass before every commit is pushed. go-qa-ci is
private, so CI's `go-qa` job fails until it is published; its `build` job must be green.

This container has Gio's headers and Xvfb but no display: the binary builds and runs under
`xvfb-run -a`, and its timings there are information only. Speed targets are judged on the
owner's desktop.

The container's tools come from `.claude/ccy/Dockerfile`. A missing tool is fixed there and
the image rebuilt by the owner; never install one by hand.

## First session in this project's own container

The hooks daemon is not installed yet. In md-viewer's own ccy session, run the skill
`/hooks-daemon install`, check its health, and commit the configuration it writes
(`.claude/hooks-daemon.yaml` and the files it lists as tracked). An agent working from another
project must not install it here. Then delete this section in the same commit.

## Public repository

This repository is public. Never commit secrets, tokens, personal names, hostnames, local
paths or anything that identifies a particular machine. Use placeholders.
