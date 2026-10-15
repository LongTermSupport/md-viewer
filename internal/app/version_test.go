package app

import (
	"runtime/debug"
	"testing"
)

func TestVersionLine(t *testing.T) {
	const rev = "0123456789abcdef0123456789abcdef01234567"
	cases := []struct {
		name string
		info *debug.BuildInfo
		ok   bool
		want string
	}{
		{
			name: "tagged version",
			info: &debug.BuildInfo{Main: debug.Module{Version: "v1.2.3"}},
			ok:   true,
			want: "md-viewer v1.2.3",
		},
		{
			name: "devel with revision, modified",
			info: &debug.BuildInfo{
				Main: debug.Module{Version: "(devel)"},
				Settings: []debug.BuildSetting{
					{Key: "vcs.revision", Value: rev},
					{Key: "vcs.modified", Value: "true"},
				},
			},
			ok:   true,
			want: "md-viewer (devel) 0123456789ab dirty",
		},
		{
			name: "devel with revision, not modified",
			info: &debug.BuildInfo{
				Main: debug.Module{Version: "(devel)"},
				Settings: []debug.BuildSetting{
					{Key: "vcs.revision", Value: rev},
					{Key: "vcs.modified", Value: "false"},
				},
			},
			ok:   true,
			want: "md-viewer (devel) 0123456789ab",
		},
		{
			name: "devel without settings",
			info: &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}},
			ok:   true,
			want: "md-viewer (devel)",
		},
		{
			name: "no build info",
			info: nil,
			ok:   false,
			want: "md-viewer unknown",
		},
		{
			name: "devel with empty revision has no trailing space",
			info: &debug.BuildInfo{
				Main: debug.Module{Version: "(devel)"},
				Settings: []debug.BuildSetting{
					{Key: "vcs.revision", Value: ""},
				},
			},
			ok:   true,
			want: "md-viewer (devel)",
		},
		{
			name: "empty module version is unknown",
			info: &debug.BuildInfo{Main: debug.Module{Version: ""}},
			ok:   true,
			want: "md-viewer unknown",
		},
		{
			name: "pseudo-version printed unchanged",
			info: &debug.BuildInfo{
				Main: debug.Module{Version: "v0.0.0-20261015093457-bc31de7ac1ad+dirty"},
				Settings: []debug.BuildSetting{
					{Key: "vcs.revision", Value: rev},
					{Key: "vcs.modified", Value: "true"},
				},
			},
			ok:   true,
			want: "md-viewer v0.0.0-20261015093457-bc31de7ac1ad+dirty",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := VersionLine(c.info, c.ok); got != c.want {
				t.Errorf("VersionLine() = %q, want %q", got, c.want)
			}
		})
	}
}
