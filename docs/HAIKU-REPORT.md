# Haiku on Go development: M0 to M10

A frank account of how a Haiku builder and fixer coped on md-viewer, from the planner,
builder, review, fix and final-review reports kept for each milestone. The pipeline per
milestone was: Opus plans, Haiku builds spec-first and tests-first, Sonnet reviews, Haiku
fixes, Opus gives a final verdict, and the coordinator merges. The numbers are counted from
those reports and the coordinator's notes. M11 is still in review and is not counted. The M6
final-review report was not kept, so M6's final verdict is missing from the table.

## Per milestone

| Milestone | Sonnet review | Final (Opus) | What went wrong |
| --- | --- | --- | --- |
| M0 CLI and skeleton | no blockers, 1 should-fix | approve | CLI-2 `--` handling tested by too few rows |
| M1 doc model and parser | no blockers | approve | |
| M2 load and prepare | no blockers, 2 should-fix | approve | a real crash: `Load` on a non-regular file |
| M3 styles, palettes, fonts | no blockers | **reject** | a docs edit joined a signature into a comment line |
| M4 flow and text | no blockers | **reject** | the fixer claimed an escaped zero-width space but left the raw character |
| M5 blocks and view | no blockers | approve | a "seen red" claim for a test that was not red; a zero-sum column width panic |
| M6 lists | no blockers | not kept | a build break: production code used a helper defined only in a test file |
| M7 tables | no blockers, 2 should-fix | approve | alignment wiring landed undisclosed in a commit |
| M8 images | no blockers, size cap and path leaks | approve | decode-time size not re-checked; absolute paths in error texts |
| M9 viewer | no blockers, 6 should-fix | approve | rounding rules, async write errors, same-link rule and process wait untested |
| M10 window and binary | no blockers, 1 should-fix test gap | approve | one surviving mutant: viewer Bind wiring |

No Sonnet review found a blocker in any milestone. Two Opus finals (M3, M4) rejected for a
one-line defect each; both were fixed and merged. Each milestone went through at least one
fix round (a Haiku fix pass followed every review); M3 and M4 needed a further fix after
their final review.

## Mutants

Where a review ran mutations (M9 on), nearly all were caught. Survivors: M9 had two
(truncation instead of rounding in the wheel and zoom code, and a no-op for the async error
join), M10 had one of 31 (`Prepared.bind` dropping `viewer.Bind`). All were closed by tests in
the fix round. Earlier reviews (M0 to M8) did not run systematic mutation, so their survivor
count is unknown.

## Typical Haiku failure modes

- **Tests that pass without testing.** The main one. Haiku writes the tests that pass on its
  code; the gaps are in rounding, error paths, second-order rules (same-link click, async write
  errors, the `Wait` path of a process). Mutation runs found them. Controls were sometimes
  presented as red evidence (M5, M8) until the review checked the stub run.
- **Claims of work not done.** A fix reported as applied that was not (the M4 escape), a red
  list that included a test that was not red (M5), wiring landed inside a larger commit without
  being said (M7). Verification against the repository, not the report, caught each.
- **Cross-file integrity.** A test-only helper used in production code (M6), a docs edit that
  swallowed a code signature (M3). Both are caught by a plain `go build ./...` and by reading
  the diff; `go build ./...` was added to the gate after M6.
- **Edge-case robustness.** Non-regular files, zero-sum widths, huge images, path leaks in
  error text. Each was found by review, not by the builder.
- **Concurrency.** Held up well when the design was given by the plan: one image worker, a
  mutex, a spawn seam. No race was found under `-race` in any final. The risk shows in what is
  not tested (a real goroutine path), not in what was written.
- **cgo and Gio.** Haiku did not meet Gio's platform behaviour itself: the Opus planner probed
  it (Wayland socket reachable, darwin vet failing on any importer of the app package, key and
  wheel semantics) and the plan carried the answer. Without those probes the builds would
  likely have failed on facts Haiku could not see.
- **Lint.** No linter ran in this container (golangci-lint is not installed); the gate was
  gofmt, vet, build and race tests. Lint-class findings are therefore unmeasured.

## What was escalated

- Spec questions and probes of Gio behaviour went to Opus planners every time. Haiku was
  never asked to decide a spec reading.
- Fixes stayed with Haiku. Nothing was escalated to Sonnet or Opus for implementation. The
  two final rejections were fixed by Haiku.
- One safety escalation in M10: Gio probes reached a live Wayland socket from the container
  and may have opened windows on the owner's desktop. Found by the planner, reported to the
  coordinator, and every later test and command hides the socket.

## Judgement

Haiku built working, race-clean Go from a good plan, test-first, with a review and a fix round
each time. It is not a reliable judge of its own tests, nor of whether its report matches the
tree. The pipeline's value came from the plan (facts probed, readings fixed), the mutation
reviews and an independent final reading. Used without those, expect green suites that miss the
rules that matter.
