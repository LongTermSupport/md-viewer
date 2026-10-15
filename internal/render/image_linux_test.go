//go:build linux

package render

import (
	"image"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// runBounded runs call, which probes or decodes fifo, in a goroutine and waits for it. Only a call
// that has not returned after a minute reaches the bound, and that is a failure: the test then opens
// fifo for reading and writing, which does not block on Linux and releases the blocked open, waits
// for call to return, and fails. The results call sets are read after it returns, so there is no
// race. If the release fails, the call stays blocked until the test binary exits.
func runBounded(t *testing.T, fifo string, call func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		call()
	}()
	select {
	case <-done:
		return
	case <-time.After(time.Minute):
	}
	release, err := os.OpenFile(fifo, os.O_RDWR, 0)
	if err != nil {
		t.Fatalf("open %s to release the blocked call: %v", fifo, err)
	}
	<-done
	if err := release.Close(); err != nil {
		t.Errorf("close %s: %v", fifo, err)
	}
	t.Fatalf("a call blocked on the FIFO %s with no writer", fifo)
}

// TestImageNonRegular (CLI-8, STYLE-IMG-3): the probe and the decode refuse a FIFO with no writer,
// a symbolic link to one, a socket and /dev/zero with the REASON "not a regular file", and read
// none of them. These rows pass on the first Stat, which guards against its removal. A symbolic link
// to a PNG is followed and drawn.
func TestImageNonRegular(t *testing.T) {
	const reason = "not a regular file"
	dir := t.TempDir()
	fifo := filepath.Join(dir, "fifo.png")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatalf("mkfifo %s: %v", fifo, err)
	}
	fifoLink := filepath.Join(dir, "fifo-link.png")
	if err := os.Symlink(fifo, fifoLink); err != nil {
		t.Fatalf("symlink %s: %v", fifoLink, err)
	}
	sock := filepath.Join(dir, "sock.png")
	if err := syscall.Mknod(sock, syscall.S_IFSOCK|0o600, 0); err != nil {
		t.Fatalf("mknod socket %s: %v", sock, err)
	}
	png := filepath.Join(fixtureDir(t), "red-4x3.png")
	pngLink := filepath.Join(dir, "red-link.png")
	if err := os.Symlink(png, pngLink); err != nil {
		t.Fatalf("symlink %s: %v", pngLink, err)
	}

	for _, p := range []string{fifo, fifoLink} {
		t.Run("fifo without a writer "+filepath.Base(p), func(t *testing.T) {
			var pt image.Point
			var why string
			runBounded(t, p, func() { pt, why = probeImage(p) })
			if pt != (image.Point{}) || why != reason {
				t.Errorf("probeImage = %v, %q, want no size and %q", pt, why, reason)
			}
			var img image.Image
			var err error
			runBounded(t, p, func() { img, err = decodeFile(p) })
			checkRefused(t, img, err, reason)
		})
	}

	t.Run("socket", func(t *testing.T) {
		pt, why := probeImage(sock)
		if pt != (image.Point{}) || why != reason {
			t.Errorf("probeImage = %v, %q, want no size and %q", pt, why, reason)
		}
		img, err := decodeFile(sock)
		checkRefused(t, img, err, reason)
	})

	t.Run("/dev/zero", func(t *testing.T) {
		img, err := decodeFile("/dev/zero")
		checkRefused(t, img, err, reason)
	})

	t.Run("symlink to a png", func(t *testing.T) {
		pt, why := probeImage(pngLink)
		if why != "" {
			t.Fatalf("probeImage of a link to a PNG gave REASON %q", why)
		}
		if pt != image.Pt(4, 3) {
			t.Errorf("probeImage size = %v, want 4x3", pt)
		}
		img := mustDecode(t, pngLink)
		r, g, b, a := img.At(0, 0).RGBA()
		if r != 0xffff || g != 0 || b != 0 || a != 0xffff {
			t.Errorf("pixel (0,0) = %x %x %x %x, want opaque red", r, g, b, a)
		}
	})
}
