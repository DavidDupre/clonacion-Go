package invoice

import (
	"context"
	"errors"
	"testing"
	"time"

	"3tcapital/ms_facturacion_core/internal/core/invoice"
	"3tcapital/ms_facturacion_core/internal/testutil"
)

// getTodayDate returns today's date in Colombia timezone (UTC-5) formatted as YYYY-MM-DD
func getTodayDate() string {
	loc, err := time.LoadLocation("America/Bogota")
	if err != nil {
		loc = time.FixedZone("America/Bogota", -5*60*60)
	}
	return time.Now().In(loc).Format("2006-01-02")
}

// getYesterdayDate returns yesterday's date in Colombia timezone (UTC-5) formatted as YYYY-MM-DD
func getYesterdayDate() string {
	loc, err := time.LoadLocation("America/Bogota")
	if err != nil {
		loc = time.FixedZone("America/Bogota", -5*60*60)
	}
	return time.Now().In(loc).AddDate(0, 0, -1).Format("2006-01-02")
}

// getTomorrowDate returns tomorrow's date in Colombia timezone (UTC-5) formatted as YYYY-MM-DD
func getTomorrowDate() string {
	loc, err := time.LoadLocation("America/Bogota")
	if err != nil {
		loc = time.FixedZone("America/Bogota", -5*60*60)
	}
	return time.Now().In(loc).AddDate(0, 0, 1).Format("2006-01-02")
}

func TestNewService(t *testing.T) {
	mockProvider := &testutil.MockProvider{}
	service := NewService(mockProvider, nil, nil, "2")

	if service == nil {
		t.Fatal("expected service to be created, got nil")
	}

	if service.provider != mockProvider {
		t.Error("expected service to have the provided provider")
	}
}

func TestService_GetDocuments(t *testing.T) {
	tests := []struct {
		name          string
		query         invoice.DocumentQuery
		setupProvider func() invoice.Provider
		expectedErr   string
		expectedCount int
	}{
		{
			name: "empty company nit",
			query: invoice.DocumentQuery{
				CompanyNit:  "",
				InitialDate: "2025-12-01",
				FinalDate:   "2025-12-31",
			},
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{}
			},
			expectedErr: "company nit is required",
		},
		{
			name: "company nit too short",
			query: invoice.DocumentQuery{
				CompanyNit:  "12345678", // 8 characters
				InitialDate: "2025-12-01",
				FinalDate:   "2025-12-31",
			},
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{}
			},
			expectedErr: "invalid company nit format",
		},
		{
			name: "company nit too long",
			query: invoice.DocumentQuery{
				CompanyNit:  "1234567890123456", // 16 characters
				InitialDate: "2025-12-01",
				FinalDate:   "2025-12-31",
			},
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{}
			},
			expectedErr: "invalid company nit format",
		},
		{
			name: "empty initial date",
			query: invoice.DocumentQuery{
				CompanyNit:  "123456789",
				InitialDate: "",
				FinalDate:   "2025-12-31",
			},
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{}
			},
			expectedErr: "initial date is required",
		},
		{
			name: "empty final date",
			query: invoice.DocumentQuery{
				CompanyNit:  "123456789",
				InitialDate: "2025-12-01",
				FinalDate:   "",
			},
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{}
			},
			expectedErr: "final date is required",
		},
		{
			name: "invalid initial date format",
			query: invoice.DocumentQuery{
				CompanyNit:  "123456789",
				InitialDate: "invalid-date",
				FinalDate:   "2025-12-31",
			},
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{}
			},
			expectedErr: "invalid initial date format",
		},
		{
			name: "invalid final date format",
			query: invoice.DocumentQuery{
				CompanyNit:  "123456789",
				InitialDate: "2025-12-01",
				FinalDate:   "invalid-date",
			},
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{}
			},
			expectedErr: "invalid final date format",
		},
		{
			name: "initial date after final date",
			query: invoice.DocumentQuery{
				CompanyNit:  "123456789",
				InitialDate: "2025-12-31",
				FinalDate:   "2025-12-01",
			},
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{}
			},
			expectedErr: "initial date must be before or equal to final date",
		},
		{
			name: "valid query - success",
			query: invoice.DocumentQuery{
				CompanyNit:  "123456789",
				InitialDate: "2025-12-01",
				FinalDate:   "2025-12-31",
			},
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{
					GetDocumentsFunc: func(ctx context.Context, query invoice.DocumentQuery) ([]invoice.Document, error) {
						return []invoice.Document{
							{OFE: "123456789", Valor: 100.0},
							{OFE: "123456789", Valor: 200.0},
						}, nil
					},
				}
			},
			expectedCount: 2,
		},
		{
			name: "provider error",
			query: invoice.DocumentQuery{
				CompanyNit:  "123456789",
				InitialDate: "2025-12-01",
				FinalDate:   "2025-12-31",
			},
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{
					GetDocumentsFunc: func(ctx context.Context, query invoice.DocumentQuery) ([]invoice.Document, error) {
						return nil, errors.New("provider error")
					},
				}
			},
			expectedErr: "provider error",
		},
		{
			name: "empty result from provider",
			query: invoice.DocumentQuery{
				CompanyNit:  "123456789",
				InitialDate: "2025-12-01",
				FinalDate:   "2025-12-31",
			},
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{
					GetDocumentsFunc: func(ctx context.Context, query invoice.DocumentQuery) ([]invoice.Document, error) {
						return []invoice.Document{}, nil
					},
				}
			},
			expectedCount: 0,
		},
		{
			name: "same initial and final date",
			query: invoice.DocumentQuery{
				CompanyNit:  "123456789",
				InitialDate: "2025-12-01",
				FinalDate:   "2025-12-01",
			},
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{
					GetDocumentsFunc: func(ctx context.Context, query invoice.DocumentQuery) ([]invoice.Document, error) {
						return []invoice.Document{}, nil
					},
				}
			},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := tt.setupProvider()
			service := NewService(provider, nil, nil, "2")

			ctx := context.Background()
			documents, err := service.GetDocuments(ctx, tt.query)

			if tt.expectedErr != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tt.expectedErr)
				}
				if !containsString(err.Error(), tt.expectedErr) {
					t.Errorf("expected error to contain %q, got %q", tt.expectedErr, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(documents) != tt.expectedCount {
				t.Errorf("expected %d documents, got %d", tt.expectedCount, len(documents))
			}
		})
	}
}

