package docker

import (
	"regexp"
	"strings"
)

// HostPattern matches Host(`...`) or Host("...") in Traefik router rules
var HostPattern = regexp.MustCompile(`Host\([\x60"]([^)\x60"]+)[\x60"]\)`)

// ExtractHostnames parses Traefik router rule labels and extracts Host() values
// Example label: "traefik.http.routers.whoami.rule=Host(`whoami.local`)"
func ExtractHostnames(labels map[string]string) []string {
	var hostnames []string
	seen := make(map[string]bool)

	for key, value := range labels {
		// Look for traefik.http.routers.*.rule labels
		if !strings.HasPrefix(key, "traefik.http.routers.") || !strings.HasSuffix(key, ".rule") {
			continue
		}

		// Extract all Host(...) patterns from the rule
		matches := HostPattern.FindAllStringSubmatch(value, -1)
		for _, match := range matches {
			if len(match) > 1 {
				hostname := match[1]
				if !seen[hostname] {
					hostnames = append(hostnames, hostname)
					seen[hostname] = true
				}
			}
		}
	}

	return hostnames
}
