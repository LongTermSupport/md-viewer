package doc

import (
	"net/url"
	"path/filepath"
	"strings"
)

// URL schemes and hosts that DOC-5 names.
const (
	schemeHTTP   = "http"
	schemeHTTPS  = "https"
	schemeMailto = "mailto"
	schemeFile   = "file"
	localHost    = "localhost" // DOC-5, P7: the only host a file URL may name besides none
)

// mailtoPrefix is put in front of an email address to make its link target (DOC-4, P2).
const mailtoPrefix = schemeMailto + ":"

// ResolveLink classifies a link destination against the document directory (DOC-5). The
// escapes and references in dest are resolved first, as DOC-3 resolves them in text (P6), so
// the Dest of the result is the resolved destination.
func ResolveLink(dir, dest string) Target {
	return classify(dir, resolveText([]byte(dest)))
}

// classify applies DOC-5's table to a destination whose escapes are already resolved. An
// autolink's URL has no escapes (CommonMark), so it is classified without resolving them.
func classify(dir, dest string) Target {
	if strings.HasPrefix(dest, "#") {
		return Target{Kind: Fragment, Value: dest, Dest: dest}
	}
	u, err := url.Parse(dest)
	if err != nil {
		return refused(dest)
	}
	switch u.Scheme {
	case schemeHTTP, schemeHTTPS, schemeMailto:
		return Target{Kind: External, Value: dest, Dest: dest}
	case schemeFile:
		if u.Opaque != "" || (u.Host != "" && u.Host != localHost) {
			return refused(dest)
		}
		return Target{Kind: LocalFile, Value: filepath.Clean(u.Path), Dest: dest}
	case "":
		if u.Host != "" {
			return refused(dest)
		}
		if filepath.IsAbs(u.Path) {
			return Target{Kind: LocalFile, Value: filepath.Clean(u.Path), Dest: dest}
		}
		return Target{Kind: LocalFile, Value: filepath.Join(dir, u.Path), Dest: dest}
	default:
		return refused(dest)
	}
}

// refused is the target of a destination md-viewer does not follow (DOC-5).
func refused(dest string) Target {
	return Target{Kind: Refused, Value: dest, Dest: dest}
}
