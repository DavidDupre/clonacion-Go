package event

import (
	"context"
	"errors"
	"testing"
	"time"

	"3tcapital/ms_facturacion_core/internal/core/event"
	"3tcapital/ms_facturacion_core/internal/core/invoice"
	"3tcapital/ms_facturacion_core/internal/testutil"
)

func TestNewService(t *testing.T) {
	mockProvider := &testutil.MockProvider{}
	emisorNit := "123456789"
	razonSocial := "PROVEDOR NOMBRE"

	service := NewService(mockProvider, emisorNit, razonSocial)

	if service == nil {
		t.Fatal("expected service to be created, got nil")
	}

	if service.provider != mockProvider {
		t.Error("expected service to have the provided provider")
	}

	if service.emisorNit != emisorNit {
		t.Errorf("expected emisorNit to be %q, got %q", emisorNit, service.emisorNit)
	}

	if service.razonSocial != razonSocial {
		t.Errorf("expected razonSocial to be %q, got %q", razonSocial, service.razonSocial)
	}
}

func TestService_RegisterEvent(t *testing.T) {
	validRejectionCode := event.RejectionCodeInconsistencias

	tests := []struct {
		name          string
		evt           event.Event
		emisorNit     string
		razonSocial   string
		setupProvider func() invoice.Provider
		expectedErr   string
		expectedCode  string
	}{
		{
			name:        "valid ACUSE event - success",
			emisorNit:   "123456789",
			razonSocial: "PROVEDOR NOMBRE",
			evt: event.Event{
				EventType:              event.EventTypeAcuse,
				DocumentNumber:         "FAC12345",
				NombreGenerador:        "Mauricio",
				ApellidoGenerador:      "Alemán",
				IdentificacionGenerador: "1061239585",
				EventGenerationDate:    time.Now(),
			},
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{
					RegisterEventFunc: func(ctx context.Context, evt event.Event, emisorNit, razonSocial string) (*invoice.EventRegistrationResult, error) {
						return &invoice.EventRegistrationResult{
							Code:            "1000",
							NumeroDocumento: "FAC12345",
							Resultado: []invoice.EventResult{
								{
									TipoEvento:      "030",
									Mensaje:         "Procesado Correctamente.",
									MensajeError:    "",
									CodigoRespuesta: "1000",
								},
							},
							MensajeError: "",
						}, nil
					},
				}
			},
			expectedCode: "1000",
		},
		{
			name:        "valid RECLAMO event with rejection code - success",
			emisorNit:   "123456789",
			razonSocial: "PROVEDOR NOMBRE",
			evt: event.Event{
				EventType:              event.EventTypeReclamo,
				DocumentNumber:         "FAC12345",
				NombreGenerador:        "Mauricio",
				ApellidoGenerador:      "Alemán",
				IdentificacionGenerador: "1061239585",
				RejectionCode:          &validRejectionCode,
				EventGenerationDate:    time.Now(),
			},
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{
					RegisterEventFunc: func(ctx context.Context, evt event.Event, emisorNit, razonSocial string) (*invoice.EventRegistrationResult, error) {
						return &invoice.EventRegistrationResult{
							Code:            "1000",
							NumeroDocumento: "FAC12345",
							Resultado: []invoice.EventResult{
								{
									TipoEvento:      "031",
									Mensaje:         "Procesado Correctamente.",
									MensajeError:    "",
									CodigoRespuesta: "1000",
								},
							},
							MensajeError: "",
						}, nil
					},
				}
			},
			expectedCode: "1000",
		},
		{
			name:        "event validation error - missing document number",
			emisorNit:   "123456789",
			razonSocial: "PROVEDOR NOMBRE",
			evt: event.Event{
				EventType:              event.EventTypeAcuse,
				DocumentNumber:         "",
				NombreGenerador:        "Mauricio",
				ApellidoGenerador:      "Alemán",
				IdentificacionGenerador: "1061239585",
				EventGenerationDate:    time.Now(),
			},
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{}
			},
			expectedErr: "validation error",
		},
		{
			name:        "event validation error - invalid event type",
			emisorNit:   "123456789",
			razonSocial: "PROVEDOR NOMBRE",
			evt: event.Event{
				EventType:              event.EventType("INVALID"),
				DocumentNumber:         "FAC12345",
				NombreGenerador:        "Mauricio",
				ApellidoGenerador:      "Alemán",
				IdentificacionGenerador: "1061239585",
				EventGenerationDate:    time.Now(),
			},
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{}
			},
			expectedErr: "validation error",
		},
		{
			name:        "event validation error - RECLAMO without rejection code",
			emisorNit:   "123456789",
			razonSocial: "PROVEDOR NOMBRE",
			evt: event.Event{
				EventType:              event.EventTypeReclamo,
				DocumentNumber:         "FAC12345",
				NombreGenerador:        "Mauricio",
				ApellidoGenerador:      "Alemán",
				IdentificacionGenerador: "1061239585",
				RejectionCode:          nil,
				EventGenerationDate:    time.Now(),
			},
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{}
			},
			expectedErr: "validation error",
		},
		{
			name:        "missing emisor nit configuration",
			emisorNit:   "",
			razonSocial: "PROVEDOR NOMBRE",
			evt: event.Event{
				EventType:              event.EventTypeAcuse,
				DocumentNumber:         "FAC12345",
				NombreGenerador:        "Mauricio",
				ApellidoGenerador:      "Alemán",
				IdentificacionGenerador: "1061239585",
				EventGenerationDate:    time.Now(),
			},
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{}
			},
			expectedErr: "emisor nit is not configured",
		},
		{
			name:        "missing razon social configuration",
			emisorNit:   "123456789",
			razonSocial: "",
			evt: event.Event{
				EventType:              event.EventTypeAcuse,
				DocumentNumber:         "FAC12345",
				NombreGenerador:        "Mauricio",
				ApellidoGenerador:      "Alemán",
				IdentificacionGenerador: "1061239585",
				EventGenerationDate:    time.Now(),
			},
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{}
			},
			expectedErr: "razon social is not configured",
		},
		{
			name:        "provider error",
			emisorNit:   "123456789",
			razonSocial: "PROVEDOR NOMBRE",
			evt: event.Event{
				EventType:              event.EventTypeAcuse,
				DocumentNumber:         "FAC12345",
				NombreGenerador:        "Mauricio",
				ApellidoGenerador:      "Alemán",
				IdentificacionGenerador: "1061239585",
				EventGenerationDate:    time.Now(),
			},
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{
					RegisterEventFunc: func(ctx context.Context, evt event.Event, emisorNit, razonSocial string) (*invoice.EventRegistrationResult, error) {
						return nil, errors.New("provider connection error")
					},
				}
			},
			expectedErr: "provider error",
		},
		{
			name:        "event with rejection - code 1001",
			emisorNit:   "123456789",
			razonSocial: "PROVEDOR NOMBRE",
			evt: event.Event{
				EventType:              event.EventTypeReciboBien,
				DocumentNumber:         "FAC12345",
				NombreGenerador:        "Mauricio",
				ApellidoGenerador:      "Alemán",
				IdentificacionGenerador: "1061239585",
				EventGenerationDate:    time.Now(),
			},
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{
					RegisterEventFunc: func(ctx context.Context, evt event.Event, emisorNit, razonSocial string) (*invoice.EventRegistrationResult, error) {
						return &invoice.EventRegistrationResult{
							Code:            "1001",
							NumeroDocumento: "FAC12345",
							Resultado: []invoice.EventResult{
								{
									TipoEvento:      "032",
									Mensaje:         "Evento rechazado.",
									MensajeError:    "Documento con errores en campos mandatorios.",
									CodigoRespuesta: "1001",
								},
							},
							MensajeError: "",
						}, nil
					},
				}
			},
			expectedCode: "1001",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := tt.setupProvider()
			service := NewService(provider, tt.emisorNit, tt.razonSocial)

			ctx := context.Background()
			result, err := service.RegisterEvent(ctx, tt.evt)

			if tt.expectedErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.expectedErr)
				}
				if !containsString(err.Error(), tt.expectedErr) {
					t.Errorf("expected error to contain %q, got %q", tt.expectedErr, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("expected result, got nil")
			}

			if tt.expectedCode != "" && result.Code != tt.expectedCode {
				t.Errorf("expected code %q, got %q", tt.expectedCode, result.Code)
			}
		})
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) && (s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			containsHelper(s, substr))))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
