package hostname

import (
	"fmt"
	"strings"
)

// Validate checks if a hostname is a valid 2-level .local hostname
// Valid: whoami.local
// Invalid: app.example.local (warns), example.com (rejects)
func Validate(hostname string) (bool, string) {
	if !strings.HasSuffix(hostname, ".local") {
		return false, fmt.Sprintf("hostname %q does not end with .local", hostname)
	}

	// Remove .local suffix and count parts
	withoutLocal := strings.TrimSuffix(hostname, ".local")
	parts := strings.Split(withoutLocal, ".")

	if len(parts) == 1 {
		// Valid: "whoami.local" -> ["whoami"]
		return true, ""
	}

	if len(parts) > 1 {
		// Invalid: "app.example.local" -> ["app", "example"]
		return false, fmt.Sprintf("hostname %q has %d levels (only 2-level .local hostnames are supported, e.g., whoami.local)",
			hostname, len(parts)+1)
	}

	return false, "invalid hostname format"
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
