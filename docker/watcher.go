package docker

import (
	"context"
	"io"
	"log"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/client"
)

// EventHandler handles Docker container events
type EventHandler interface {
	OnContainerStart(containerID string, labels map[string]string) error
	OnContainerStop(containerID string) error
}

// Watcher watches Docker events and calls handlers
type Watcher struct {
	client  *client.Client
	handler EventHandler
}

// NewWatcher creates a new Docker event watcher
func NewWatcher(handler EventHandler) (*Watcher, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	return &Watcher{
		client:  cli,
		handler: handler,
	}, nil
}

// Watch starts watching Docker events
func (w *Watcher) Watch(ctx context.Context) error {
	// First, process all running containers
	if err := w.syncRunningContainers(ctx); err != nil {
		log.Printf("Warning: failed to sync running containers: %v", err)
	}

	// Then watch for events
	eventsChan, errChan := w.client.Events(ctx, types.EventsOptions{})

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errChan:
			if err != nil && err != io.EOF {
				return err
			}
			return nil
		case event := <-eventsChan:
			if err := w.handleEvent(ctx, event); err != nil {
				log.Printf("Error handling event: %v", err)
			}
		}
	}
}

// syncRunningContainers processes all currently running containers
func (w *Watcher) syncRunningContainers(ctx context.Context) error {
	containers, err := w.client.ContainerList(ctx, types.ContainerListOptions{})
	if err != nil {
		return err
	}

	for _, container := range containers {
		log.Printf("Syncing existing container: %s", container.ID[:12])
		if err := w.handler.OnContainerStart(container.ID, container.Labels); err != nil {
			log.Printf("Error syncing container %s: %v", container.ID[:12], err)
		}
	}

	return nil
}

// handleEvent processes a single Docker event
func (w *Watcher) handleEvent(ctx context.Context, event events.Message) error {
	if event.Type != "container" {
		return nil
	}

	switch event.Action {
	case "start":
		// Get container details to access labels
		inspect, err := w.client.ContainerInspect(ctx, event.Actor.ID)
		if err != nil {
			return err
		}

		log.Printf("Container started: %s", event.Actor.ID[:12])
		return w.handler.OnContainerStart(event.Actor.ID, inspect.Config.Labels)

	case "die", "stop", "kill":
		log.Printf("Container stopped: %s", event.Actor.ID[:12])
		return w.handler.OnContainerStop(event.Actor.ID)
	}

	return nil
}

// Close closes the Docker client
func (w *Watcher) Close() error {
	return w.client.Close()
}
