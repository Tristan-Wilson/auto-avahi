package avahi

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"sync"
	"time"
)

// Publisher manages avahi-publish processes
type Publisher struct {
	hostIP    string
	processes map[string]*Process // containerID -> Process
	mu        sync.RWMutex
}

// Process represents a running avahi-publish process
type Process struct {
	ContainerID string
	Hostname    string
	Cmd         *exec.Cmd
}

// NewPublisher creates a new Avahi publisher
func NewPublisher(hostIP string) *Publisher {
	return &Publisher{
		hostIP:    hostIP,
		processes: make(map[string]*Process),
	}
}

// Publish starts avahi-publish for a hostname associated with a container
func (p *Publisher) Publish(containerID, hostname string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check if already publishing
	if existing, ok := p.processes[containerID]; ok {
		if existing.Hostname == hostname {
			log.Printf("Already publishing %s for container %s", hostname, containerID[:12])
			return nil
		}
		// Hostname changed, stop old process
		log.Printf("Hostname changed for container %s, stopping old publish", containerID[:12])
		p.killProcess(existing)
	}

	return p.startProcess(containerID, hostname)
}

// Unpublish stops the avahi-publish process for a container
func (p *Publisher) Unpublish(containerID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	proc, ok := p.processes[containerID]
	if !ok {
		return nil // Not publishing, nothing to do
	}

	log.Printf("Unpublishing %s for container %s", proc.Hostname, containerID[:12])
	p.killProcess(proc)
	delete(p.processes, containerID)
	return nil
}

// RefreshAll kills all avahi-publish processes and restarts them.
// This handles the case where avahi-daemon restarts and existing
// registrations become stale.
func (p *Publisher) RefreshAll() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.processes) == 0 {
		return
	}

	log.Printf("Refreshing all %d avahi-publish registrations...", len(p.processes))

	type entry struct {
		containerID string
		hostname    string
	}
	var entries []entry
	for containerID, proc := range p.processes {
		entries = append(entries, entry{containerID, proc.Hostname})
		p.killProcess(proc)
	}

	// Clear the map before re-publishing
	for k := range p.processes {
		delete(p.processes, k)
	}

	for _, e := range entries {
		if err := p.startProcess(e.containerID, e.hostname); err != nil {
			log.Printf("Failed to re-publish %s: %v", e.hostname, err)
		}
	}
}

// CheckHealth verifies that avahi-publish registrations are working
// by resolving a published hostname. Returns true if healthy.
func (p *Publisher) CheckHealth() bool {
	p.mu.RLock()
	var hostname string
	for _, proc := range p.processes {
		hostname = proc.Hostname
		break
	}
	p.mu.RUnlock()

	if hostname == "" {
		return true // No registrations to check
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "avahi-resolve", "-n", hostname)
	if err := cmd.Run(); err != nil {
		log.Printf("Health check: cannot resolve %s: %v", hostname, err)
		return false
	}
	return true
}

// startProcess starts an avahi-publish process and adds it to the map.
// Caller must hold p.mu write lock.
func (p *Publisher) startProcess(containerID, hostname string) error {
	cmd := exec.Command("avahi-publish", "-a", "-R", hostname, p.hostIP)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start avahi-publish: %w", err)
	}

	proc := &Process{
		ContainerID: containerID,
		Hostname:    hostname,
		Cmd:         cmd,
	}

	p.processes[containerID] = proc
	log.Printf("Publishing %s for container %s (PID: %d)", hostname, containerID[:12], cmd.Process.Pid)

	go p.monitorProcess(proc)

	return nil
}

// killProcess sends SIGKILL to an avahi-publish process.
// Does not wait or remove from map — monitorProcess handles cleanup.
func (p *Publisher) killProcess(proc *Process) {
	if proc.Cmd != nil && proc.Cmd.Process != nil {
		proc.Cmd.Process.Kill()
	}
}

// monitorProcess watches a process and removes it if it exits unexpectedly.
// Only removes the map entry if the process is still the current one for
// its containerID — this prevents clobbering entries after RefreshAll.
func (p *Publisher) monitorProcess(proc *Process) {
	if err := proc.Cmd.Wait(); err != nil {
		log.Printf("avahi-publish for %s exited with error: %v", proc.Hostname, err)
	} else {
		log.Printf("avahi-publish for %s exited", proc.Hostname)
	}

	p.mu.Lock()
	if current, ok := p.processes[proc.ContainerID]; ok && current == proc {
		delete(p.processes, proc.ContainerID)
	}
	p.mu.Unlock()
}

// Shutdown stops all running avahi-publish processes
func (p *Publisher) Shutdown() {
	p.mu.Lock()
	defer p.mu.Unlock()

	log.Println("Shutting down all avahi-publish processes...")

	for containerID, proc := range p.processes {
		log.Printf("Stopping publish for %s", proc.Hostname)
		p.killProcess(proc)
		delete(p.processes, containerID)
	}
}
