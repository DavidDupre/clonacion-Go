package resolution

import (
	"context"
	"fmt"

	"3tcapital/ms_facturacion_core/internal/core/invoice"
	coreresolution "3tcapital/ms_facturacion_core/internal/core/resolution"
)

// Service orchestrates resolution-related use cases.
type Service struct {
	provider invoice.Provider
}

// NewService creates a new resolution service with the given invoice provider.
func NewService(provider invoice.Provider) *Service {
	return &Service{
		provider: provider,
	}
}

// GetResolutions retrieves all active resolutions for a given NIT.
func (s *Service) GetResolutions(ctx context.Context, nit string) ([]coreresolution.Resolution, error) {
	if nit == "" {
		return nil, fmt.Errorf("nit is required")
	}

	// Basic NIT validation: should be numeric and have reasonable length
	if len(nit) < 9 || len(nit) > 15 {
		return nil, fmt.Errorf("invalid nit format: must be between 9 and 15 characters")
	}

	resolutions, err := s.provider.GetResolutions(ctx, nit)
	if err != nil {
		// Preserve the original error message for better error handling
		return nil, err
	}

	return resolutions, nil
}
