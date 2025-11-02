package certs

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// Generator handles certificate generation using mkcert
type Generator struct {
	certDir string
}

// NewGenerator creates a new certificate generator
func NewGenerator(certDir string) *Generator {
	return &Generator{
		certDir: certDir,
	}
}

// CertExists checks if a certificate already exists for the given hostname
func (g *Generator) CertExists(hostname string) bool {
	certFile := g.getCertPath(hostname)
	keyFile := g.getKeyPath(hostname)

	_, certErr := os.Stat(certFile)
	_, keyErr := os.Stat(keyFile)

	return certErr == nil && keyErr == nil
}

// Generate generates a certificate for the given hostname using mkcert
func (g *Generator) Generate(hostname string) error {
	if g.CertExists(hostname) && g.ValidateCert(hostname) == nil {
		log.Printf("Certificate already exists for %s, skipping generation", hostname)
		return nil
	}

	// Ensure cert directory exists
	if err := os.MkdirAll(g.certDir, 0755); err != nil {
		return fmt.Errorf("failed to create cert directory: %w", err)
	}

	// Run mkcert to generate certificate
	certFile := g.getCertPath(hostname)
	keyFile := g.getKeyPath(hostname)

	log.Printf("Generating certificate for %s...", hostname)

	cmd := exec.Command("mkcert", "-cert-file", certFile, "-key-file", keyFile, hostname)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mkcert failed for %s: %w", hostname, err)
	}

	// Wait for certificate files to be fully written and validate
	maxAttempts := 10
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := g.ValidateCert(hostname); err == nil {
			log.Printf("Certificate generated and validated: %s", certFile)
			return nil
		}

		if attempt < maxAttempts {
			log.Printf("Certificate not yet valid (attempt %d/%d), waiting...", attempt, maxAttempts)
			time.Sleep(100 * time.Millisecond)
		}
	}

	return fmt.Errorf("certificate generation completed but files are not valid PEM after %d attempts", maxAttempts)
}

// GetCertPaths returns the certificate and key file paths for a hostname
func (g *Generator) GetCertPaths(hostname string) (certFile, keyFile string) {
	return g.getCertPath(hostname), g.getKeyPath(hostname)
}

func (g *Generator) getCertPath(hostname string) string {
	return filepath.Join(g.certDir, fmt.Sprintf("%s.pem", hostname))
}

func (g *Generator) getKeyPath(hostname string) string {
	return filepath.Join(g.certDir, fmt.Sprintf("%s-key.pem", hostname))
}

// ValidateCert checks if a certificate file contains valid PEM data
func (g *Generator) ValidateCert(hostname string) error {
	certFile := g.getCertPath(hostname)
	keyFile := g.getKeyPath(hostname)

	// Check cert file
	certData, err := os.ReadFile(certFile)
	if err != nil {
		return fmt.Errorf("failed to read cert file: %w", err)
	}

	if len(certData) == 0 {
		return fmt.Errorf("cert file is empty")
	}

	// Parse PEM block
	block, _ := pem.Decode(certData)
	if block == nil {
		return fmt.Errorf("failed to decode PEM block from cert file")
	}

	// Parse certificate
	_, err = x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %w", err)
	}

	// Check key file
	keyData, err := os.ReadFile(keyFile)
	if err != nil {
		return fmt.Errorf("failed to read key file: %w", err)
	}

	if len(keyData) == 0 {
		return fmt.Errorf("key file is empty")
	}

	// Parse PEM block for key
	keyBlock, _ := pem.Decode(keyData)
	if keyBlock == nil {
		return fmt.Errorf("failed to decode PEM block from key file")
	}

	return nil
}
