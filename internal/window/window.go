// Package window runs the one window of md-viewer: its event loop, the frame and the close, with
// no import of gioui.org/app, so it builds without cgo on every platform. The package that
// adapts the native window to Source is internal/window/gioapp (ARCHITECTURE "internal/window").
package window

import (
	"image/color"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

// The window's size, in dp (WIN-2).
const (
	Width, Height       unit.Dp = 980, 900
	MinWidth, MinHeight unit.Dp = 320, 240
)

// Config is what the window shows and how it reacts to its frames.
type Config struct {
	Title          string                                          // WIN-1
	Canvas         color.NRGBA                                     // painted under every frame (STYLE-PAL-3)
	Frame          func(gtx layout.Context) (quit bool, err error) // the viewer; quit closes the window; an error fails the frame
	Bind           func(invalidate func())                         // called once, before the first event, with the window's Invalidate
	OnFirst        func() error                                    // called once, after the first frame is submitted; an error fails the frame (CLI-6)
	ExitAfterFirst bool                                            // --time: close after the first frame
}

// FrameError is the error of a failed frame (CLI-6). Err is the error that failed it.
type FrameError struct{ Err error }

// Error is the text of Err, without a prefix; main adds the prefix (CLI-6).
func (e *FrameError) Error() string { return "" }

// Unwrap returns Err.
func (e *FrameError) Unwrap() error { return nil }

// Event is an event of the window, as Loop sees it. The set is closed: only this package
// defines the kinds, so an unknown value is an error.
type Event interface{ windowEvent() }

// FrameEvent asks for one frame. Gtx is the frame's context; Submit completes the frame.
type FrameEvent struct {
	Gtx    layout.Context
	Submit func(*op.Ops)
}

// DestroyEvent says the window is gone. Err is the window's own error, nil when it closed
// normally.
type DestroyEvent struct{ Err error }

func (FrameEvent) windowEvent()   {}
func (DestroyEvent) windowEvent() {}

// Source is the window: its next event, its Invalidate, and its Close, which asks the window to
// close.
type Source interface {
	Next() Event
	Invalidate()
	Close()
}

// Loop runs the window until it is destroyed. It returns nil when the window closed normally, a
// *FrameError when a frame failed, and otherwise the error of the destroyed window. Its frames are
// filled with Config.Canvas (STYLE-PAL-3).
func Loop(src Source, c Config) error { return loop(src, c, paint.Fill) }

// loop is Loop with the canvas fill given, so a test can see each fill.
func loop(src Source, c Config, fill func(*op.Ops, color.NRGBA)) error { return nil }
