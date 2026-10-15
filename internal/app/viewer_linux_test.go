//go:build linux

package app

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// TestViewerLinkStatErrors checks LINK-1 for the two stat outcomes that are not "exists": a path
// under a regular file fails with the operating system's reason (ENOTDIR, which is not "not
// found"), and a broken symbolic link is not found, because a stat follows links. Nothing is
// opened in either case. The reason text is Linux's, so this file is Linux only.
func TestViewerLinkStatErrors(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "missing.md"), filepath.Join(dir, "broken.md")); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name   string
		src    string
		stderr string
	}{
		{
			name:   "a path under a regular file is a stat error",
			src:    "[x](file.txt/x)",
			stderr: "md-viewer: cannot open link: " + filepath.Join(dir, "file.txt", "x") + ": not a directory\n",
		},
		{
			name:   "a broken symbolic link is not found",
			src:    "[b](broken.md)",
			stderr: "md-viewer: link target not found: " + filepath.Join(dir, "broken.md") + "\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := parse(t, tt.src+"\n", dir)
			var stderr bytes.Buffer
			e := newEnv(t, d, defaultSize, &stderr)
			e.frame()
			e.click(linkCentre(t, d, e.images, defaultSize.X-10))
			if _, err := e.step(); err != nil {
				t.Fatalf("click frame returned %v", err)
			}
			e.runSpawned()
			if len(e.opener.calls) != 0 {
				t.Errorf("opener was called with %q, want no call", e.opener.calls)
			}
			if got := stderr.String(); got != tt.stderr {
				t.Errorf("stderr is %q, want %q", got, tt.stderr)
			}
		})
	}
}