func TestService_GetDocumentByNumber(t *testing.T) {
	tests := []struct {
		name          string
		query         invoice.DocumentByNumberQuery
		setupProvider func() invoice.Provider
		expectedErr   string
		expectedCount int
	}{
		{
			name: "empty company nit",
			query: invoice.DocumentByNumberQuery{
				CompanyNit:     "",
				DocumentNumber: "4009100394926",
				SupplierNit:    "800242106",
			},
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{}
			},
			expectedErr: "company nit is required",
		},
		{
			name: "company nit too short",
			query: invoice.DocumentByNumberQuery{
				CompanyNit:     "12345678", // 8 characters
				DocumentNumber: "4009100394926",
				SupplierNit:    "800242106",
			},
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{}
			},
			expectedErr: "invalid company nit format",
		},
		{
			name: "company nit too long",
			query: invoice.DocumentByNumberQuery{
				CompanyNit:     "1234567890123456", // 16 characters
				DocumentNumber: "4009100394926",
				SupplierNit:    "800242106",
			},
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{}
			},
			expectedErr: "invalid company nit format",
		},
		{
			name: "empty document number",
			query: invoice.DocumentByNumberQuery{
				CompanyNit:     "811026198",
				DocumentNumber: "",
				SupplierNit:    "800242106",
			},
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{}
			},
			expectedErr: "document number is required",
		},
		{
			name: "empty supplier nit",
			query: invoice.DocumentByNumberQuery{
				CompanyNit:     "811026198",
				DocumentNumber: "4009100394926",
				SupplierNit:    "",
			},
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{}
			},
			expectedErr: "supplier nit is required",
		},
		{
			name: "supplier nit too short",
			query: invoice.DocumentByNumberQuery{
				CompanyNit:     "811026198",
				DocumentNumber: "4009100394926",
				SupplierNit:    "12345678", // 8 characters
			},
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{}
			},
			expectedErr: "invalid supplier nit format",
		},
		{
			name: "supplier nit too long",
			query: invoice.DocumentByNumberQuery{
				CompanyNit:     "811026198",
				DocumentNumber: "4009100394926",
				SupplierNit:    "1234567890123456", // 16 characters
			},
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{}
			},
			expectedErr: "invalid supplier nit format",
		},
		{
			name: "valid query - success",
			query: invoice.DocumentByNumberQuery{
				CompanyNit:     "811026198",
				DocumentNumber: "4009100394926",
				SupplierNit:    "800242106",
			},
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{
					GetDocumentByNumberFunc: func(ctx context.Context, query invoice.DocumentByNumberQuery) ([]invoice.Document, error) {
						return []invoice.Document{
							{OFE: "811026198", Valor: 100.0},
						}, nil
					},
				}
			},
			expectedCount: 1,
		},
		{
			name: "provider error",
			query: invoice.DocumentByNumberQuery{
				CompanyNit:     "811026198",
				DocumentNumber: "4009100394926",
				SupplierNit:    "800242106",
			},
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{
					GetDocumentByNumberFunc: func(ctx context.Context, query invoice.DocumentByNumberQuery) ([]invoice.Document, error) {
						return nil, errors.New("provider error")
					},
				}
			},
			expectedErr: "provider error",
		},
		{
			name: "empty result from provider",
			query: invoice.DocumentByNumberQuery{
				CompanyNit:     "811026198",
				DocumentNumber: "4009100394926",
				SupplierNit:    "800242106",
			},
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{
					GetDocumentByNumberFunc: func(ctx context.Context, query invoice.DocumentByNumberQuery) ([]invoice.Document, error) {
						return []invoice.Document{}, nil
					},
				}
			},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := tt.setupProvider()
			service := NewService(provider, nil, nil, "2")

			ctx := context.Background()
			documents, err := service.GetDocumentByNumber(ctx, tt.query)

			if tt.expectedErr != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tt.expectedErr)
				}
				if !containsString(err.Error(), tt.expectedErr) {
					t.Errorf("expected error to contain %q, got %q", tt.expectedErr, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(documents) != tt.expectedCount {
				t.Errorf("expected %d documents, got %d", tt.expectedCount, len(documents))
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

func TestService_RegisterDocument(t *testing.T) {
	tests := []struct {
		name          string
		req           invoice.DocumentRegistrationRequest
		setupProvider func() invoice.Provider
		expectedErr   string
	}{
		{
			name: "no documents provided",
			req: invoice.DocumentRegistrationRequest{
				Documentos: invoice.DocumentsByType{
					FC: []invoice.OpenETLDocument{},
					NC: []invoice.OpenETLDocument{},
					ND: []invoice.OpenETLDocument{},
				},
			},
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{}
			},
			expectedErr: "no documents provided",
		},
		{
			name: "multiple document types",
			req: invoice.DocumentRegistrationRequest{
				Documentos: invoice.DocumentsByType{
					FC: []invoice.OpenETLDocument{
						{TdeCodigo: "01"},
					},
					NC: []invoice.OpenETLDocument{
						{TdeCodigo: "03"},
					},
					ND: []invoice.OpenETLDocument{},
				},
			},
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{}
			},
			expectedErr: "only one document type",
		},
		{
			name: "missing tde_codigo",
			req: invoice.DocumentRegistrationRequest{
				Documentos: invoice.DocumentsByType{
					FC: []invoice.OpenETLDocument{
						{
							TdeCodigo: "",
						},
					},
					NC: []invoice.OpenETLDocument{},
					ND: []invoice.OpenETLDocument{},
				},
			},
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{}
			},
			expectedErr: "tde_codigo is required",
		},
		{
			name: "missing ofe_identificacion",
			req: invoice.DocumentRegistrationRequest{
				Documentos: invoice.DocumentsByType{
					FC: []invoice.OpenETLDocument{
						{
							TdeCodigo:         "01",
							OfeIdentificacion: "",
						},
					},
					NC: []invoice.OpenETLDocument{},
					ND: []invoice.OpenETLDocument{},
				},
			},
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{}
			},
			expectedErr: "ofe_identificacion is required",
		},
		{
			name: "invalid date format",
			req: invoice.DocumentRegistrationRequest{
				Documentos: invoice.DocumentsByType{
					FC: []invoice.OpenETLDocument{
						{
							TdeCodigo:            "01",
							OfeIdentificacion:    "860011153",
							AdqIdentificacion:    "900123456",
							RfaPrefijo:           "SETT",
							RfaResolucion:        "18760000001",
							CdoConsecutivo:       "5604",
							CdoFecha:             "29-06-2023", // Invalid format
							CdoHora:              "14:37:00",
							MonCodigo:            "COP",
							CdoValorSinImpuestos: "100000.00",
							CdoImpuestos:         "19000.00",
							CdoTotal:             "119000.00",
							Items: []invoice.OpenETLItem{
								{DdoSecuencia: "1"},
							},
						},
					},
					NC: []invoice.OpenETLDocument{},
					ND: []invoice.OpenETLDocument{},
				},
			},
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{}
			},
			expectedErr: "invalid cdo_fecha format",
		},
		{
			name: "invalid time format",
			req: func() invoice.DocumentRegistrationRequest {
				return invoice.DocumentRegistrationRequest{
					Documentos: invoice.DocumentsByType{
						FC: []invoice.OpenETLDocument{
							{
								TdeCodigo:            "01",
								OfeIdentificacion:    "860011153",
								AdqIdentificacion:    "900123456",
								RfaPrefijo:           "SETT",
								RfaResolucion:        "18760000001",
								CdoConsecutivo:       "5604",
								CdoFecha:             getTodayDate(),
								CdoHora:              "14:37", // Invalid format
								MonCodigo:            "COP",
								CdoValorSinImpuestos: "100000.00",
								CdoImpuestos:         "19000.00",
								CdoTotal:             "119000.00",
								Items: []invoice.OpenETLItem{
									{DdoSecuencia: "1"},
								},
							},
						},
						NC: []invoice.OpenETLDocument{},
						ND: []invoice.OpenETLDocument{},
					},
				}
			}(),
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{}
			},
			expectedErr: "invalid cdo_hora format",
		},
		{
			name: "no items",
			req: func() invoice.DocumentRegistrationRequest {
				return invoice.DocumentRegistrationRequest{
					Documentos: invoice.DocumentsByType{
						FC: []invoice.OpenETLDocument{
							{
								TdeCodigo:            "01",
								OfeIdentificacion:    "860011153",
								AdqIdentificacion:    "900123456",
								RfaPrefijo:           "SETT",
								RfaResolucion:        "18760000001",
								CdoConsecutivo:       "5604",
								CdoFecha:             getTodayDate(),
								CdoHora:              "14:37:00",
								MonCodigo:            "COP",
								CdoValorSinImpuestos: "100000.00",
								CdoImpuestos:         "19000.00",
								CdoTotal:             "119000.00",
								Items:                []invoice.OpenETLItem{}, // Empty
							},
						},
						NC: []invoice.OpenETLDocument{},
						ND: []invoice.OpenETLDocument{},
					},
				}
			}(),
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{}
			},
			expectedErr: "at least one item is required",
		},
		{
			name: "invalid document type code for FC",
			req: func() invoice.DocumentRegistrationRequest {
				return invoice.DocumentRegistrationRequest{
					Documentos: invoice.DocumentsByType{
						FC: []invoice.OpenETLDocument{
							{
								TdeCodigo:            "03", // NC code in FC array
								OfeIdentificacion:    "860011153",
								AdqIdentificacion:    "900123456",
								RfaPrefijo:           "SETT",
								RfaResolucion:        "18760000001",
								CdoConsecutivo:       "5604",
								CdoFecha:             getTodayDate(),
								CdoHora:              "14:37:00",
								MonCodigo:            "COP",
								CdoValorSinImpuestos: "100000.00",
								CdoImpuestos:         "19000.00",
								CdoTotal:             "119000.00",
								Items: []invoice.OpenETLItem{
									{DdoSecuencia: "1"},
								},
							},
						},
						NC: []invoice.OpenETLDocument{},
						ND: []invoice.OpenETLDocument{},
					},
				}
			}(),
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{}
			},
			expectedErr: "does not match document type",
		},
		{
			name: "valid FC document - success",
			req: func() invoice.DocumentRegistrationRequest {
				return invoice.DocumentRegistrationRequest{
					Documentos: invoice.DocumentsByType{
						FC: []invoice.OpenETLDocument{
							{
								TdeCodigo:               "01",
								OfeIdentificacion:       "860011153",
								AdqIdentificacion:       "900123456",
								RfaPrefijo:              "SETT",
								RfaResolucion:           "18760000001",
								CdoConsecutivo:          "5604",
								CdoFecha:                getTodayDate(),
								CdoHora:                 "14:37:00",
								MonCodigo:               "COP",
								CdoValorSinImpuestos:    "100000.00",
								CdoImpuestos:            "19000.00",
								CdoTotal:                "119000.00",
								CdoRetencionesSugeridas: "0.00",
								CdoAnticipo:             "0.00",
								CdoRedondeo:             "0.00",
								Items: []invoice.OpenETLItem{
									{
										DdoSecuencia:      "1",
										DdoDescripcionUno: "Producto",
										DdoCantidad:       "1",
										DdoValorUnitario:  "100000.00",
										DdoTotal:          "100000.00",
									},
								},
								Tributos: []invoice.OpenETLTributo{},
							},
						},
						NC: []invoice.OpenETLDocument{},
						ND: []invoice.OpenETLDocument{},
					},
				}
			}(),
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{
					RegisterDocumentFunc: func(ctx context.Context, req invoice.DocumentRegistrationRequest) (*invoice.DocumentRegistrationResponse, error) {
						return &invoice.DocumentRegistrationResponse{
							Message: "Documentos procesados exitosamente",
							Lote:    "lote-20240115-103000",
							DocumentosProcesados: []invoice.ProcessedDocument{
								{
									CdoID:              123456,
									RfaPrefijo:         "SETT",
									CdoConsecutivo:     "5604",
									FechaProcesamiento: "2024-01-15",
									HoraProcesamiento:  "10:35:00",
								},
							},
							DocumentosFallidos: []invoice.FailedDocument{},
						}, nil
					},
				}
			},
		},
		{
			name: "valid NC document - success",
			req: func() invoice.DocumentRegistrationRequest {
				return invoice.DocumentRegistrationRequest{
					Documentos: invoice.DocumentsByType{
						FC: []invoice.OpenETLDocument{},
						NC: []invoice.OpenETLDocument{
							{
								TdeCodigo:               "03",
								OfeIdentificacion:       "860011153",
								AdqIdentificacion:       "900123456",
								RfaPrefijo:              "NC",
								RfaResolucion:           "18760000001",
								CdoConsecutivo:          "100",
								CdoFecha:                getTodayDate(),
								CdoHora:                 "14:37:00",
								MonCodigo:               "COP",
								CdoValorSinImpuestos:    "100000.00",
								CdoImpuestos:            "19000.00",
								CdoTotal:                "119000.00",
								CdoRetencionesSugeridas: "0.00",
								CdoAnticipo:             "0.00",
								CdoRedondeo:             "0.00",
								Items: []invoice.OpenETLItem{
									{
										DdoSecuencia:      "1",
										DdoDescripcionUno: "Devolución",
										DdoCantidad:       "1",
										DdoValorUnitario:  "100000.00",
										DdoTotal:          "100000.00",
									},
								},
								Tributos: []invoice.OpenETLTributo{},
							},
						},
						ND: []invoice.OpenETLDocument{},
					},
				}
			}(),
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{
					RegisterDocumentFunc: func(ctx context.Context, req invoice.DocumentRegistrationRequest) (*invoice.DocumentRegistrationResponse, error) {
						return &invoice.DocumentRegistrationResponse{
							Message: "Documentos procesados exitosamente",
							Lote:    "lote-20240115-103000",
							DocumentosProcesados: []invoice.ProcessedDocument{
								{
									CdoID:              123457,
									RfaPrefijo:         "NC",
									CdoConsecutivo:     "100",
									FechaProcesamiento: "2024-01-15",
									HoraProcesamiento:  "10:35:00",
								},
							},
							DocumentosFallidos: []invoice.FailedDocument{},
						}, nil
					},
				}
			},
		},
		{
			name: "provider error",
			req: func() invoice.DocumentRegistrationRequest {
				return invoice.DocumentRegistrationRequest{
					Documentos: invoice.DocumentsByType{
						FC: []invoice.OpenETLDocument{
							{
								TdeCodigo:               "01",
								OfeIdentificacion:       "860011153",
								AdqIdentificacion:       "900123456",
								RfaPrefijo:              "SETT",
								RfaResolucion:           "18760000001",
								CdoConsecutivo:          "5604",
								CdoFecha:                getTodayDate(),
								CdoHora:                 "14:37:00",
								MonCodigo:               "COP",
								CdoValorSinImpuestos:    "100000.00",
								CdoImpuestos:            "19000.00",
								CdoTotal:                "119000.00",
								CdoRetencionesSugeridas: "0.00",
								CdoAnticipo:             "0.00",
								CdoRedondeo:             "0.00",
								Items: []invoice.OpenETLItem{
									{
										DdoSecuencia:      "1",
										DdoDescripcionUno: "Producto",
										DdoCantidad:       "1",
										DdoValorUnitario:  "100000.00",
										DdoTotal:          "100000.00",
									},
								},
								Tributos: []invoice.OpenETLTributo{},
							},
						},
						NC: []invoice.OpenETLDocument{},
						ND: []invoice.OpenETLDocument{},
					},
				}
			}(),
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{
					RegisterDocumentFunc: func(ctx context.Context, req invoice.DocumentRegistrationRequest) (*invoice.DocumentRegistrationResponse, error) {
						return nil, errors.New("provider error")
					},
				}
			},
			expectedErr: "provider error",
		},
		{
			name: "FAD09e - yesterday date fails",
			req: func() invoice.DocumentRegistrationRequest {
				return invoice.DocumentRegistrationRequest{
					Documentos: invoice.DocumentsByType{
						FC: []invoice.OpenETLDocument{
							{
								TdeCodigo:               "01",
								OfeIdentificacion:       "860011153",
								AdqIdentificacion:       "900123456",
								RfaPrefijo:              "SETT",
								RfaResolucion:           "18760000001",
								CdoConsecutivo:          "5604",
								CdoFecha:                getYesterdayDate(),
								CdoHora:                 "14:37:00",
								MonCodigo:               "COP",
								CdoValorSinImpuestos:    "100000.00",
								CdoImpuestos:            "19000.00",
								CdoTotal:                "119000.00",
								CdoRetencionesSugeridas: "0.00",
								CdoAnticipo:             "0.00",
								CdoRedondeo:             "0.00",
								Items: []invoice.OpenETLItem{
									{
										DdoSecuencia:      "1",
										DdoDescripcionUno: "Producto",
										DdoCantidad:       "1",
										DdoValorUnitario:  "100000.00",
										DdoTotal:          "100000.00",
									},
								},
								Tributos: []invoice.OpenETLTributo{},
							},
						},
						NC: []invoice.OpenETLDocument{},
						ND: []invoice.OpenETLDocument{},
					},
				}
			}(),
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{}
			},
			expectedErr: "DIAN FAD09e compliance",
		},
		{
			name: "FAD09e - future date fails",
			req: func() invoice.DocumentRegistrationRequest {
				return invoice.DocumentRegistrationRequest{
					Documentos: invoice.DocumentsByType{
						FC: []invoice.OpenETLDocument{
							{
								TdeCodigo:               "01",
								OfeIdentificacion:       "860011153",
								AdqIdentificacion:       "900123456",
								RfaPrefijo:              "SETT",
								RfaResolucion:           "18760000001",
								CdoConsecutivo:          "5604",
								CdoFecha:                getTomorrowDate(),
								CdoHora:                 "14:37:00",
								MonCodigo:               "COP",
								CdoValorSinImpuestos:    "100000.00",
								CdoImpuestos:            "19000.00",
								CdoTotal:                "119000.00",
								CdoRetencionesSugeridas: "0.00",
								CdoAnticipo:             "0.00",
								CdoRedondeo:             "0.00",
								Items: []invoice.OpenETLItem{
									{
										DdoSecuencia:      "1",
										DdoDescripcionUno: "Producto",
										DdoCantidad:       "1",
										DdoValorUnitario:  "100000.00",
										DdoTotal:          "100000.00",
									},
								},
								Tributos: []invoice.OpenETLTributo{},
							},
						},
						NC: []invoice.OpenETLDocument{},
						ND: []invoice.OpenETLDocument{},
					},
				}
			}(),
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{}
			},
			expectedErr: "DIAN FAD09e compliance",
		},
		{
			name: "FAD09e - historical date fails",
			req: invoice.DocumentRegistrationRequest{
				Documentos: invoice.DocumentsByType{
					FC: []invoice.OpenETLDocument{
						{
							TdeCodigo:               "01",
							OfeIdentificacion:       "860011153",
							AdqIdentificacion:       "900123456",
							RfaPrefijo:              "SETT",
							RfaResolucion:           "18760000001",
							CdoConsecutivo:          "5604",
							CdoFecha:                "2023-06-29", // Historical date
							CdoHora:                 "14:37:00",
							MonCodigo:               "COP",
							CdoValorSinImpuestos:    "100000.00",
							CdoImpuestos:            "19000.00",
							CdoTotal:                "119000.00",
							CdoRetencionesSugeridas: "0.00",
							CdoAnticipo:             "0.00",
							CdoRedondeo:             "0.00",
							Items: []invoice.OpenETLItem{
								{
									DdoSecuencia:      "1",
									DdoDescripcionUno: "Producto",
									DdoCantidad:       "1",
									DdoValorUnitario:  "100000.00",
									DdoTotal:          "100000.00",
								},
							},
							Tributos: []invoice.OpenETLTributo{},
						},
					},
					NC: []invoice.OpenETLDocument{},
					ND: []invoice.OpenETLDocument{},
				},
			},
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{}
			},
			expectedErr: "DIAN FAD09e compliance",
		},
		{
			name: "invalid cdo_vencimiento format",
			req: func() invoice.DocumentRegistrationRequest {
				invalidDate := "15-07-2023" // Invalid format (DD-MM-YYYY instead of YYYY-MM-DD)
				return invoice.DocumentRegistrationRequest{
					Documentos: invoice.DocumentsByType{
						FC: []invoice.OpenETLDocument{
							{
								TdeCodigo:               "01",
								OfeIdentificacion:       "860011153",
								AdqIdentificacion:       "900123456",
								RfaPrefijo:              "SETT",
								RfaResolucion:           "18760000001",
								CdoConsecutivo:          "5604",
								CdoFecha:                getTodayDate(),
								CdoHora:                 "14:37:00",
								CdoVencimiento:          &invalidDate,
								MonCodigo:               "COP",
								CdoValorSinImpuestos:    "100000.00",
								CdoImpuestos:            "19000.00",
								CdoTotal:                "119000.00",
								CdoRetencionesSugeridas: "0.00",
								CdoAnticipo:             "0.00",
								CdoRedondeo:             "0.00",
								Items: []invoice.OpenETLItem{
									{
										DdoSecuencia:      "1",
										DdoDescripcionUno: "Producto",
										DdoCantidad:       "1",
										DdoValorUnitario:  "100000.00",
										DdoTotal:          "100000.00",
									},
								},
								Tributos: []invoice.OpenETLTributo{},
							},
						},
						NC: []invoice.OpenETLDocument{},
						ND: []invoice.OpenETLDocument{},
					},
				}
			}(),
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{}
			},
			expectedErr: "invalid cdo_vencimiento format",
		},
		{
			name: "cdo_vencimiento before cdo_fecha",
			req: func() invoice.DocumentRegistrationRequest {
				pastDate := getYesterdayDate()
				return invoice.DocumentRegistrationRequest{
					Documentos: invoice.DocumentsByType{
						FC: []invoice.OpenETLDocument{
							{
								TdeCodigo:               "01",
								OfeIdentificacion:       "860011153",
								AdqIdentificacion:       "900123456",
								RfaPrefijo:              "SETT",
								RfaResolucion:           "18760000001",
								CdoConsecutivo:          "5604",
								CdoFecha:                getTodayDate(),
								CdoHora:                 "14:37:00",
								CdoVencimiento:          &pastDate,
								MonCodigo:               "COP",
								CdoValorSinImpuestos:    "100000.00",
								CdoImpuestos:            "19000.00",
								CdoTotal:                "119000.00",
								CdoRetencionesSugeridas: "0.00",
								CdoAnticipo:             "0.00",
								CdoRedondeo:             "0.00",
								Items: []invoice.OpenETLItem{
									{
										DdoSecuencia:      "1",
										DdoDescripcionUno: "Producto",
										DdoCantidad:       "1",
										DdoValorUnitario:  "100000.00",
										DdoTotal:          "100000.00",
									},
								},
								Tributos: []invoice.OpenETLTributo{},
							},
						},
						NC: []invoice.OpenETLDocument{},
						ND: []invoice.OpenETLDocument{},
					},
				}
			}(),
			setupProvider: func() invoice.Provider {
				return &testutil.MockProvider{}
			},
			expectedErr: "must be on or after cdo_fecha",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := tt.setupProvider()
			service := NewService(provider, nil, nil, "2")

			ctx := context.Background()
			resp, err := service.RegisterDocument(ctx, tt.req)

			if tt.expectedErr != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tt.expectedErr)
				}
				if !containsString(err.Error(), tt.expectedErr) {
					t.Errorf("expected error to contain %q, got %q", tt.expectedErr, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if resp == nil {
				t.Fatal("expected response, got nil")
			}
		})
	}
}
