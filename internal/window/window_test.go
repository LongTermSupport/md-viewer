package window

import (
	"errors"
	"image"
	"image/color"
	"slices"
	"strings"
	"testing"

	"gioui.org/layout"
	"gioui.org/op"
)

// Errors the scripts return. Their texts are the expectations of the messages, so they are
// literals where a text is checked.
var (
	errBoom   = errors.New("boom")
	errGPU    = errors.New("gpu lost")
	errGio    = errors.New("wayland: no")
	errW      = errors.New("write failed")
	errScript = errors.New("script exhausted")
)

// fakeSource replays a script of events. Every call to it is appended to log, in order, and so
// is every call the loop makes to the functions of Config that the test wires to it.
type fakeSource struct {
	script []Event
	log    []string
}

// Next returns the next scripted event. Past the end of the script it returns DestroyEvent with
// errScript, so a test that runs out of script fails on its result, not by hanging.
func (s *fakeSource) Next() Event {
	s.log = append(s.log, "next")
	if len(s.script) == 0 {
		return DestroyEvent{Err: errScript}
	}
	e := s.script[0]
	s.script = s.script[1:]
	return e
}

// Invalidate records the call.
func (s *fakeSource) Invalidate() { s.log = append(s.log, "invalidate") }

// Close records the call. Like the window, it then delivers a DestroyEvent, unless the script
// already has one still to come.
func (s *fakeSource) Close() {
	s.log = append(s.log, "close")
	if !slices.ContainsFunc(s.script, isDestroy) {
		s.script = append(s.script, DestroyEvent{})
	}
}

// isDestroy reports whether e is a DestroyEvent.
func isDestroy(e Event) bool {
	_, ok := e.(DestroyEvent)
	return ok
}

// frame is a FrameEvent for a 100 × 100 px window with its own Ops. Its Submit records the call.
func (s *fakeSource) frame() FrameEvent {
	return FrameEvent{
		Gtx: layout.Context{
			Ops:         new(op.Ops),
			Constraints: layout.Exact(image.Pt(100, 100)),
		},
		Submit: func(*op.Ops) { s.log = append(s.log, "submit") },
	}
}

// frameResult is what one call of a scripted Config.Frame returns.
type frameResult struct {
	quit bool
	err  error
}

// frames returns a Config.Frame that records each call and returns the results in order. Past
// the end of results it returns (false, nil).
func (s *fakeSource) frames(results ...frameResult) func(layout.Context) (bool, error) {
	return func(layout.Context) (bool, error) {
		s.log = append(s.log, "frame")
		if len(results) == 0 {
			return false, nil
		}
		r := results[0]
		results = results[1:]
		return r.quit, r.err
	}
}

// onFirst returns a Config.OnFirst that records the call and returns err.
func (s *fakeSource) onFirst(err error) func() error {
	return func() error {
		s.log = append(s.log, "onfirst")
		return err
	}
}

// checkLog asserts the order of the calls the loop made.
func checkLog(t *testing.T, got, want []string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Errorf("calls are %q, want %q", got, want)
	}
}

// TestFrameErrorMapping: a failed frame comes out of Loop as a *FrameError whose text is the
// error's own text, that unwraps to it (CLI-6). The window is closed once, after the frame was
// submitted. Without a failed frame, Loop returns nil on close and never closes.
func TestFrameErrorMapping(t *testing.T) {
	t.Run("failed frame", func(t *testing.T) {
		src := &fakeSource{}
		src.script = []Event{src.frame(), DestroyEvent{}}
		err := Loop(src, Config{Frame: src.frames(frameResult{err: errBoom})})
		var fe *FrameError
		if !errors.As(err, &fe) {
			t.Fatalf("Loop returned %v (%T), want a *FrameError", err, err)
		}
		if fe.Err != errBoom {
			t.Errorf("FrameError.Err = %v, want the frame's error", fe.Err)
		}
		if !errors.Is(err, errBoom) {
			t.Errorf("errors.Is(err, boom) is false, want true")
		}
		if err.Error() != "boom" {
			t.Errorf("Error() = %q, want %q", err.Error(), "boom")
		}
		checkLog(t, src.log, []string{"next", "frame", "submit", "close", "next"})
	})

	t.Run("no failed frame", func(t *testing.T) {
		src := &fakeSource{}
		src.script = []Event{src.frame(), DestroyEvent{}}
		if err := Loop(src, Config{Frame: src.frames()}); err != nil {
			t.Errorf("Loop returned %v, want nil", err)
		}
		checkLog(t, src.log, []string{"next", "frame", "submit", "next"})
	})
}

