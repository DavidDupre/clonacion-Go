package invoice

import (
	"context"

	"3tcapital/ms_facturacion_core/internal/core/event"
	"3tcapital/ms_facturacion_core/internal/core/resolution"
)

// EventRegistrationResult represents the result of registering an event.
type EventRegistrationResult struct {
	Code            string
	NumeroDocumento string
	Resultado       []EventResult
	MensajeError    string
}

// EventResult represents the result for a single event type.
type EventResult struct {
	TipoEvento      string
	Mensaje         string
	MensajeError    string
	CodigoRespuesta string
}

// Provider defines the interface for invoice service providers.
// This abstraction allows the system to work with multiple providers
// (Numrot, and future providers) without coupling to specific implementations.
type Provider interface {
	// GetResolutions retrieves all active resolutions for a given NIT.
	// Returns an error if the provider is unavailable or the NIT is invalid.
	GetResolutions(ctx context.Context, nit string) ([]resolution.Resolution, error)
	// GetDocuments retrieves documents/invoices for a given query.
	// Returns an error if the provider is unavailable or the query is invalid.
	GetDocuments(ctx context.Context, query DocumentQuery) ([]Document, error)
	// GetDocumentByNumber retrieves a document/invoice by document number.
	// Returns an error if the provider is unavailable or the query is invalid.
	GetDocumentByNumber(ctx context.Context, query DocumentByNumberQuery) ([]Document, error)
	// GetReceivedDocuments retrieves received documents/invoices for a given query.
	// Returns an error if the provider is unavailable or the query is invalid.
	GetReceivedDocuments(ctx context.Context, query DocumentQuery) ([]Document, error)
	// RegisterEvent registers a Radian event for a document.
	// Returns the registration result or an error if the registration fails.
	RegisterEvent(ctx context.Context, evt event.Event, emisorNit, razonSocial string) (*EventRegistrationResult, error)
	// RegisterDocument registers a document (invoice, credit note, or debit note) with the provider.
	// Returns the registration response with processed and failed documents or an error if the registration fails.
	RegisterDocument(ctx context.Context, req DocumentRegistrationRequest) (*DocumentRegistrationResponse, error)
}
