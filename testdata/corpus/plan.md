# Plan: Example Feature

**Status**: In Progress
**Owner**: the team
**Created**: 2026-01-01

## Overview

This plan describes an example feature, written the way real project plans are written: a status block, goals, non-goals, phases with task lists, a table of decisions, and some code. It exists to be rendered next to GitHub's own rendering of the same file.

## Goals

- A thing opens quickly, from a launcher and from a terminal.
- It follows the system's light or dark scheme.
- It is installed by automation and set as the default handler.

## Non-Goals

- An editor. Editing stays elsewhere.
- Rendering remote content or running scripts from a document.

## Tasks

### Phase 1: Research

- [x] **Task 1.1**: Survey the existing tools
  - [x] List the candidates
  - [x] Measure start-up time
- [x] **Task 1.2**: Decide to build one
- [ ] **Task 1.3**: Write the specification
  - [ ] Number every expectation
  - [ ] Review with the owner

### Phase 2: Build

- [ ] **Task 2.1**: Parse the document
- [ ] **Task 2.2**: Lay it out
- [ ] **Task 2.3**: Draw it, `internal/render`, and test without a window

## Decisions

| ID | Decision | Status | Rationale |
| --- | --- | --- | --- |
| D1 | Use one window | Accepted | Simplicity |
| D2 | No settings | Revisit | The owner asked for dark mode |
| D3 | Gio for drawing | Accepted | Fast start, no web view |

## Technical Notes

```bash
go build ./cmd/md-viewer
./md-viewer --time docs/SPEC.md
```

> **Note**: timings under a virtual display are information only. The speed is judged on a real desktop.

## Success Criteria

1. Every numbered expectation has a test.
2. The gate passes: `gofmt -l .`, `go vet ./...`, `go test -race ./...`.
3. The owner confirms it feels fast.

---

*Last updated: 2026-01-01*
