package config

import (
	"fmt"
	"os"
	"strings"
)

// Config holds the application configuration
type Config struct {
	// HostIP is the IP address to publish in mDNS records.
	// Required when MDNSEnable is true; ignored otherwise.
	HostIP string

	// CertDir is the directory where mkcert certificates are stored (host path)
	CertDir string

	// CertDirContainer is the path where certs are mounted inside Traefik container
	CertDirContainer string

	// TraefikConfigDir is the directory where Traefik cert YAML files are written
	TraefikConfigDir string

	// CARoot is the mkcert CA root directory. When set, it is passed as the
	// CAROOT environment variable to mkcert so certificates are signed by a
	// specific CA (e.g. a non-root user's CA when the service runs as root).
	// When empty, mkcert uses its default CA for the current user.
	CARoot string

	// HostnameSuffixes is the list of allowed hostname suffixes (without leading dot).
	// Hostnames extracted from Traefik labels must end with one of these suffixes
	// to be processed. Defaults to ["local"].
	HostnameSuffixes []string

	// MDNSEnable controls whether hostnames are published via avahi-publish.
	// When false, the mDNS publisher is not initialized and no avahi-publish
	// processes are spawned — useful when DNS is handled elsewhere (e.g.
	// split-horizon DNS, public DNS, /etc/hosts). Defaults to true.
	MDNSEnable bool
}

// Load loads configuration from environment variables with defaults
func Load() (*Config, error) {
	cfg := &Config{
		HostIP:           getEnv("AVAHI_HOST_IP", ""),
		CertDir:          getEnv("CERT_DIR", "/certs"),
		CertDirContainer: getEnv("CERT_DIR_CONTAINER", "/certs"),
		TraefikConfigDir: getEnv("TRAEFIK_CONFIG_DIR", "/traefik/dynamic"),
		CARoot:           os.Getenv("CAROOT"),
		HostnameSuffixes: parseSuffixes(getEnv("HOSTNAME_SUFFIXES", "local")),
		MDNSEnable:       parseBool(getEnv("MDNS_ENABLE", "true")),
	}

	if cfg.MDNSEnable && cfg.HostIP == "" {
		return nil, fmt.Errorf("AVAHI_HOST_IP environment variable is required when MDNS_ENABLE=true")
	}

	if len(cfg.HostnameSuffixes) == 0 {
		return nil, fmt.Errorf("HOSTNAME_SUFFIXES must contain at least one suffix")
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// parseSuffixes splits a comma-separated suffix list, trims whitespace and
// leading dots, and drops empty entries.
func parseSuffixes(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		p = strings.TrimPrefix(p, ".")
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// parseBool accepts the usual true/false spellings.
func parseBool(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "true", "1", "yes", "on":
		return true
	default:
		return false
	}
}
