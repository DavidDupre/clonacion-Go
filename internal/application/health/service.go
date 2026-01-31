package health

import (
	"context"
	"time"

	corehealth "3tcapital/ms_facturacion_core/internal/core/health"
)

// Metadata contains immutable metadata about the running service.
type Metadata struct {
	Service     string
	Version     string
	Environment string
}

// Service exposes health-check use cases to adapters.
type Service struct {
	meta      Metadata
	startedAt time.Time
}

func NewService(meta Metadata) *Service {
	return &Service{
		meta:      meta,
		startedAt: time.Now().UTC(),
	}
}

// Status returns the current availability snapshot.
func (s *Service) Status(_ context.Context) corehealth.Status {
	uptime := time.Since(s.startedAt)
	return corehealth.Status{
		Service:     s.meta.Service,
		Version:     s.meta.Version,
		Environment: s.meta.Environment,
		Status:      "UP",
		StartedAt:   s.startedAt,
		Uptime:      uptime.String(),
		UptimeSecs:  int64(uptime.Seconds()),
	}
}
