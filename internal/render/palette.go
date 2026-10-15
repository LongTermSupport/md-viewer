// Package render turns a *doc.Document into Gio layout calls. Styles and palettes are plain
// values, so most of the package is tested without a window (ARCHITECTURE "internal/render").
package render

import "image/color"

// Palette is the set of named colours (STYLE-PAL-1). Each field is a colour of the light or
// dark stylesheet, with alpha last.
type Palette struct {
	Fg, Canvas, FgMuted, Accent, Border, BorderMuted, BgMuted, CodeBg, OnAccent color.NRGBA
}

// Light is the default palette (STYLE-PAL-1, STYLE-PAL-2). Its colours are GitHub's light
// stylesheet values, cited per field in SPEC section 3.1.
var Light = Palette{
	Fg:          color.NRGBA{R: 0x1f, G: 0x23, B: 0x28, A: 0xff}, // .markdown-body color
	Canvas:      color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}, // .markdown-body background
	FgMuted:     color.NRGBA{R: 0x59, G: 0x63, B: 0x6e, A: 0xff}, // h6, blockquote colour
	Accent:      color.NRGBA{R: 0x09, G: 0x69, B: 0xda, A: 0xff}, // a colour
	Border:      color.NRGBA{R: 0xd1, G: 0xd9, B: 0xe0, A: 0xff}, // table, rule, quote bar
	BorderMuted: color.NRGBA{R: 0xd1, G: 0xd9, B: 0xe0, A: 0xb3}, // h1, h2 border-bottom
	BgMuted:     color.NRGBA{R: 0xf6, G: 0xf8, B: 0xfa, A: 0xff}, // pre background
	CodeBg:      color.NRGBA{R: 0x81, G: 0x8b, B: 0x98, A: 0x1f}, // code background
	OnAccent:    color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}, // md-viewer's task check mark
}

// Dark is the palette for --dark (STYLE-PAL-1, STYLE-PAL-2), from GitHub's dark stylesheet.
var Dark = Palette{
	Fg:          color.NRGBA{R: 0xf0, G: 0xf6, B: 0xfc, A: 0xff}, // .markdown-body color
	Canvas:      color.NRGBA{R: 0x0d, G: 0x11, B: 0x17, A: 0xff}, // .markdown-body background
	FgMuted:     color.NRGBA{R: 0x91, G: 0x98, B: 0xa1, A: 0xff}, // h6, blockquote colour
	Accent:      color.NRGBA{R: 0x44, G: 0x93, B: 0xf8, A: 0xff}, // a colour
	Border:      color.NRGBA{R: 0x3d, G: 0x44, B: 0x4d, A: 0xff}, // table, rule, quote bar
	BorderMuted: color.NRGBA{R: 0x3d, G: 0x44, B: 0x4d, A: 0xb3}, // h1, h2 border-bottom
	BgMuted:     color.NRGBA{R: 0x15, G: 0x1b, B: 0x23, A: 0xff}, // pre background
	CodeBg:      color.NRGBA{R: 0x65, G: 0x6c, B: 0x76, A: 0x33}, // code background
	OnAccent:    color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}, // md-viewer's task check mark
}
