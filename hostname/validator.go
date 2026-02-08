package hostname

import (
	"fmt"
	"strings"
)

// Validate checks if a hostname is a valid .local hostname.
// Supports 2-level (e.g., myserver.local) and 3-level (e.g., app.myserver.local) names.
// Rejects bare ".local", non-.local names, and names with 4+ levels.
func Validate(hostname string) (bool, string) {
	if !strings.HasSuffix(hostname, ".local") {
		return false, fmt.Sprintf("hostname %q does not end with .local", hostname)
	}

	// Remove .local suffix and count parts
	withoutLocal := strings.TrimSuffix(hostname, ".local")
	parts := strings.Split(withoutLocal, ".")

	if withoutLocal == "" || len(parts) == 0 {
		// Invalid: bare ".local"
		return false, fmt.Sprintf("hostname %q is bare .local (need at least a name, e.g., myserver.local)", hostname)
	}

	if len(parts) <= 2 {
		// Valid: "myserver.local" -> ["myserver"] (2-level)
		// Valid: "app.myserver.local" -> ["app", "myserver"] (3-level)
		return true, ""
	}

	// Invalid: 4+ levels
	return false, fmt.Sprintf("hostname %q has %d levels (only 2 or 3-level .local hostnames are supported, e.g., myserver.local or app.myserver.local)",
		hostname, len(parts)+1)
}

// Extract extracts all valid hostnames from a list
// Returns valid hostnames and warnings for invalid ones
func Extract(hostnames []string) (valid []string, warnings []string) {
	for _, hostname := range hostnames {
		ok, warning := Validate(hostname)
		if ok {
			valid = append(valid, hostname)
		} else if warning != "" {
			warnings = append(warnings, warning)
		}
	}
	return valid, warnings
}
