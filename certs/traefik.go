package certs

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// TraefikConfig represents the Traefik TLS configuration file structure
type TraefikConfig struct {
	TLS *TLSConfig `yaml:"tls"`
}

// TLSConfig represents the TLS section
type TLSConfig struct {
	Certificates []Certificate `yaml:"certificates"`
}

// Certificate represents a single certificate entry
type Certificate struct {
	CertFile string `yaml:"certFile"`
	KeyFile  string `yaml:"keyFile"`
}

// TraefikConfigGenerator generates Traefik YAML configuration files
type TraefikConfigGenerator struct {
	configDir        string
	certDirContainer string // Path as seen inside Traefik container
}

// NewTraefikConfigGenerator creates a new Traefik config generator
// certDirContainer is the path where certs are mounted inside the Traefik container
func NewTraefikConfigGenerator(configDir, certDirContainer string) *TraefikConfigGenerator {
	return &TraefikConfigGenerator{
		configDir:        configDir,
		certDirContainer: certDirContainer,
	}
}

// Generate creates or updates a Traefik configuration file with the certificate
// certFile and keyFile should be the host paths (will be converted to container paths)
func (g *TraefikConfigGenerator) Generate(hostname, certFile, keyFile string) error {
	// Convert host paths to container paths
	// Extract just the filename and use container directory
	certFileName := filepath.Base(certFile)
	keyFileName := filepath.Base(keyFile)

	containerCertPath := filepath.Join(g.certDirContainer, certFileName)
	containerKeyPath := filepath.Join(g.certDirContainer, keyFileName)

	return g.generateWithPaths(hostname, containerCertPath, containerKeyPath)
}

// generateWithPaths creates the config with the given paths
func (g *TraefikConfigGenerator) generateWithPaths(hostname, certPath, keyPath string) error {
	// Ensure config directory exists
	if err := os.MkdirAll(g.configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	configFile := filepath.Join(g.configDir, fmt.Sprintf("%s.yml", hostname))

	// Check if config already exists and has this cert
	if g.configExists(configFile, certPath, keyPath) {
		log.Printf("Traefik config already exists for %s, skipping", hostname)
		return nil
	}

	// Create config
	config := &TraefikConfig{
		TLS: &TLSConfig{
			Certificates: []Certificate{
				{
					CertFile: certPath,
					KeyFile:  keyPath,
				},
			},
		},
	}

	// Write YAML file
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal YAML: %w", err)
	}

	if err := os.WriteFile(configFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	log.Printf("Generated Traefik config: %s", configFile)
	return nil
}

// configExists checks if the config file already exists with the given cert
func (g *TraefikConfigGenerator) configExists(configFile, certFile, keyFile string) bool {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return false
	}

	var config TraefikConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return false
	}

	if config.TLS == nil {
		return false
	}

	// Check if cert already in config
	for _, cert := range config.TLS.Certificates {
		if cert.CertFile == certFile && cert.KeyFile == keyFile {
			return true
		}
	}

	return false
}
