package services

import (
	"testing"

	"mcloud/repositories"
)

func TestNewContainerInitializesServicesAndRegistersCleanup(t *testing.T) {
	previous := defaultCleanupService
	defer SetCleanupService(previous)

	SetCleanupService(nil)
	container := NewContainer(repositories.Container{})

	if container == nil {
		t.Fatalf("expected container instance")
	}
	if container.Auth == nil || container.User == nil || container.Folder == nil || container.File == nil || container.RecycleBin == nil || container.Cleanup == nil {
		t.Fatalf("expected all services to be initialized")
	}
	if defaultCleanupService != container.Cleanup {
		t.Fatalf("expected cleanup service to be registered as default")
	}
}

func TestStartCleanupWorkersNoopWhenServiceMissing(t *testing.T) {
	previous := defaultCleanupService
	defer SetCleanupService(previous)

	SetCleanupService(nil)
	StartCleanupWorkers()
}