// TestLoopDestroyError: a window destroyed with an error of its own returns that error exactly,
// and no frame is drawn (CLI-9's path).
func TestLoopDestroyError(t *testing.T) {
	src := &fakeSource{script: []Event{DestroyEvent{Err: errGio}}}
	err := Loop(src, Config{
		Frame:   src.frames(),
		OnFirst: src.onFirst(nil),
	})
	if err != errGio {
		t.Errorf("Loop returned %v, want the destroy error", err)
	}
	checkLog(t, src.log, []string{"next"})
}

// TestLoopFrameErrorWins: when a frame failed, the destroy error does not replace it (CLI-6).
func TestLoopFrameErrorWins(t *testing.T) {
	src := &fakeSource{}
	src.script = []Event{src.frame(), DestroyEvent{Err: errGPU}}
	err := Loop(src, Config{Frame: src.frames(frameResult{err: errBoom})})
	var fe *FrameError
	if !errors.As(err, &fe) || fe.Err != errBoom {
		t.Errorf("Loop returned %v, want a FrameError holding the frame's error", err)
	}
	checkLog(t, src.log, []string{"next", "frame", "submit", "close", "next"})
}

// TestLoopQuit: a quit from the frame closes the window once. A frame the window still delivers
// is submitted, but the viewer is not called again.
func TestLoopQuit(t *testing.T) {
	src := &fakeSource{}
	src.script = []Event{src.frame(), src.frame(), DestroyEvent{}}
	err := Loop(src, Config{Frame: src.frames(frameResult{quit: true})})
	if err != nil {
		t.Errorf("Loop returned %v, want nil", err)
	}
	checkLog(t, src.log, []string{"next", "frame", "submit", "close", "next", "submit", "next"})
}

// TestLoopStopsAfterError: after a failed frame the viewer is not called again, but the frames
// the window still delivers are submitted. The result is the first failure.
func TestLoopStopsAfterError(t *testing.T) {
	src := &fakeSource{}
	src.script = []Event{src.frame(), src.frame(), DestroyEvent{}}
	err := Loop(src, Config{Frame: src.frames(frameResult{err: errBoom})})
	var fe *FrameError
	if !errors.As(err, &fe) || fe.Err != errBoom {
		t.Errorf("Loop returned %v, want a FrameError holding the frame's error", err)
	}
	checkLog(t, src.log, []string{"next", "frame", "submit", "close", "next", "submit", "next"})
}

// TestLoopOnFirst: OnFirst is called once, right after the first frame is submitted. Without
// ExitAfterFirst the window stays open; with it the window closes after OnFirst.
func TestLoopOnFirst(t *testing.T) {
	t.Run("window stays open", func(t *testing.T) {
		src := &fakeSource{}
		src.script = []Event{src.frame(), src.frame(), src.frame(), DestroyEvent{}}
		err := Loop(src, Config{Frame: src.frames(), OnFirst: src.onFirst(nil)})
		if err != nil {
			t.Errorf("Loop returned %v, want nil", err)
		}
		checkLog(t, src.log, []string{
			"next", "frame", "submit", "onfirst",
			"next", "frame", "submit",
			"next", "frame", "submit",
			"next",
		})
	})

	t.Run("exit after first", func(t *testing.T) {
		src := &fakeSource{}
		src.script = []Event{src.frame(), DestroyEvent{}}
		err := Loop(src, Config{Frame: src.frames(), OnFirst: src.onFirst(nil), ExitAfterFirst: true})
		if err != nil {
			t.Errorf("Loop returned %v, want nil", err)
		}
		checkLog(t, src.log, []string{"next", "frame", "submit", "onfirst", "close", "next"})
	})
}

// TestLoopOnFirstError: a failed OnFirst is a frame failure: the window closes and Loop returns a
// *FrameError that wraps the error (CLI-6, the --time line's write).
func TestLoopOnFirstError(t *testing.T) {
	src := &fakeSource{}
	src.script = []Event{src.frame(), DestroyEvent{}}
	err := Loop(src, Config{Frame: src.frames(), OnFirst: src.onFirst(errW), ExitAfterFirst: true})
	var fe *FrameError
	if !errors.As(err, &fe) || !errors.Is(err, errW) {
		t.Errorf("Loop returned %v, want a FrameError wrapping the OnFirst error", err)
	}
	checkLog(t, src.log, []string{"next", "frame", "submit", "onfirst", "close", "next"})
}

