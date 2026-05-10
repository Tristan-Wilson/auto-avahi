package hostname

import (
	"fmt"
	"strings"
)

// Options controls hostname validation.
type Options struct {
	// Suffixes is the list of allowed hostname suffixes (without leading dot).
	// A hostname must end with ".<suffix>" for one of these to be considered valid.
	Suffixes []string

	// EnforceDepthLimit, when true, additionally requires that the hostname have
	// 1 or 2 subdomain labels before the suffix (e.g. "app.local" or
	// "app.host.local" but not "a.b.c.local"). This is the recommended setting
	// for mDNS publishing where deep names are fragile to resolve. For
	// non-mDNS use (DNS handled elsewhere), set to false to allow any depth.
	EnforceDepthLimit bool
}

// Validate checks whether a hostname satisfies the given Options.
// Returns (true, "") if valid; (false, reason) otherwise.
func Validate(hostname string, opts Options) (bool, string) {
	suffix, ok := matchSuffix(hostname, opts.Suffixes)
	if !ok {
		return false, fmt.Sprintf("hostname %q does not end with any configured suffix (%s)",
			hostname, strings.Join(opts.Suffixes, ", "))
	}

	// Strip the suffix and the dot before it. The matched suffix is guaranteed
	// to be preceded by a dot OR to be the entire hostname (which is bare and
	// always rejected below).
	withoutSuffix := strings.TrimSuffix(hostname, suffix)
	withoutSuffix = strings.TrimSuffix(withoutSuffix, ".")

	if withoutSuffix == "" {
		return false, fmt.Sprintf("hostname %q is bare (need at least one subdomain, e.g., myserver.%s)",
			hostname, suffix)
	}

	if !opts.EnforceDepthLimit {
		return true, ""
	}

	parts := strings.Split(withoutSuffix, ".")
	if len(parts) <= 2 {
		return true, ""
	}

	return false, fmt.Sprintf("hostname %q has %d labels before suffix %q (mDNS-mode validation allows 1 or 2; e.g., myserver.%s or app.myserver.%s)",
		hostname, len(parts), suffix, suffix, suffix)
}

// Extract filters a list of hostnames using Validate and returns the valid ones
// alongside human-readable warnings for the rejects.
func Extract(hostnames []string, opts Options) (valid []string, warnings []string) {
	for _, hostname := range hostnames {
		ok, warning := Validate(hostname, opts)
		if ok {
			valid = append(valid, hostname)
		} else if warning != "" {
			warnings = append(warnings, warning)
		}
	}
	return valid, warnings
}

// matchSuffix returns the suffix that the hostname ends with (preceded by a dot),
// or an exact-match suffix if the hostname IS the bare suffix.
func matchSuffix(hostname string, suffixes []string) (string, bool) {
	for _, s := range suffixes {
		if strings.HasSuffix(hostname, "."+s) || hostname == s {
			return s, true
		}
	}
	return "", false
}
