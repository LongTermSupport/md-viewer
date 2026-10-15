package app

import "runtime/debug"

// Stub: zero-value behaviour, written so the M0 tests compile and fail on their assertions.

func VersionLine(info *debug.BuildInfo, ok bool) string {
	return ""
}
