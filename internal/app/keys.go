package app

import "gioui.org/io/key"

// Action is what a key press does in the viewer (SCROLL-2, ZOOM-2, WIN-4).
type Action uint8

const (
	ActionNone Action = iota
	ActionLineDown
	ActionLineUp
	ActionPageDown
	ActionPageUp
	ActionTop
	ActionBottom
	ActionZoomIn
	ActionZoomOut
	ActionZoomReset
	ActionQuit
)

// String is the action's name. Stub: not yet implemented.
func (a Action) String() string {
	return ""
}

// ActionFor is the action of a key event. Stub: not yet implemented.
func ActionFor(e key.Event) Action {
	return ActionNone
}
