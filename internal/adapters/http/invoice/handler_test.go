package invoice

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	appinvoice "3tcapital/ms_facturacion_core/internal/application/invoice"
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

func TestNewHandler(t *testing.T) {
	service := &appinvoice.Service{}
	logger := testutil.NewNullLogger()
	handler := NewHandler(service, nil, logger)

	if handler == nil {
		t.Fatal("expected handler to be created, got nil")
	}

	if handler.service != service {
		t.Error("expected handler to have the provided service")
	}

	if handler.log != logger {
		t.Error("expected handler to have the provided logger")
	}
}

func TestHandler_GetDocuments(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    interface{}
		setupService   func() *appinvoice.Service
		expectedStatus int
		expectedBody   func(t *testing.T, body map[string]interface{})
	}{
		{
			name: "success with valid body",
			requestBody: GetDocumentsRequest{
				CompanyNit:  "860011153",
				InitialDate: "2025-12-01",
				FinalDate:   "2025-12-31",
			},
			setupService: func() *appinvoice.Service {
				mockProvider := &testutil.MockProvider{
					GetDocumentsFunc: func(ctx context.Context, query invoice.DocumentQuery) ([]invoice.Document, error) {
						return []invoice.Document{
							{
								OFE:         "123456789",
								Proveedor:   "Test Provider",
								Tipo:        "01",
								Prefijo:     "",
								Consecutivo: "00000001",
								CUFE:        "c1234567890",
								Fecha:       time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC),
								Hora:        "10:30:00",
								Valor:       150000.50,
								Marca:       false,
							},
						}, nil
					},
				}
				return appinvoice.NewService(mockProvider, nil, nil, "2")
			},
			expectedStatus: http.StatusOK,
			expectedBody: func(t *testing.T, body map[string]interface{}) {
				if body["status"] != "200" {
					t.Errorf("expected status '200', got %v", body["status"])
				}
				if body["message"] != "Exitoso" {
					t.Errorf("expected message 'Exitoso', got %v", body["message"])
				}
				if body["total"].(float64) != 1 {
					t.Errorf("expected total 1, got %v", body["total"])
				}
				data, ok := body["data"].([]interface{})
				if !ok {
					t.Fatal("expected data array")
				}
				if len(data) != 1 {
					t.Errorf("expected 1 document, got %d", len(data))
				}
			},
		},
		{
			name:        "invalid JSON",
			requestBody: "invalid json",
			setupService: func() *appinvoice.Service {
				mockProvider := &testutil.MockProvider{}
				return appinvoice.NewService(mockProvider, nil, nil, "2")
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody: func(t *testing.T, body map[string]interface{}) {
				if body["message"] != "Error de Validación" {
					t.Errorf("expected message 'Error de Validación', got %v", body["message"])
				}
			},
		},
		{
			name: "missing company nit",
			requestBody: GetDocumentsRequest{
				CompanyNit:  "",
				InitialDate: "2025-12-01",
				FinalDate:   "2025-12-31",
			},
			setupService: func() *appinvoice.Service {
				mockProvider := &testutil.MockProvider{}
				return appinvoice.NewService(mockProvider, nil, nil, "2")
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody: func(t *testing.T, body map[string]interface{}) {
				if body["message"] != "Error de Validación" {
					t.Errorf("expected message 'Error de Validación', got %v", body["message"])
				}
			},
		},
		{
			name: "missing initial date",
			requestBody: GetDocumentsRequest{
				CompanyNit:  "860011153",
				InitialDate: "",
				FinalDate:   "2025-12-31",
			},
			setupService: func() *appinvoice.Service {
				mockProvider := &testutil.MockProvider{}
				return appinvoice.NewService(mockProvider, nil, nil, "2")
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody: func(t *testing.T, body map[string]interface{}) {
				if body["message"] != "Error de Validación" {
					t.Errorf("expected message 'Error de Validación', got %v", body["message"])
				}
			},
		},
		{
			name: "missing final date",
			requestBody: GetDocumentsRequest{
				CompanyNit:  "860011153",
				InitialDate: "2025-12-01",
				FinalDate:   "",
			},
			setupService: func() *appinvoice.Service {
				mockProvider := &testutil.MockProvider{}
				return appinvoice.NewService(mockProvider, nil, nil, "2")
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody: func(t *testing.T, body map[string]interface{}) {
				if body["message"] != "Error de Validación" {
					t.Errorf("expected message 'Error de Validación', got %v", body["message"])
				}
			},
		},
		{
			name: "empty result",
			requestBody: GetDocumentsRequest{
				CompanyNit:  "860011153",
				InitialDate: "2025-12-01",
				FinalDate:   "2025-12-31",
			},
			setupService: func() *appinvoice.Service {
				mockProvider := &testutil.MockProvider{
					GetDocumentsFunc: func(ctx context.Context, query invoice.DocumentQuery) ([]invoice.Document, error) {
						return []invoice.Document{}, nil
					},
				}
				return appinvoice.NewService(mockProvider, nil, nil, "2")
			},
			expectedStatus: http.StatusOK,
			expectedBody: func(t *testing.T, body map[string]interface{}) {
				if body["total"].(float64) != 0 {
					t.Errorf("expected total 0, got %v", body["total"])
				}
				data, ok := body["data"].([]interface{})
				if !ok {
					t.Fatal("expected data array")
				}
				if len(data) != 0 {
					t.Errorf("expected 0 documents, got %d", len(data))
				}
			},
		},
		{
			name: "service error",
			requestBody: GetDocumentsRequest{
				CompanyNit:  "860011153",
				InitialDate: "2025-12-01",
				FinalDate:   "2025-12-31",
			},
			setupService: func() *appinvoice.Service {
				mockProvider := &testutil.MockProvider{
					GetDocumentsFunc: func(ctx context.Context, query invoice.DocumentQuery) ([]invoice.Document, error) {
						return nil, errors.New("provider error")
					},
				}
				return appinvoice.NewService(mockProvider, nil, nil, "2")
			},
			expectedStatus: http.StatusBadGateway,
			expectedBody: func(t *testing.T, body map[string]interface{}) {
				if body["message"] != "Error del Proveedor" {
					t.Errorf("expected message 'Error del Proveedor', got %v", body["message"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := tt.setupService()
			logger := testutil.NewNullLogger()
			handler := NewHandler(service, nil, logger)

			var bodyBytes []byte
			var err error
			if str, ok := tt.requestBody.(string); ok {
				bodyBytes = []byte(str)
			} else {
				bodyBytes, err = json.Marshal(tt.requestBody)
				if err != nil {
					t.Fatalf("failed to marshal request body: %v", err)
				}
			}

			req := httptest.NewRequest(http.MethodPost, "/api/v1/facturas", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.GetDocuments(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			var body map[string]interface{}
			if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if tt.expectedBody != nil {
				tt.expectedBody(t, body)
			}
		})
	}
}

func TestHandler_handleError(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		expectedStatus int
		expectedMsg    string
	}{
		{
			name:           "validation error - company nit",
			err:            errors.New("company nit is required"),
			expectedStatus: http.StatusBadRequest,
			expectedMsg:    "Error de Validación",
		},
		{
			name:           "validation error - date format",
			err:            errors.New("invalid initial date format: must be YYYY-MM-DD"),
			expectedStatus: http.StatusBadRequest,
			expectedMsg:    "Error de Validación",
		},
		{
			name:           "provider authentication error",
			err:            errors.New("authentication failed"),
			expectedStatus: http.StatusBadGateway,
			expectedMsg:    "Error de Autenticación",
		},
		{
			name:           "provider error",
			err:            errors.New("provider error"),
			expectedStatus: http.StatusBadGateway,
			expectedMsg:    "Error del Proveedor",
		},
		{
			name:           "FAD09e compliance error",
			err:            errors.New("document 1: cdo_fecha must be today's date (2025-12-17) for DIAN FAD09e compliance. Provided: 2025-12-16"),
			expectedStatus: http.StatusBadRequest,
			expectedMsg:    "Error de Validación",
		},
		{
			name:           "must be today's date error",
			err:            errors.New("document 1: cdo_fecha must be today's date"),
			expectedStatus: http.StatusBadRequest,
			expectedMsg:    "Error de Validación",
		},
		{
			name:           "failed documents error",
			err:            errors.New("documentos fallidos: Algunos documentos fallaron al procesarse: Documento SETT5604: Error de validación"),
			expectedStatus: http.StatusBadRequest,
			expectedMsg:    "Error de Validación",
		},
		{
			name:           "unknown error",
			err:            errors.New("unknown error"),
			expectedStatus: http.StatusInternalServerError,
			expectedMsg:    "Error Interno del Servidor",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := testutil.NewNullLogger()
			handler := NewHandler(&appinvoice.Service{}, nil, logger)
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/test", nil)

			handler.handleError(w, req, tt.err)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			var response map[string]interface{}
			if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if response["message"] != tt.expectedMsg {
				t.Errorf("expected message %q, got %q", tt.expectedMsg, response["message"])
			}
		})
	}
}

func TestHandler_GetDocumentByNumber(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    interface{}
		setupService   func() *appinvoice.Service
		expectedStatus int
		expectedBody   func(t *testing.T, body map[string]interface{})
	}{
		{
			name: "success with valid body",
			requestBody: GetDocumentByNumberRequest{
				CompanyNit:     "811026198",
				DocumentNumber: "4009100394926",
				SupplierNit:    "800242106",
			},
			setupService: func() *appinvoice.Service {
				mockProvider := &testutil.MockProvider{
					GetDocumentByNumberFunc: func(ctx context.Context, query invoice.DocumentByNumberQuery) ([]invoice.Document, error) {
						return []invoice.Document{
							{
								OFE:         "811026198",
								Proveedor:   "Test Provider",
								Tipo:        "01",
								Prefijo:     "",
								Consecutivo: "4009100394926",
								CUFE:        "c1234567890",
								Fecha:       time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC),
								Hora:        "10:30:00",
								Valor:       150000.50,
								Marca:       false,
							},
						}, nil
					},
				}
				return appinvoice.NewService(mockProvider, nil, nil, "2")
			},
			expectedStatus: http.StatusOK,
			expectedBody: func(t *testing.T, body map[string]interface{}) {
				if body["status"] != "200" {
					t.Errorf("expected status '200', got %v", body["status"])
				}
				if body["message"] != "Exitoso" {
					t.Errorf("expected message 'Exitoso', got %v", body["message"])
				}
				if body["total"].(float64) != 1 {
					t.Errorf("expected total 1, got %v", body["total"])
				}
				data, ok := body["data"].([]interface{})
				if !ok {
					t.Fatal("expected data array")
				}
				if len(data) != 1 {
					t.Errorf("expected 1 document, got %d", len(data))
				}
			},
		},
		{
			name:        "invalid JSON",
			requestBody: "invalid json",
			setupService: func() *appinvoice.Service {
				mockProvider := &testutil.MockProvider{}
				return appinvoice.NewService(mockProvider, nil, nil, "2")
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody: func(t *testing.T, body map[string]interface{}) {
				if body["message"] != "Error de Validación" {
					t.Errorf("expected message 'Error de Validación', got %v", body["message"])
				}
			},
		},
		{
			name: "missing company nit",
			requestBody: GetDocumentByNumberRequest{
				CompanyNit:     "",
				DocumentNumber: "4009100394926",
				SupplierNit:    "800242106",
			},
			setupService: func() *appinvoice.Service {
				mockProvider := &testutil.MockProvider{}
				return appinvoice.NewService(mockProvider, nil, nil, "2")
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody: func(t *testing.T, body map[string]interface{}) {
				if body["message"] != "Error de Validación" {
					t.Errorf("expected message 'Error de Validación', got %v", body["message"])
				}
			},
		},
		{
			name: "missing document number",
			requestBody: GetDocumentByNumberRequest{
				CompanyNit:     "811026198",
				DocumentNumber: "",
				SupplierNit:    "800242106",
			},
			setupService: func() *appinvoice.Service {
				mockProvider := &testutil.MockProvider{}
				return appinvoice.NewService(mockProvider, nil, nil, "2")
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody: func(t *testing.T, body map[string]interface{}) {
				if body["message"] != "Error de Validación" {
					t.Errorf("expected message 'Error de Validación', got %v", body["message"])
				}
			},
		},
		{
			name: "missing supplier nit",
			requestBody: GetDocumentByNumberRequest{
				CompanyNit:     "811026198",
				DocumentNumber: "4009100394926",
				SupplierNit:    "",
			},
			setupService: func() *appinvoice.Service {
				mockProvider := &testutil.MockProvider{}
				return appinvoice.NewService(mockProvider, nil, nil, "2")
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody: func(t *testing.T, body map[string]interface{}) {
				if body["message"] != "Error de Validación" {
					t.Errorf("expected message 'Error de Validación', got %v", body["message"])
				}
			},
		},
		{
			name: "empty result",
			requestBody: GetDocumentByNumberRequest{
				CompanyNit:     "811026198",
				DocumentNumber: "4009100394926",
				SupplierNit:    "800242106",
			},
			setupService: func() *appinvoice.Service {
				mockProvider := &testutil.MockProvider{
					GetDocumentByNumberFunc: func(ctx context.Context, query invoice.DocumentByNumberQuery) ([]invoice.Document, error) {
						return []invoice.Document{}, nil
					},
				}
				return appinvoice.NewService(mockProvider, nil, nil, "2")
			},
			expectedStatus: http.StatusOK,
			expectedBody: func(t *testing.T, body map[string]interface{}) {
				if body["total"].(float64) != 0 {
					t.Errorf("expected total 0, got %v", body["total"])
				}
				data, ok := body["data"].([]interface{})
				if !ok {
					t.Fatal("expected data array")
				}
				if len(data) != 0 {
					t.Errorf("expected 0 documents, got %d", len(data))
				}
			},
		},
		{
			name: "service error",
			requestBody: GetDocumentByNumberRequest{
				CompanyNit:     "811026198",
				DocumentNumber: "4009100394926",
				SupplierNit:    "800242106",
			},
			setupService: func() *appinvoice.Service {
				mockProvider := &testutil.MockProvider{
					GetDocumentByNumberFunc: func(ctx context.Context, query invoice.DocumentByNumberQuery) ([]invoice.Document, error) {
						return nil, errors.New("provider error")
					},
				}
				return appinvoice.NewService(mockProvider, nil, nil, "2")
			},
			expectedStatus: http.StatusBadGateway,
			expectedBody: func(t *testing.T, body map[string]interface{}) {
				if body["message"] != "Error del Proveedor" {
					t.Errorf("expected message 'Error del Proveedor', got %v", body["message"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := tt.setupService()
			logger := testutil.NewNullLogger()
			handler := NewHandler(service, nil, logger)

			var bodyBytes []byte
			var err error
			if str, ok := tt.requestBody.(string); ok {
				bodyBytes = []byte(str)
			} else {
				bodyBytes, err = json.Marshal(tt.requestBody)
				if err != nil {
					t.Fatalf("failed to marshal request body: %v", err)
				}
			}

			req := httptest.NewRequest(http.MethodPost, "/api/v1/facturas/by-number", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.GetDocumentByNumber(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			var body map[string]interface{}
			if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}

			if tt.expectedBody != nil {
				tt.expectedBody(t, body)
			}
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		substr   string
		expected bool
	}{
		{
			name:     "case insensitive match",
			s:        "Hello World",
			substr:   "hello",
			expected: true,
		},
		{
			name:     "exact match",
			s:        "test string",
			substr:   "test",
			expected: true,
		},
		{
			name:     "no match",
			s:        "test string",
			substr:   "notfound",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.s, tt.substr)
			if result != tt.expected {
				t.Errorf("contains(%q, %q) = %v, expected %v", tt.s, tt.substr, result, tt.expected)
			}
		})
	}
}

func TestHandler_RegisterDocument(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		body           interface{}
		setupService   func() *appinvoice.Service
		expectedStatus int
		expectedBody   func(t *testing.T, body map[string]interface{})
	}{
		{
			name:   "method not allowed",
			method: http.MethodGet,
			body:   map[string]interface{}{},
			setupService: func() *appinvoice.Service {
				return appinvoice.NewService(&testutil.MockProvider{}, nil, nil, "2")
			},
			expectedStatus: http.StatusMethodNotAllowed,
		},
		{
			name:   "invalid JSON body",
			method: http.MethodPost,
			body:   "invalid json",
			setupService: func() *appinvoice.Service {
				return appinvoice.NewService(&testutil.MockProvider{}, nil, nil, "2")
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "success - FC document",
			method: http.MethodPost,
			body: func() invoice.DocumentRegistrationRequest {
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
			setupService: func() *appinvoice.Service {
				provider := &testutil.MockProvider{
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
				return appinvoice.NewService(provider, nil, nil, "2")
			},
			expectedStatus: http.StatusOK,
			expectedBody: func(t *testing.T, body map[string]interface{}) {
				if body["message"] != "Documentos procesados exitosamente" {
					t.Errorf("expected message 'Documentos procesados exitosamente', got %v", body["message"])
				}
				if body["lote"] != "lote-20240115-103000" {
					t.Errorf("expected lote 'lote-20240115-103000', got %v", body["lote"])
				}
			},
		},
		{
			name:   "validation error - no documents",
			method: http.MethodPost,
			body: invoice.DocumentRegistrationRequest{
				Documentos: invoice.DocumentsByType{
					FC: []invoice.OpenETLDocument{},
					NC: []invoice.OpenETLDocument{},
					ND: []invoice.OpenETLDocument{},
				},
			},
			setupService: func() *appinvoice.Service {
				return appinvoice.NewService(&testutil.MockProvider{}, nil, nil, "2")
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "validation error - missing required field",
			method: http.MethodPost,
			body: invoice.DocumentRegistrationRequest{
				Documentos: invoice.DocumentsByType{
					FC: []invoice.OpenETLDocument{
						{
							TdeCodigo: "01",
							// Missing other required fields
						},
					},
					NC: []invoice.OpenETLDocument{},
					ND: []invoice.OpenETLDocument{},
				},
			},
			setupService: func() *appinvoice.Service {
				return appinvoice.NewService(&testutil.MockProvider{}, nil, nil, "2")
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:   "provider error - authentication failed",
			method: http.MethodPost,
			body: func() invoice.DocumentRegistrationRequest {
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
			setupService: func() *appinvoice.Service {
				provider := &testutil.MockProvider{
					RegisterDocumentFunc: func(ctx context.Context, req invoice.DocumentRegistrationRequest) (*invoice.DocumentRegistrationResponse, error) {
						return nil, errors.New("authentication failed: token expired")
					},
				}
				return appinvoice.NewService(provider, nil, nil, "2")
			},
			expectedStatus: http.StatusBadGateway,
		},
		{
			name:   "FAD09e validation error - wrong date",
			method: http.MethodPost,
			body: func() invoice.DocumentRegistrationRequest {
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
								CdoFecha:                "2025-12-16", // Wrong date
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
			setupService: func() *appinvoice.Service {
				return appinvoice.NewService(&testutil.MockProvider{}, nil, nil, "2")
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody: func(t *testing.T, body map[string]interface{}) {
				if body["message"] != "Error de Validación" {
					t.Errorf("expected message 'Error de Validación', got %v", body["message"])
				}
			},
		},
		{
			name:   "response with failed documents",
			method: http.MethodPost,
			body: func() invoice.DocumentRegistrationRequest {
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
			setupService: func() *appinvoice.Service {
				provider := &testutil.MockProvider{
					RegisterDocumentFunc: func(ctx context.Context, req invoice.DocumentRegistrationRequest) (*invoice.DocumentRegistrationResponse, error) {
						return &invoice.DocumentRegistrationResponse{
							Message:              "Algunos documentos fallaron",
							Lote:                 "lote-20240115-103000",
							DocumentosProcesados: []invoice.ProcessedDocument{},
							DocumentosFallidos: []invoice.FailedDocument{
								{
									Documento:          "SETT5604",
									Consecutivo:        "5604",
									Prefijo:            "SETT",
									Errors:             []string{"Error de validación DIAN", "Regla FAJ24"},
									FechaProcesamiento: "2024-01-15",
									HoraProcesamiento:  "10:35:00",
								},
							},
						}, nil
					},
				}
				return appinvoice.NewService(provider, nil, nil, "2")
			},
			expectedStatus: http.StatusBadRequest,
			expectedBody: func(t *testing.T, body map[string]interface{}) {
				if body["message"] != "Error de Validación" {
					t.Errorf("expected message 'Error de Validación', got %v", body["message"])
				}
			},
		},
		{
			name:   "response with empty message but processed documents",
			method: http.MethodPost,
			body: func() invoice.DocumentRegistrationRequest {
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
			setupService: func() *appinvoice.Service {
				provider := &testutil.MockProvider{
					RegisterDocumentFunc: func(ctx context.Context, req invoice.DocumentRegistrationRequest) (*invoice.DocumentRegistrationResponse, error) {
						return &invoice.DocumentRegistrationResponse{
							Message: "", // Empty message
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
				return appinvoice.NewService(provider, nil, nil, "2")
			},
			expectedStatus: http.StatusOK,
			expectedBody: func(t *testing.T, body map[string]interface{}) {
				// Should have default message
				if body["message"] != "Documentos procesados exitosamente" {
					t.Errorf("expected message 'Documentos procesados exitosamente', got %v", body["message"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := tt.setupService()
			logger := testutil.NewNullLogger()
			handler := NewHandler(service, nil, logger)

			var bodyBytes []byte
			if strBody, ok := tt.body.(string); ok {
				bodyBytes = []byte(strBody)
			} else {
				var err error
				bodyBytes, err = json.Marshal(tt.body)
				if err != nil {
					t.Fatalf("failed to marshal body: %v", err)
				}
			}

			req := httptest.NewRequest(tt.method, "/api/v1/registrar-documentos", bytes.NewReader(bodyBytes))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			handler.RegisterDocument(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedStatus == http.StatusOK && tt.expectedBody != nil {
				var body map[string]interface{}
				if err := json.NewDecoder(w.Body).Decode(&body); err != nil {
					t.Fatalf("failed to decode response: %v", err)
				}
				tt.expectedBody(t, body)
			}
		})
	}
}
