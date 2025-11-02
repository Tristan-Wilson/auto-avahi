package avahi

import (
	"fmt"
	"log"
	"os/exec"
	"sync"
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
		if err := p.stopProcess(existing); err != nil {
			log.Printf("Warning: failed to stop old process: %v", err)
		}
	}

	// Start new avahi-publish process
	// Using -R flag to avoid reverse PTR collision
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

	// Monitor process in background
	go p.monitorProcess(proc)

	return nil
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

	if err := p.stopProcess(proc); err != nil {
		return err
	}

	delete(p.processes, containerID)
	return nil
}

// stopProcess kills a running avahi-publish process
func (p *Publisher) stopProcess(proc *Process) error {
	if proc.Cmd == nil || proc.Cmd.Process == nil {
		return nil
	}

	if err := proc.Cmd.Process.Kill(); err != nil {
		return fmt.Errorf("failed to kill process: %w", err)
	}

	// Wait for process to exit
	_ = proc.Cmd.Wait()

	return nil
}

// monitorProcess watches a process and removes it if it exits unexpectedly
func (p *Publisher) monitorProcess(proc *Process) {
	if err := proc.Cmd.Wait(); err != nil {
		log.Printf("avahi-publish for %s exited with error: %v", proc.Hostname, err)
	} else {
		log.Printf("avahi-publish for %s exited", proc.Hostname)
	}

	// Remove from map
	p.mu.Lock()
	delete(p.processes, proc.ContainerID)
	p.mu.Unlock()
}

// Shutdown stops all running avahi-publish processes
func (p *Publisher) Shutdown() {
	p.mu.Lock()
	defer p.mu.Unlock()

	log.Println("Shutting down all avahi-publish processes...")

	for containerID, proc := range p.processes {
		log.Printf("Stopping publish for %s", proc.Hostname)
		if err := p.stopProcess(proc); err != nil {
			log.Printf("Error stopping process: %v", err)
		}
		delete(p.processes, containerID)
	}
}