// TestLoopNoOnFirstAfterFailedFrame: when the first frame fails, OnFirst is never called. This is
// a control: the stub calls nothing, so it passes on the stub too. The failure itself is checked
// by TestFrameErrorMapping.
func TestLoopNoOnFirstAfterFailedFrame(t *testing.T) {
	src := &fakeSource{}
	src.script = []Event{src.frame(), DestroyEvent{}}
	err := Loop(src, Config{Frame: src.frames(frameResult{err: errBoom}), OnFirst: src.onFirst(nil)})
	var fe *FrameError
	if err != nil && !errors.As(err, &fe) {
		t.Errorf("Loop returned %v, want a *FrameError", err)
	}
	if slices.Contains(src.log, "onfirst") {
		t.Errorf("OnFirst was called after a failed first frame: calls are %q", src.log)
	}
}

// fillCall is one call of the canvas fill: the Ops it painted, and the colour.
type fillCall struct {
	ops    *op.Ops
	colour color.NRGBA
}

// TestLoopCanvasFill: every frame is filled with the canvas colour, in its own Ops, before the
// viewer draws it, and a frame after a quit is filled too (STYLE-PAL-3). The colours are the
// light and dark palettes' Canvas.
func TestLoopCanvasFill(t *testing.T) {
	cases := []struct {
		name   string
		canvas color.NRGBA
	}{
		{name: "light", canvas: color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}},
		{name: "dark", canvas: color.NRGBA{R: 0x0d, G: 0x11, B: 0x17, A: 0xff}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			src := &fakeSource{}
			f1, f2 := src.frame(), src.frame()
			src.script = []Event{f1, f2, DestroyEvent{}}
			var fills []fillCall
			fill := func(ops *op.Ops, colour color.NRGBA) {
				src.log = append(src.log, "fill")
				fills = append(fills, fillCall{ops: ops, colour: colour})
			}
			err := loop(src, Config{Canvas: c.canvas, Frame: src.frames(frameResult{quit: true})}, fill)
			if err != nil {
				t.Errorf("loop returned %v, want nil", err)
			}
			want := []fillCall{{ops: f1.Gtx.Ops, colour: c.canvas}, {ops: f2.Gtx.Ops, colour: c.canvas}}
			if len(fills) != len(want) {
				t.Fatalf("fill was called %d times, want %d", len(fills), len(want))
			}
			for i := range want {
				if fills[i].ops != want[i].ops || fills[i].colour != want[i].colour {
					t.Errorf("fill call %d = %+v, want the frame's Ops and colour %v", i, fills[i], want[i].colour)
				}
			}
			checkLog(t, src.log, []string{"next", "fill", "frame", "submit", "close", "next", "fill", "submit", "next"})
		})
	}
}

// TestLoopBind: Bind is called once, before the first Next, with a function that invalidates
// the source. A nil Bind is allowed.
func TestLoopBind(t *testing.T) {
	t.Run("bound invalidate", func(t *testing.T) {
		src := &fakeSource{script: []Event{DestroyEvent{}}}
		var invalidate func()
		err := Loop(src, Config{Frame: src.frames(), Bind: func(inv func()) {
			src.log = append(src.log, "bind")
			invalidate = inv
		}})
		if err != nil {
			t.Errorf("Loop returned %v, want nil", err)
		}
		checkLog(t, src.log, []string{"bind", "next"})
		if invalidate == nil {
			t.Fatal("Bind was not given a function")
		}
		invalidate()
		invalidates := 0
		for _, c := range src.log {
			if c == "invalidate" {
				invalidates++
			}
		}
		if invalidates != 1 {
			t.Errorf("the bound function invalidated %d times, want 1", invalidates)
		}
	})

	t.Run("nil bind", func(t *testing.T) {
		src := &fakeSource{}
		src.script = []Event{src.frame(), DestroyEvent{}}
		if err := Loop(src, Config{Frame: src.frames()}); err != nil {
			t.Errorf("Loop returned %v, want nil", err)
		}
	})
}

// TestLoopNilFrame: a Config without Frame is an error, and no window is asked for an event, so
// no window would open.
func TestLoopNilFrame(t *testing.T) {
	src := &fakeSource{script: []Event{DestroyEvent{}}}
	err := Loop(src, Config{})
	if err == nil {
		t.Fatalf("Loop returned nil, want an error naming Config.Frame")
	}
	if !strings.Contains(err.Error(), "Config.Frame") {
		t.Errorf("Loop error is %q, want it to name Config.Frame", err.Error())
	}
	checkLog(t, src.log, nil)
}
