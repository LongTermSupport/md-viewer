package doc

import (
	"bytes"
	"os"
	"testing"
)

// largeCopies is how many copies of elements.md large.md holds, joined by one blank line
// (ARCHITECTURE "Fixtures").
const largeCopies = 100

// largePath is the benchmark and --time fixture, relative to this package's directory.
const largePath = "../../testdata/large.md"

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
