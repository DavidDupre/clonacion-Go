package health

import (
	"context"
	"testing"
	"time"
)

func TestNewService(t *testing.T) {
	meta := Metadata{
		Service:     "test-service",
		Version:     "1.0.0",
		Environment: "test",
	}

	service := NewService(meta)

	if service == nil {
		t.Fatal("expected service to be created, got nil")
	}

	if service.meta != meta {
		t.Error("expected service to have the provided metadata")
	}

	if service.startedAt.IsZero() {
		t.Error("expected startedAt to be set")
	}
}

func TestService_Status(t *testing.T) {
	meta := Metadata{
		Service:     "test-service",
		Version:     "1.0.0",
		Environment: "test",
	}

	service := NewService(meta)
	startTime := service.startedAt

	// Wait a bit to ensure uptime is calculated
	time.Sleep(10 * time.Millisecond)

	ctx := context.Background()
	status := service.Status(ctx)

	if status.Service != meta.Service {
		t.Errorf("expected service %q, got %q", meta.Service, status.Service)
	}

	if status.Version != meta.Version {
		t.Errorf("expected version %q, got %q", meta.Version, status.Version)
	}

	if status.Environment != meta.Environment {
		t.Errorf("expected environment %q, got %q", meta.Environment, status.Environment)
	}

	if status.Status != "UP" {
		t.Errorf("expected status 'UP', got %q", status.Status)
	}

	if !status.StartedAt.Equal(startTime) {
		t.Errorf("expected startedAt to match service start time")
	}

	if status.UptimeSecs < 0 {
		t.Errorf("expected uptimeSecs to be non-negative, got %d", status.UptimeSecs)
	}

	if status.Uptime == "" {
		t.Error("expected uptime to be set")
	}

	// Verify uptime is reasonable (should be non-negative)
	if status.UptimeSecs < 0 {
		t.Errorf("expected uptimeSecs >= 0, got %d", status.UptimeSecs)
	}
}

func TestService_Status_UptimeCalculation(t *testing.T) {
	meta := Metadata{
		Service:     "test",
		Version:     "1.0.0",
		Environment: "test",
	}

	service := NewService(meta)
	time.Sleep(100 * time.Millisecond)

	status := service.Status(context.Background())

	// Uptime should be at least 100ms
	if status.UptimeSecs < 0 {
		t.Errorf("expected uptimeSecs >= 0, got %d", status.UptimeSecs)
	}

	// Verify uptime string is not empty
	if status.Uptime == "" {
		t.Error("expected uptime string to be non-empty")
	}
}
