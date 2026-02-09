package config

import (
	"fmt"
	"os"
)

// Config holds the application configuration
type Config struct {
	// HostIP is the IP address to use for mDNS records
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
}

// Load loads configuration from environment variables with defaults
func Load() (*Config, error) {
	cfg := &Config{
		HostIP:           getEnv("AVAHI_HOST_IP", ""),
		CertDir:          getEnv("CERT_DIR", "/certs"),
		CertDirContainer: getEnv("CERT_DIR_CONTAINER", "/certs"),
		TraefikConfigDir: getEnv("TRAEFIK_CONFIG_DIR", "/traefik/dynamic"),
		CARoot:           os.Getenv("CAROOT"),
	}

	if cfg.HostIP == "" {
		return nil, fmt.Errorf("AVAHI_HOST_IP environment variable is required")
	}

	return cfg, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
