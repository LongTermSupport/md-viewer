package doc

import "testing"

// TestResolveLink is the DOC-5 table, resolved against the document directory /doc. Each
// expected Target is written out, never computed by ResolveLink.
func TestResolveLink(t *testing.T) {
	cases := []struct {
		name string
		dest string
		want Target
	}{
		{"http", "http://e.org/x", Target{External, "http://e.org/x", "http://e.org/x"}},
		{"https", "https://e.org/", Target{External, "https://e.org/", "https://e.org/"}},
		{"mailto", "mailto:a@example.com", Target{External, "mailto:a@example.com", "mailto:a@example.com"}},
		{"scheme case", "HTTPS://X", Target{External, "HTTPS://X", "HTTPS://X"}},
		{"file URL", "file:///tmp/x.md", Target{LocalFile, "/tmp/x.md", "file:///tmp/x.md"}},
		{"file URL cleaned", "file:///tmp/a/../x.md", Target{LocalFile, "/tmp/x.md", "file:///tmp/a/../x.md"}},
		{"file URL with host", "file://host/x", Target{Refused, "file://host/x", "file://host/x"}},
		{"opaque file URL", "file:x.md", Target{Refused, "file:x.md", "file:x.md"}},
		{"relative file", "other.md", Target{LocalFile, "/doc/other.md", "other.md"}},
		{"fragment dropped", "other.md#part", Target{LocalFile, "/doc/other.md", "other.md#part"}},
		{"query dropped", "other.md?q=1#p", Target{LocalFile, "/doc/other.md", "other.md?q=1#p"}},
		{"parent", "../up.md", Target{LocalFile, "/up.md", "../up.md"}},
		{"dot segment", "sub/./c.md", Target{LocalFile, "/doc/sub/c.md", "sub/./c.md"}},
		{"absolute", "/abs/x.md", Target{LocalFile, "/abs/x.md", "/abs/x.md"}},
		{"absolute cleaned", "/abs/../x.md", Target{LocalFile, "/x.md", "/abs/../x.md"}},
		{"percent escape", "a%20b.md", Target{LocalFile, "/doc/a b.md", "a%20b.md"}},
		{"space", "a b.md", Target{LocalFile, "/doc/a b.md", "a b.md"}},
		{"backslash escape resolved", "a\\_b.md", Target{LocalFile, "/doc/a_b.md", "a_b.md"}},
		{"empty destination", "", Target{LocalFile, "/doc", ""}},
		{"scheme-relative with host", "//e.org/x", Target{Refused, "//e.org/x", "//e.org/x"}},
		{"fragment", "#top", Target{Fragment, "#top", "#top"}},
		{"javascript", "javascript:alert(1)", Target{Refused, "javascript:alert(1)", "javascript:alert(1)"}},
		{"ftp", "ftp://e.org/f", Target{Refused, "ftp://e.org/f", "ftp://e.org/f"}},
		{"bad escape", "%zz", Target{Refused, "%zz", "%zz"}},
		{"bad host", "http://[::1", Target{Refused, "http://[::1", "http://[::1"}},
		// The entity is resolved before the scheme is read, so it cannot hide javascript (P6).
		{"entity-built scheme", "javascript&colon;alert(1)", Target{Refused, "javascript:alert(1)", "javascript:alert(1)"}},
		{"file URL with localhost", "file://localhost/x", Target{LocalFile, "/x", "file://localhost/x"}},
		// No path part: the document's directory, as an empty destination (DOC-5, P8).
		{"query alone", "?q", Target{LocalFile, "/doc", "?q"}},
		{"file URL percent escape", "file:///x%20y", Target{LocalFile, "/x y", "file:///x%20y"}},
		{"Windows path", "C:\\x.md", Target{Refused, "C:\\x.md", "C:\\x.md"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ResolveLink("/doc", c.dest); got != c.want {
				t.Errorf("ResolveLink(/doc, %q) = %#v, want %#v", c.dest, got, c.want)
			}
		})
	}
}
