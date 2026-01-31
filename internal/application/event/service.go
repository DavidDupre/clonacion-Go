package event

import (
	"context"
	"fmt"

	"3tcapital/ms_facturacion_core/internal/core/event"
	"3tcapital/ms_facturacion_core/internal/core/invoice"
)

// Service orchestrates event registration use cases.
type Service struct {
	provider   invoice.Provider
	emisorNit  string
	razonSocial string
}

// NewService creates a new event service with the given invoice provider.
func NewService(provider invoice.Provider, emisorNit, razonSocial string) *Service {
	return &Service{
		provider:    provider,
		emisorNit:   emisorNit,
		razonSocial: razonSocial,
	}
}

// RegisterEvent registers a Radian event for a document.
func (s *Service) RegisterEvent(ctx context.Context, evt event.Event) (*invoice.EventRegistrationResult, error) {
	// Validate event
	if err := evt.Validate(); err != nil {
		return nil, fmt.Errorf("validation error: %w", err)
	}

	// Validate configuration
	if s.emisorNit == "" {
		return nil, fmt.Errorf("emisor nit is not configured")
	}

	if s.razonSocial == "" {
		return nil, fmt.Errorf("razon social is not configured")
	}

	// Register event through provider
	result, err := s.provider.RegisterEvent(ctx, evt, s.emisorNit, s.razonSocial)
	if err != nil {
		return nil, fmt.Errorf("provider error: %w", err)
	}

	return result, nil
}
