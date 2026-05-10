package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Tristan-Wilson/auto-avahi/avahi"
	"github.com/Tristan-Wilson/auto-avahi/certs"
	"github.com/Tristan-Wilson/auto-avahi/config"
	"github.com/Tristan-Wilson/auto-avahi/docker"
	"github.com/Tristan-Wilson/auto-avahi/hostname"
)

// App is the main application
type App struct {
	cfg              *config.Config
	hostnameOpts     hostname.Options
	publisher        *avahi.Publisher // nil when MDNSEnable is false
	certGenerator    *certs.Generator
	traefikGenerator *certs.TraefikConfigGenerator
}

// NewApp creates a new application instance
func NewApp(cfg *config.Config) *App {
	app := &App{
		cfg: cfg,
		hostnameOpts: hostname.Options{
			Suffixes:          cfg.HostnameSuffixes,
			EnforceDepthLimit: cfg.MDNSEnable,
		},
		certGenerator:    certs.NewGenerator(cfg.CertDir, cfg.CARoot),
		traefikGenerator: certs.NewTraefikConfigGenerator(cfg.TraefikConfigDir, cfg.CertDirContainer),
	}
	if cfg.MDNSEnable {
		app.publisher = avahi.NewPublisher(cfg.HostIP)
	}
	return app
}

// OnContainerStart handles container start events
func (a *App) OnContainerStart(containerID string, labels map[string]string) error {
	// Extract hostnames from Traefik labels
	hostnames := docker.ExtractHostnames(labels)
	if len(hostnames) == 0 {
		log.Printf("Container %s: no Traefik Host rules found", containerID[:12])
		return nil
	}

	log.Printf("Container %s: found hostnames: %v", containerID[:12], hostnames)

	// Validate and filter against the configured suffixes
	validHostnames, warnings := hostname.Extract(hostnames, a.hostnameOpts)

	for _, warning := range warnings {
		log.Printf("Container %s: WARNING: %s", containerID[:12], warning)
	}

	if len(validHostnames) == 0 {
		log.Printf("Container %s: no hostnames matched configured suffixes (%s)",
			containerID[:12], strings.Join(a.hostnameOpts.Suffixes, ", "))
		return nil
	}

	// Process first valid hostname (could be extended to handle multiple)
	selectedHostname := validHostnames[0]
	if len(validHostnames) > 1 {
		log.Printf("Container %s: multiple valid hostnames found, using first: %s", containerID[:12], selectedHostname)
	}

	// Generate certificate if it doesn't exist
	if err := a.certGenerator.Generate(selectedHostname); err != nil {
		log.Printf("Container %s: failed to generate certificate for %s: %v", containerID[:12], selectedHostname, err)
		return err
	}

	// Verify certificate is valid before creating Traefik config
	if err := a.certGenerator.ValidateCert(selectedHostname); err != nil {
		log.Printf("Container %s: certificate validation failed for %s: %v", containerID[:12], selectedHostname, err)
		return err
	}

	// Generate Traefik config
	certFile, keyFile := a.certGenerator.GetCertPaths(selectedHostname)
	if err := a.traefikGenerator.Generate(selectedHostname, certFile, keyFile); err != nil {
		log.Printf("Container %s: failed to generate Traefik config for %s: %v", containerID[:12], selectedHostname, err)
		return err
	}

	if a.publisher != nil {
		if err := a.publisher.Publish(containerID, selectedHostname); err != nil {
			log.Printf("Container %s: failed to publish %s: %v", containerID[:12], selectedHostname, err)
			return err
		}
	}

	log.Printf("Container %s: successfully configured %s", containerID[:12], selectedHostname)
	return nil
}

// OnContainerStop handles container stop events
func (a *App) OnContainerStop(containerID string) error {
	if a.publisher == nil {
		return nil
	}
	return a.publisher.Unpublish(containerID)
}

// Shutdown cleans up resources
func (a *App) Shutdown() {
	if a.publisher != nil {
		a.publisher.Shutdown()
	}
}

func main() {
	log.Println("Starting auto-avahi daemon...")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	log.Printf("Configuration: HostnameSuffixes=%v, MDNSEnable=%t, HostIP=%s, CertDir=%s, CertDirContainer=%s, TraefikConfigDir=%s",
		cfg.HostnameSuffixes, cfg.MDNSEnable, cfg.HostIP, cfg.CertDir, cfg.CertDirContainer, cfg.TraefikConfigDir)

	if cfg.CARoot != "" {
		log.Printf("mkcert CA root: %s (from CAROOT env)", cfg.CARoot)
	} else {
		log.Printf("mkcert CA root: not configured, using default for current user")
	}

	// Create application
	app := NewApp(cfg)
	defer app.Shutdown()

	// Create Docker watcher
	watcher, err := docker.NewWatcher(app)
	if err != nil {
		log.Fatalf("Failed to create Docker watcher: %v", err)
	}
	defer watcher.Close()

	// Set up context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start watching in a goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- watcher.Watch(ctx)
	}()

	// Start mDNS health check loop — detects and recovers from avahi-daemon restarts.
	// Only runs when mDNS publishing is enabled.
	if app.publisher != nil {
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if !app.publisher.CheckHealth() {
						log.Println("Detected stale mDNS registrations (avahi-daemon likely restarted), refreshing all...")
						app.publisher.RefreshAll()
					}
				}
			}
		}()
	}

	log.Println("Watching Docker events... (Press Ctrl+C to stop)")

	// Wait for signal or error
	select {
	case <-sigChan:
		log.Println("Received shutdown signal")
		cancel()
	case err := <-errChan:
		if err != nil {
			log.Fatalf("Watcher error: %v", err)
		}
	}

	log.Println("Shutting down...")
}
