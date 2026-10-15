package render

import (
	"image/color"
	"testing"
)

// TestPalettes checks every colour of STYLE-PAL-1 for both palettes. The values are written
// out by hand from the spec's hex table, so a wrong constant fails here.
func TestPalettes(t *testing.T) {
	tests := []struct {
		name      string
		got, want Palette
	}{
		{
			name: "light",
			got:  Light,
			want: Palette{
				Fg:          color.NRGBA{R: 0x1f, G: 0x23, B: 0x28, A: 0xff},
				Canvas:      color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
				FgMuted:     color.NRGBA{R: 0x59, G: 0x63, B: 0x6e, A: 0xff},
				Accent:      color.NRGBA{R: 0x09, G: 0x69, B: 0xda, A: 0xff},
				Border:      color.NRGBA{R: 0xd1, G: 0xd9, B: 0xe0, A: 0xff},
				BorderMuted: color.NRGBA{R: 0xd1, G: 0xd9, B: 0xe0, A: 0xb3},
				BgMuted:     color.NRGBA{R: 0xf6, G: 0xf8, B: 0xfa, A: 0xff},
				CodeBg:      color.NRGBA{R: 0x81, G: 0x8b, B: 0x98, A: 0x1f},
				OnAccent:    color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
			},
		},
		{
			name: "dark",
			got:  Dark,
			want: Palette{
				Fg:          color.NRGBA{R: 0xf0, G: 0xf6, B: 0xfc, A: 0xff},
				Canvas:      color.NRGBA{R: 0x0d, G: 0x11, B: 0x17, A: 0xff},
				FgMuted:     color.NRGBA{R: 0x91, G: 0x98, B: 0xa1, A: 0xff},
				Accent:      color.NRGBA{R: 0x44, G: 0x93, B: 0xf8, A: 0xff},
				Border:      color.NRGBA{R: 0x3d, G: 0x44, B: 0x4d, A: 0xff},
				BorderMuted: color.NRGBA{R: 0x3d, G: 0x44, B: 0x4d, A: 0xb3},
				BgMuted:     color.NRGBA{R: 0x15, G: 0x1b, B: 0x23, A: 0xff},
				CodeBg:      color.NRGBA{R: 0x65, G: 0x6c, B: 0x76, A: 0x33},
				OnAccent:    color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("palette:\n got %#v\nwant %#v", tt.got, tt.want)
			}
		})
	}
}
