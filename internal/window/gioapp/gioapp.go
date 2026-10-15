// Package gioapp is the window of md-viewer on Gio's app package: the only importer of
// gioui.org/app. It adapts app.Window to window.Source, so the loop and its rules live in
// internal/window, where they are unit-tested.
package gioapp

import (
	"gioui.org/app"
	"gioui.org/io/system"
	"gioui.org/op"

	"github.com/LongTermSupport/md-viewer/internal/window"
)

// Run opens the window and runs its loop until the window is gone. It returns nil when the window
// closed normally, a *window.FrameError when a frame failed, and otherwise the window's error
// (CLI-6, CLI-9).
func Run(c window.Config) error {
	w := new(app.Window)
	w.Option(options(c.Title)...)
	return window.Loop(&source{w: w}, c)
}

// Main hands the main goroutine to gioui.org/app. It must run on the main goroutine and it never
// returns.
func Main() { app.Main() }

// options are the window's options: its title, and its size and minimum size in dp from
// internal/window (WIN-1, WIN-2).
func options(title string) []app.Option {
	return []app.Option{
		app.Title(title),
		app.Size(window.Width, window.Height),
		app.MinSize(window.MinWidth, window.MinHeight),
	}
}

// source is the window as window.Source.
type source struct {
	w   *app.Window
	ops op.Ops
}

// Next returns the next frame or destroy event. Every other Gio event is skipped.
func (s *source) Next() window.Event {
	for {
		switch e := s.w.Event().(type) {
		case app.FrameEvent:
			return window.FrameEvent{Gtx: app.NewContext(&s.ops, e), Submit: e.Frame}
		case app.DestroyEvent:
			return window.DestroyEvent{Err: e.Err}
		}
	}
}

// Invalidate asks for a frame. It is safe to call from any goroutine.
func (s *source) Invalidate() { s.w.Invalidate() }

// Close asks the window to close (WIN-4).
func (s *source) Close() { s.w.Perform(system.ActionClose) }
