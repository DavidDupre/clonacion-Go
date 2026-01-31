package testutil

import (
	"context"

	"3tcapital/ms_facturacion_core/internal/core/resolution"
)

// MockResolutionService is a mock implementation of resolution service for testing.
type MockResolutionService struct {
	GetResolutionsFunc func(ctx context.Context, nit string) ([]resolution.Resolution, error)
}

// GetResolutions calls the mock function if set, otherwise returns empty slice.
func (m *MockResolutionService) GetResolutions(ctx context.Context, nit string) ([]resolution.Resolution, error) {
	if m.GetResolutionsFunc != nil {
		return m.GetResolutionsFunc(ctx, nit)
	}
	return []resolution.Resolution{}, nil
}
