package app

import (
	"testing"

	"gioui.org/io/key"
)

// pressed is a key press event, as the window sends it.
func pressed(name key.Name, mods key.Modifiers) key.Event {
	return key.Event{Name: name, Modifiers: mods, State: key.Press}
}

// released is a key release event.
func released(name key.Name, mods key.Modifiers) key.Event {
	return key.Event{Name: name, Modifiers: mods, State: key.Release}
}

// TestActionFor checks every key of SCROLL-2, ZOOM-2 and WIN-4 with and without modifiers, and
// the controls that must do nothing. Gio names a letter by its upper-case form and folds Shift
// into the name of the key (Ctrl+Shift+= arrives as "+", Ctrl+Shift+- as "_").
func TestActionFor(t *testing.T) {
	const ctrl, shift, alt, super = key.ModCtrl, key.ModShift, key.ModAlt, key.ModSuper
	tests := []struct {
		name  string
		event key.Event
		want  Action
	}{
		// SCROLL-2
		{"down arrow", pressed(key.NameDownArrow, 0), ActionLineDown},
		{"up arrow", pressed(key.NameUpArrow, 0), ActionLineUp},
		{"page down", pressed(key.NamePageDown, 0), ActionPageDown},
		{"page up", pressed(key.NamePageUp, 0), ActionPageUp},
		{"space", pressed(key.NameSpace, 0), ActionPageDown},
		{"shift space", pressed(key.NameSpace, shift), ActionPageUp},
		{"home", pressed(key.NameHome, 0), ActionTop},
		{"end", pressed(key.NameEnd, 0), ActionBottom},
		// ZOOM-2
		{"ctrl plus", pressed("+", ctrl), ActionZoomIn},
		{"ctrl shift equals, delivered as plus", pressed("+", ctrl|shift), ActionZoomIn},
		{"ctrl equals", pressed("=", ctrl), ActionZoomIn},
		{"ctrl minus", pressed("-", ctrl), ActionZoomOut},
		{"ctrl zero", pressed("0", ctrl), ActionZoomReset},
		// WIN-4
		{"q", pressed("Q", 0), ActionQuit},
		{"shift q", pressed("Q", shift), ActionQuit},
		{"escape", pressed(key.NameEscape, 0), ActionQuit},
		{"shift escape", pressed(key.NameEscape, shift), ActionQuit},
		// Controls: no action.
		{"release of down arrow", released(key.NameDownArrow, 0), ActionNone},
		{"release of q", released("Q", 0), ActionNone},
		{"plus without ctrl", pressed("+", 0), ActionNone},
		{"zero without ctrl", pressed("0", 0), ActionNone},
		{"ctrl shift minus, delivered as underscore", pressed("_", ctrl|shift), ActionNone},
		{"ctrl q", pressed("Q", ctrl), ActionNone},
		{"alt q", pressed("Q", alt), ActionNone},
		{"super q", pressed("Q", super), ActionNone},
		{"ctrl alt equals", pressed("=", ctrl|alt), ActionNone},
		{"ctrl space", pressed(key.NameSpace, ctrl), ActionNone},
		{"letter a", pressed("A", 0), ActionNone},
		{"shift down arrow", pressed(key.NameDownArrow, shift), ActionNone},
		{"ctrl end", pressed(key.NameEnd, ctrl), ActionNone},
		{"ctrl home", pressed(key.NameHome, ctrl), ActionNone},
		{"alt page down", pressed(key.NamePageDown, alt), ActionNone},
		{"ctrl escape", pressed(key.NameEscape, ctrl), ActionNone},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ActionFor(tt.event); got != tt.want {
				t.Errorf("ActionFor(%+v) = %v, want %v", tt.event, got, tt.want)
			}
		})
	}
}

// TestActionString checks each action's name, and that a value outside the table is shown by
// its number.
func TestActionString(t *testing.T) {
	tests := []struct {
		action Action
		want   string
	}{
		{ActionNone, "ActionNone"},
		{ActionLineDown, "ActionLineDown"},
		{ActionLineUp, "ActionLineUp"},
		{ActionPageDown, "ActionPageDown"},
		{ActionPageUp, "ActionPageUp"},
		{ActionTop, "ActionTop"},
		{ActionBottom, "ActionBottom"},
		{ActionZoomIn, "ActionZoomIn"},
		{ActionZoomOut, "ActionZoomOut"},
		{ActionZoomReset, "ActionZoomReset"},
		{ActionQuit, "ActionQuit"},
		{Action(99), "Action(99)"},
	}
	for _, tt := range tests {
		if got := tt.action.String(); got != tt.want {
			t.Errorf("Action(%d).String() = %q, want %q", uint8(tt.action), got, tt.want)
		}
	}
}
