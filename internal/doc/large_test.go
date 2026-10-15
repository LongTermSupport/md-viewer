package doc

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// largeCopies is how many copies of elements.md large.md holds, joined by one blank line
// (ARCHITECTURE "Fixtures").
const largeCopies = 100

// testdataDir is the shared fixtures directory, relative to this package's directory.
const testdataDir = "../../testdata"

// largePath is the benchmark and --time fixture, relative to this package's directory.
const largePath = testdataDir + "/large.md"

// updateFixturesEnv names the variable that makes TestLargeFixtureIsCurrent rewrite large.md.
const updateFixturesEnv = "MDV_UPDATE_FIXTURES"

// largeFixture is what large.md must hold: largeCopies copies of elements.md joined by "\n". As
// elements.md ends in exactly one newline, each join is one blank line.
func largeFixture(t *testing.T) []byte {
	t.Helper()
	elements, err := os.ReadFile(elementsPath)
	if err != nil {
		t.Fatalf("read %s: %v", elementsPath, err)
	}
	if !bytes.HasSuffix(elements, []byte("\n")) || bytes.HasSuffix(elements, []byte("\n\n")) {
		t.Fatalf("%s must end with exactly one newline", elementsPath)
	}
	copies := make([][]byte, largeCopies)
	for i := range copies {
		copies[i] = elements
	}
	return bytes.Join(copies, []byte("\n"))
}

// firstDiff is the offset of the first byte where a and b differ, or the shorter length when one
// is a prefix of the other.
func firstDiff(a, b []byte) int {
	n := min(len(a), len(b))
	for i := range n {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}

// TestLargeFixtureIsCurrent: large.md is exactly largeCopies copies of elements.md joined by one
// blank line (SPEED-7 and ARCHITECTURE "Fixtures"). With MDV_UPDATE_FIXTURES=1 the file is written
// first. A mismatch reports the lengths and the first differing offset, never the content.
func TestLargeFixtureIsCurrent(t *testing.T) {
	want := largeFixture(t)
	if os.Getenv(updateFixturesEnv) == "1" {
		if err := os.WriteFile(largePath, want, 0o644); err != nil {
			t.Fatalf("write %s: %v", largePath, err)
		}
	}
	got, err := os.ReadFile(largePath)
	if err != nil {
		t.Fatalf("read %s: %v (generate it with %s=1 go test -run TestLargeFixtureIsCurrent ./internal/doc)",
			largePath, err, updateFixturesEnv)
	}
	if bytes.Equal(got, want) {
		return
	}
	t.Fatalf("%s is not %d copies of elements.md: %d bytes, want %d, first difference at byte %d (regenerate with %s=1)",
		largePath, largeCopies, len(got), len(want), firstDiff(got, want), updateFixturesEnv)
}

// readLarge reads large.md, failing the test on error.
func readLarge(t testing.TB) []byte {
	t.Helper()
	src, err := os.ReadFile(largePath)
	if err != nil {
		t.Fatalf("read %s: %v", largePath, err)
	}
	return src
}

// TestParseLargeDeterministic: large.md parses to the same model twice (DOC-14), and to 2600
// top-level blocks, 26 per copy, which pins the copy count. The count does not pin the joins;
// the join count below does.
func TestParseLargeDeterministic(t *testing.T) {
	src := readLarge(t)
	checkDeterministic(t, string(src))
	dir, err := filepath.Abs(testdataDir)
	if err != nil {
		t.Fatalf("Abs of the testdata directory: %v", err)
	}
	d, err := Parse(src, dir)
	if err != nil {
		t.Fatalf("Parse(large.md): %v", err)
	}
	if n := len(d.Blocks); n != 2600 {
		t.Errorf("large.md has %d top-level blocks, want 2600 (26 per copy)", n)
	}
	// The count alone does not pin the join: a heading interrupts a paragraph, so a copy that
	// follows "The end." without a blank line still parses to the same blocks.
	if n := bytes.Count(src, []byte("The end.\n\n# Heading level 1")); n != largeCopies-1 {
		t.Errorf("large.md has %d blank-line joins between copies, want %d", n, largeCopies-1)
	}
}

// sinkDocument keeps BenchmarkParseLarge's result alive, so the parse is not optimised away.
var sinkDocument *Document

// BenchmarkParseLarge measures Parse of large.md's bytes, read once outside the loop (SPEED-3).
// Its numbers are reported, never asserted.
func BenchmarkParseLarge(b *testing.B) {
	src := readLarge(b)
	dir, err := filepath.Abs(testdataDir)
	if err != nil {
		b.Fatalf("Abs of the testdata directory: %v", err)
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(src)))
	for b.Loop() {
		d, err := Parse(src, dir)
		if err != nil {
			b.Fatal(err)
		}
		sinkDocument = d
	}
}
