package testutil

import (
	"context"

	"3tcapital/ms_facturacion_core/internal/core/event"
	"3tcapital/ms_facturacion_core/internal/core/invoice"
	"3tcapital/ms_facturacion_core/internal/core/resolution"
)

// MockProvider is a mock implementation of invoice.Provider for testing.
type MockProvider struct {
	GetResolutionsFunc       func(ctx context.Context, nit string) ([]resolution.Resolution, error)
	GetDocumentsFunc         func(ctx context.Context, query invoice.DocumentQuery) ([]invoice.Document, error)
	GetDocumentByNumberFunc  func(ctx context.Context, query invoice.DocumentByNumberQuery) ([]invoice.Document, error)
	GetReceivedDocumentsFunc func(ctx context.Context, query invoice.DocumentQuery) ([]invoice.Document, error)
	RegisterEventFunc        func(ctx context.Context, evt event.Event, emisorNit, razonSocial string) (*invoice.EventRegistrationResult, error)
	RegisterDocumentFunc     func(ctx context.Context, req invoice.DocumentRegistrationRequest) (*invoice.DocumentRegistrationResponse, error)
}

// GetResolutions calls the mock function if set, otherwise returns empty slice.
func (m *MockProvider) GetResolutions(ctx context.Context, nit string) ([]resolution.Resolution, error) {
	if m.GetResolutionsFunc != nil {
		return m.GetResolutionsFunc(ctx, nit)
	}
	return []resolution.Resolution{}, nil
}

// GetDocuments calls the mock function if set, otherwise returns empty slice.
func (m *MockProvider) GetDocuments(ctx context.Context, query invoice.DocumentQuery) ([]invoice.Document, error) {
	if m.GetDocumentsFunc != nil {
		return m.GetDocumentsFunc(ctx, query)
	}
	return []invoice.Document{}, nil
}

// GetDocumentByNumber calls the mock function if set, otherwise returns empty slice.
func (m *MockProvider) GetDocumentByNumber(ctx context.Context, query invoice.DocumentByNumberQuery) ([]invoice.Document, error) {
	if m.GetDocumentByNumberFunc != nil {
		return m.GetDocumentByNumberFunc(ctx, query)
	}
	return []invoice.Document{}, nil
}

// RegisterEvent calls the mock function if set, otherwise returns nil result and error.
func (m *MockProvider) RegisterEvent(ctx context.Context, evt event.Event, emisorNit, razonSocial string) (*invoice.EventRegistrationResult, error) {
	if m.RegisterEventFunc != nil {
		return m.RegisterEventFunc(ctx, evt, emisorNit, razonSocial)
	}
	return nil, nil
}

// RegisterDocument calls the mock function if set, otherwise returns nil result and error.
func (m *MockProvider) RegisterDocument(ctx context.Context, req invoice.DocumentRegistrationRequest) (*invoice.DocumentRegistrationResponse, error) {
	if m.RegisterDocumentFunc != nil {
		return m.RegisterDocumentFunc(ctx, req)
	}
	return nil, nil
}

// GetReceivedDocuments calls the mock function if set, otherwise returns empty slice.
func (m *MockProvider) GetReceivedDocuments(ctx context.Context, query invoice.DocumentQuery) ([]invoice.Document, error) {
	if m.GetReceivedDocumentsFunc != nil {
		return m.GetReceivedDocumentsFunc(ctx, query)
	}
	return []invoice.Document{}, nil
}

// Ensure MockProvider implements invoice.Provider interface.
var _ invoice.Provider = (*MockProvider)(nil)
