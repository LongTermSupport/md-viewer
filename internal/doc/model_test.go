package doc

import "testing"

// Compile-time checks: every block type satisfies Block (the seal is the unexported method).
var (
	_ Block = Heading{}
	_ Block = Paragraph{}
	_ Block = CodeBlock{}
	_ Block = Quote{}
	_ Block = Rule{}
	_ Block = List{}
	_ Block = Table{}
	_ Block = Image{}
)

// DOC-3: nested markup combines style flags, so the flags must be non-zero and disjoint.
func TestStyleFlagsDisjoint(t *testing.T) {
	flags := []struct {
		name string
		s    Style
	}{
		{"Emphasis", Emphasis},
		{"Strong", Strong},
		{"Strike", Strike},
		{"Code", Code},
	}
	for i, a := range flags {
		if a.s == 0 {
			t.Errorf("%s is 0, want a non-zero flag", a.name)
		}
		for j, b := range flags {
			if i != j && a.s&b.s != 0 {
				t.Errorf("%s and %s share a bit: %d and %d", a.name, b.name, a.s, b.s)
			}
		}
	}
}
