package app

import (
	"fmt"

	"gioui.org/io/event"
	"gioui.org/io/key"
)

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

// String is the action's name, for messages and tests.
func (a Action) String() string {
	switch a {
	case ActionNone:
		return "ActionNone"
	case ActionLineDown:
		return "ActionLineDown"
	case ActionLineUp:
		return "ActionLineUp"
	case ActionPageDown:
		return "ActionPageDown"
	case ActionPageUp:
		return "ActionPageUp"
	case ActionTop:
		return "ActionTop"
	case ActionBottom:
		return "ActionBottom"
	case ActionZoomIn:
		return "ActionZoomIn"
	case ActionZoomOut:
		return "ActionZoomOut"
	case ActionZoomReset:
		return "ActionZoomReset"
	case ActionQuit:
		return "ActionQuit"
	default:
		return fmt.Sprintf("Action(%d)", uint8(a))
	}
}

// binding is one row of the key table. A key event matches when its name is name, it has every
// required modifier, and it has no modifier outside required and optional. Gio folds Shift into
// the name of the key, so Ctrl+Shift+= arrives as "+" (ZOOM-2).
type binding struct {
	name     key.Name
	required key.Modifiers
	optional key.Modifiers
	action   Action
}

// bindings is the one table of keys the viewer answers to. ActionFor and keyFilters both read it.
var bindings = []binding{
	{key.NameDownArrow, 0, 0, ActionLineDown},      // SCROLL-2
	{key.NameUpArrow, 0, 0, ActionLineUp},          // SCROLL-2
	{key.NamePageDown, 0, 0, ActionPageDown},       // SCROLL-2
	{key.NamePageUp, 0, 0, ActionPageUp},           // SCROLL-2
	{key.NameSpace, 0, 0, ActionPageDown},          // SCROLL-2
	{key.NameSpace, key.ModShift, 0, ActionPageUp}, // SCROLL-2
	{key.NameHome, 0, 0, ActionTop},                // SCROLL-2
	{key.NameEnd, 0, 0, ActionBottom},              // SCROLL-2
	{"+", key.ModCtrl, key.ModShift, ActionZoomIn}, // ZOOM-2
	{"=", key.ModCtrl, key.ModShift, ActionZoomIn}, // ZOOM-2
	{"-", key.ModCtrl, 0, ActionZoomOut},           // ZOOM-2
	{"0", key.ModCtrl, 0, ActionZoomReset},         // ZOOM-2
	{"Q", 0, key.ModShift, ActionQuit},             // WIN-4
	{key.NameEscape, 0, key.ModShift, ActionQuit},  // WIN-4
}

// ActionFor is the action of a key event: the table's action for a press, and None for a release
// or for a key the table does not hold (SCROLL-2: a key acts on press).
func ActionFor(e key.Event) Action {
	if e.State != key.Press {
		return ActionNone
	}
	for _, b := range bindings {
		if e.Name == b.name &&
			e.Modifiers&b.required == b.required &&
			e.Modifiers&^(b.required|b.optional) == 0 {
			return b.action
		}
	}
	return ActionNone
}

// keyFilters are the filters that route each bound key to the viewer. A filter with no Focus gets
// its keys whatever has focus, and the viewer never takes focus (ARCHITECTURE "Input routing").
func keyFilters() []event.Filter {
	fs := make([]event.Filter, len(bindings))
	for i, b := range bindings {
		fs[i] = key.Filter{Name: b.name, Required: b.required, Optional: b.optional}
	}
	return fs
}
