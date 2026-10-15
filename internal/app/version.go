package app

import "runtime/debug"

// Version line parts (CLI-5).
const (
	unknownVersion = "unknown"
	develVersion   = "(devel)"
	revisionKey    = "vcs.revision"
	modifiedKey    = "vcs.modified"
	modifiedValue  = "true"
	revisionChars  = 12 // CLI-5: the first 12 characters of vcs.revision
	dirtySuffix    = " dirty"
)

// VersionLine returns the --version line, `md-viewer VERSION` (CLI-5). The version is the
// main module version as the Go toolchain stamps it. A (devel) build with a vcs.revision
// setting shows the revision, and " dirty" when vcs.modified is true. With no build info
// the version is "unknown".
func VersionLine(info *debug.BuildInfo, ok bool) string {
	if !ok || info == nil {
		return progName + " " + unknownVersion
	}
	v := info.Main.Version
	if v == develVersion {
		settings := make(map[string]string, len(info.Settings))
		for _, s := range info.Settings {
			settings[s.Key] = s.Value
		}
		if rev, found := settings[revisionKey]; found {
			v += " " + rev[:min(revisionChars, len(rev))]
			if settings[modifiedKey] == modifiedValue {
				v += dirtySuffix
			}
		}
	}
	return progName + " " + v
}
