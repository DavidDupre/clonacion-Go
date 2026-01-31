package numrot

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"3tcapital/ms_facturacion_core/internal/core/event"
	"3tcapital/ms_facturacion_core/internal/core/invoice"
	"3tcapital/ms_facturacion_core/internal/testutil"
)

func TestNewClient(t *testing.T) {
	baseURL := "https://api.example.com"
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("token"))
	}))
	defer authServer.Close()

	auth := NewAuthManager(authServer.URL, "user", "pass", 1*time.Hour, authServer.Client(), testutil.NewTestLogger())
	client := &http.Client{}
	logger := testutil.NewTestLogger()

	numrotClient := NewClient(baseURL, auth, client, logger, "", "", "", nil)

	if numrotClient == nil {
		t.Fatal("expected client to be created, got nil")
	}

	// Type assertion to check it's a Client
	clientImpl, ok := numrotClient.(*Client)
	if !ok {
		t.Fatal("expected *Client type")
	}

	if clientImpl.baseURL != baseURL {
		t.Errorf("expected baseURL %q, got %q", baseURL, clientImpl.baseURL)
	}
}

func TestClient_GetResolutions_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/api/Resoluciones/") {
			t.Errorf("expected path to contain /api/Resoluciones/, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("expected Authorization header, got %q", r.Header.Get("Authorization"))
		}

		response := numrotResolutionResponse{
			OperationCode:        "100",
			OperationDescription: "Success",
			NumberRangeResponse: []numrotNumberRange{
				{
					ResolutionNumber: "RES001",
					ResolutionDate:   "2024-01-01",
					Prefix:           "PREF",
					FromNumber:       1,
					ToNumber:         100,
					ValidDateFrom:    "2024-01-01",
					ValidDateTo:      "2024-12-31",
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test-token"))
	}))
	defer authServer.Close()

	auth := NewAuthManager(authServer.URL, "user", "pass", 1*time.Hour, authServer.Client(), testutil.NewTestLogger())

	client := NewClient(server.URL, auth, server.Client(), testutil.NewTestLogger(), "", "", "", nil)

	resolutions, err := client.GetResolutions(context.Background(), "123456789")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resolutions) != 1 {
		t.Fatalf("expected 1 resolution, got %d", len(resolutions))
	}

	if resolutions[0].ResolutionNumber != "RES001" {
		t.Errorf("expected ResolutionNumber 'RES001', got %q", resolutions[0].ResolutionNumber)
	}
}

func TestClient_GetResolutions_AuthError(t *testing.T) {
	// Create an auth manager that will fail
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer authServer.Close()

	auth := NewAuthManager(authServer.URL, "user", "pass", 1*time.Hour, authServer.Client(), testutil.NewTestLogger())

	client := NewClient("https://api.example.com", auth, &http.Client{}, testutil.NewTestLogger(), "", "", "", nil)

	_, err := client.GetResolutions(context.Background(), "123456789")
	if err == nil {
		t.Fatal("expected error for auth failure")
	}

	if !strings.Contains(err.Error(), "get authentication token") {
		t.Errorf("expected error to mention authentication token, got %q", err.Error())
	}
}

func TestClient_GetResolutions_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("unauthorized"))
	}))
	defer server.Close()

	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("expired-token"))
	}))
	defer authServer.Close()

	auth := NewAuthManager(authServer.URL, "user", "pass", 1*time.Hour, authServer.Client(), testutil.NewTestLogger())

	client := NewClient(server.URL, auth, server.Client(), testutil.NewTestLogger(), "", "", "", nil)

	_, err := client.GetResolutions(context.Background(), "123456789")
	if err == nil {
		t.Fatal("expected error for 401 status")
	}
}

func TestClient_GetResolutions_Non200Status(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer server.Close()

	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("token"))
	}))
	defer authServer.Close()

	auth := NewAuthManager(authServer.URL, "user", "pass", 1*time.Hour, authServer.Client(), testutil.NewTestLogger())

	client := NewClient(server.URL, auth, server.Client(), testutil.NewTestLogger(), "", "", "", nil)

	_, err := client.GetResolutions(context.Background(), "123456789")
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}

	if !strings.Contains(err.Error(), "unexpected status code") {
		t.Errorf("expected error to mention status code, got %q", err.Error())
	}
}

func TestClient_GetResolutions_GzipCompressed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Encoding", "gzip")
		response := numrotResolutionResponse{
			OperationCode:        "100",
			OperationDescription: "Success",
			NumberRangeResponse:  []numrotNumberRange{},
		}

		gzWriter := gzip.NewWriter(w)
		defer gzWriter.Close()
		json.NewEncoder(gzWriter).Encode(response)
	}))
	defer server.Close()

	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("token"))
	}))
	defer authServer.Close()

	auth := NewAuthManager(authServer.URL, "user", "pass", 1*time.Hour, authServer.Client(), testutil.NewTestLogger())

	client := NewClient(server.URL, auth, server.Client(), testutil.NewTestLogger(), "", "", "", nil)

	_, err := client.GetResolutions(context.Background(), "123456789")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClient_GetResolutions_Non100OperationCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := numrotResolutionResponse{
			OperationCode:        "200",
			OperationDescription: "Error occurred",
			NumberRangeResponse:  []numrotNumberRange{},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("token"))
	}))
	defer authServer.Close()

	auth := NewAuthManager(authServer.URL, "user", "pass", 1*time.Hour, authServer.Client(), testutil.NewTestLogger())

	client := NewClient(server.URL, auth, server.Client(), testutil.NewTestLogger(), "", "", "", nil)

	_, err := client.GetResolutions(context.Background(), "123456789")
	if err == nil {
		t.Fatal("expected error for non-100 operation code")
	}

	if !strings.Contains(err.Error(), "numrot API error") {
		t.Errorf("expected error to mention numrot API error, got %q", err.Error())
	}
}

func TestClient_GetResolutions_UnmarshalError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("token"))
	}))
	defer authServer.Close()

	auth := NewAuthManager(authServer.URL, "user", "pass", 1*time.Hour, authServer.Client(), testutil.NewTestLogger())

	client := NewClient(server.URL, auth, server.Client(), testutil.NewTestLogger(), "", "", "", nil)

	_, err := client.GetResolutions(context.Background(), "123456789")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}

	if !strings.Contains(err.Error(), "unmarshal response") {
		t.Errorf("expected error to mention unmarshal, got %q", err.Error())
	}
}

func TestClient_transformToResolution_Success(t *testing.T) {
	client := &Client{log: testutil.NewTestLogger()}

	nr := numrotNumberRange{
		ResolutionNumber: "RES001",
		ResolutionDate:   "2024-01-15",
		Prefix:           "PREF",
		FromNumber:       1,
		ToNumber:         100,
		ValidDateFrom:    "2024-01-01",
		ValidDateTo:      "2024-12-31",
	}

	res, err := client.transformToResolution(nr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.ResolutionNumber != "RES001" {
		t.Errorf("expected ResolutionNumber 'RES001', got %q", res.ResolutionNumber)
	}

	expectedDate, _ := time.Parse("2006-01-02", "2024-01-15")
	if !res.ResolutionDate.Equal(expectedDate) {
		t.Errorf("expected ResolutionDate %v, got %v", expectedDate, res.ResolutionDate)
	}
}

func TestClient_transformToResolution_InvalidResolutionDate(t *testing.T) {
	client := &Client{log: testutil.NewTestLogger()}

	nr := numrotNumberRange{
		ResolutionDate: "invalid-date",
	}

	_, err := client.transformToResolution(nr)
	if err == nil {
		t.Fatal("expected error for invalid resolution date")
	}

	if !strings.Contains(err.Error(), "parse resolution date") {
		t.Errorf("expected error to mention parse resolution date, got %q", err.Error())
	}
}

func TestClient_transformToResolution_InvalidValidDateFrom(t *testing.T) {
	client := &Client{log: testutil.NewTestLogger()}

	nr := numrotNumberRange{
		ResolutionDate: "2024-01-01",
		ValidDateFrom:  "invalid-date",
	}

	_, err := client.transformToResolution(nr)
	if err == nil {
		t.Fatal("expected error for invalid valid date from")
	}

	if !strings.Contains(err.Error(), "parse valid date from") {
		t.Errorf("expected error to mention parse valid date from, got %q", err.Error())
	}
}

func TestClient_transformToResolution_InvalidValidDateTo(t *testing.T) {
	client := &Client{log: testutil.NewTestLogger()}

	nr := numrotNumberRange{
		ResolutionDate: "2024-01-01",
		ValidDateFrom:  "2024-01-01",
		ValidDateTo:    "invalid-date",
	}

	_, err := client.transformToResolution(nr)
	if err == nil {
		t.Fatal("expected error for invalid valid date to")
	}

	if !strings.Contains(err.Error(), "parse valid date to") {
		t.Errorf("expected error to mention parse valid date to, got %q", err.Error())
	}
}

func TestClient_GetResolutions_InvalidDateInResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := numrotResolutionResponse{
			OperationCode:        "100",
			OperationDescription: "Success",
			NumberRangeResponse: []numrotNumberRange{
				{
					ResolutionNumber: "RES001",
					ResolutionDate:   "invalid-date",
					Prefix:           "PREF",
					FromNumber:       1,
					ToNumber:         100,
					ValidDateFrom:    "2024-01-01",
					ValidDateTo:      "2024-12-31",
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("token"))
	}))
	defer authServer.Close()

	auth := NewAuthManager(authServer.URL, "user", "pass", 1*time.Hour, authServer.Client(), testutil.NewTestLogger())

	client := NewClient(server.URL, auth, server.Client(), testutil.NewTestLogger(), "", "", "", nil)

	resolutions, err := client.GetResolutions(context.Background(), "123456789")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Invalid entries should be skipped
	if len(resolutions) != 0 {
		t.Errorf("expected 0 resolutions (invalid entry skipped), got %d", len(resolutions))
	}
}

func TestClient_GetDocuments_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/api/Radian/GetInfoDocument") {
			t.Errorf("expected path to contain /api/Radian/GetInfoDocument, got %s", r.URL.Path)
		}

		response := numrotDocumentResponse{
			Code:    200,
			Message: "Documentos obtenidos con รฉxito",
			Data: []numrotDocument{
				{
					NumeroFactura:     "4009100394926",
					TotalFactura:      283700.00,
					FechaEmision:      "2025-04-24",
					HoraEmision:       "01:14:45",
					TipoFactura:       "01",
					EmisorNit:         "123456987",
					EmisorNombre:      "ALMACENES S.A.",
					ReferenciaFactura: "c1234567890",
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := &Client{
		key:        "test-key",
		secret:     "test-secret",
		radianURL:  server.URL,
		httpClient: server.Client(),
		log:        testutil.NewTestLogger(),
	}

	query := invoice.DocumentQuery{
		CompanyNit:  "860011153",
		InitialDate: "2025-12-01",
		FinalDate:   "2025-12-31",
	}

	documents, err := client.GetDocuments(context.Background(), query)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(documents) != 1 {
		t.Fatalf("expected 1 document, got %d", len(documents))
	}

	if documents[0].OFE != "123456987" {
		t.Errorf("expected OFE '123456987', got %q", documents[0].OFE)
	}

	if documents[0].Proveedor != "ALMACENES S.A." {
		t.Errorf("expected Proveedor 'ALMACENES S.A.', got %q", documents[0].Proveedor)
	}

	if documents[0].Valor != 283700.00 {
		t.Errorf("expected Valor 283700.00, got %f", documents[0].Valor)
	}
}

func TestClient_GetDocuments_NoDocuments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := numrotDocumentResponse{
			Code:    204,
			Message: "No se encontraron documentos con los parรกmetros enviados",
			Data:    []numrotDocument{},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := &Client{
		key:        "test-key",
		secret:     "test-secret",
		radianURL:  server.URL,
		httpClient: server.Client(),
		log:        testutil.NewTestLogger(),
	}

	query := invoice.DocumentQuery{
		CompanyNit:  "860011153",
		InitialDate: "2025-12-01",
		FinalDate:   "2025-12-31",
	}

	documents, err := client.GetDocuments(context.Background(), query)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(documents) != 0 {
		t.Errorf("expected 0 documents, got %d", len(documents))
	}
}

func TestClient_GetDocuments_HTTP204Status(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := &Client{
		key:        "test-key",
		secret:     "test-secret",
		radianURL:  server.URL,
		httpClient: server.Client(),
		log:        testutil.NewTestLogger(),
	}

	query := invoice.DocumentQuery{
		CompanyNit:  "860011153",
		InitialDate: "2025-12-01",
		FinalDate:   "2025-12-31",
	}

	documents, err := client.GetDocuments(context.Background(), query)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(documents) != 0 {
		t.Errorf("expected 0 documents, got %d", len(documents))
	}
}

func TestClient_GetDocuments_MissingKeySecret(t *testing.T) {
	client := &Client{
		key:        "",
		secret:     "",
		radianURL:  "https://api.example.com",
		httpClient: &http.Client{},
		log:        testutil.NewTestLogger(),
	}

	query := invoice.DocumentQuery{
		CompanyNit:  "860011153",
		InitialDate: "2025-12-01",
		FinalDate:   "2025-12-31",
	}

	_, err := client.GetDocuments(context.Background(), query)
	if err == nil {
		t.Fatal("expected error for missing key/secret")
	}

	if !strings.Contains(err.Error(), "key and secret are required") {
		t.Errorf("expected error to mention key and secret, got %q", err.Error())
	}
}

func TestClient_GetDocuments_Non200Status(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer server.Close()

	client := &Client{
		key:        "test-key",
		secret:     "test-secret",
		radianURL:  server.URL,
		httpClient: server.Client(),
		log:        testutil.NewTestLogger(),
	}

	query := invoice.DocumentQuery{
		CompanyNit:  "860011153",
		InitialDate: "2025-12-01",
		FinalDate:   "2025-12-31",
	}

	_, err := client.GetDocuments(context.Background(), query)
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}

	if !strings.Contains(err.Error(), "unexpected status code") {
		t.Errorf("expected error to mention status code, got %q", err.Error())
	}
}

func TestClient_GetDocuments_Non200Code(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := numrotDocumentResponse{
			Code:    500,
			Message: "Error interno",
			Data:    []numrotDocument{},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := &Client{
		key:        "test-key",
		secret:     "test-secret",
		radianURL:  server.URL,
		httpClient: server.Client(),
		log:        testutil.NewTestLogger(),
	}

	query := invoice.DocumentQuery{
		CompanyNit:  "860011153",
		InitialDate: "2025-12-01",
		FinalDate:   "2025-12-31",
	}

	_, err := client.GetDocuments(context.Background(), query)
	if err == nil {
		t.Fatal("expected error for non-200 code")
	}

	if !strings.Contains(err.Error(), "numrot API error") {
		t.Errorf("expected error to mention numrot API error, got %q", err.Error())
	}
}

func TestClient_GetDocuments_UnmarshalError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	client := &Client{
		key:        "test-key",
		secret:     "test-secret",
		radianURL:  server.URL,
		httpClient: server.Client(),
		log:        testutil.NewTestLogger(),
	}

	query := invoice.DocumentQuery{
		CompanyNit:  "860011153",
		InitialDate: "2025-12-01",
		FinalDate:   "2025-12-31",
	}

	_, err := client.GetDocuments(context.Background(), query)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}

	if !strings.Contains(err.Error(), "unmarshal response") {
		t.Errorf("expected error to mention unmarshal, got %q", err.Error())
	}
}

func TestClient_GetDocuments_InvalidDate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := numrotDocumentResponse{
			Code:    200,
			Message: "Success",
			Data: []numrotDocument{
				{
					NumeroFactura: "4009100394926",
					FechaEmision:  "invalid-date",
					EmisorNit:     "123456987",
					EmisorNombre:  "Test",
				},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := &Client{
		key:        "test-key",
		secret:     "test-secret",
		radianURL:  server.URL,
		httpClient: server.Client(),
		log:        testutil.NewTestLogger(),
	}

	query := invoice.DocumentQuery{
		CompanyNit:  "860011153",
		InitialDate: "2025-12-01",
		FinalDate:   "2025-12-31",
	}

	documents, err := client.GetDocuments(context.Background(), query)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Invalid entries should be skipped
	if len(documents) != 0 {
		t.Errorf("expected 0 documents (invalid entry skipped), got %d", len(documents))
	}
}

func TestClient_GetDocuments_HTTPError(t *testing.T) {
	client := &Client{
		key:       "test-key",
		secret:    "test-secret",
		radianURL: "http://invalid-url-that-does-not-exist.local",
		httpClient: &http.Client{
			Timeout: 1, // Very short timeout to force error
		},
		log: testutil.NewTestLogger(),
	}

	query := invoice.DocumentQuery{
		CompanyNit:  "860011153",
		InitialDate: "2025-12-01",
		FinalDate:   "2025-12-31",
	}

	_, err := client.GetDocuments(context.Background(), query)
	if err == nil {
		t.Fatal("expected error for HTTP request failure")
	}

	if !strings.Contains(err.Error(), "execute request") {
		t.Errorf("expected error to mention execute request, got %q", err.Error())
	}
}

func TestClient_transformToDocument_Success(t *testing.T) {
	client := &Client{log: testutil.NewTestLogger()}

	nd := numrotDocument{
		NumeroFactura:     "4009100394926",
		TotalFactura:      283700.00,
		FechaEmision:      "2025-04-24",
		HoraEmision:       "01:14:45",
		TipoFactura:       "01",
		EmisorNit:         "123456987",
		EmisorNombre:      "ALMACENES S.A.",
		ReferenciaFactura: "c1234567890",
	}

	doc, err := client.transformToDocument(nd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if doc.OFE != "123456987" {
		t.Errorf("expected OFE '123456987', got %q", doc.OFE)
	}

	if doc.Proveedor != "ALMACENES S.A." {
		t.Errorf("expected Proveedor 'ALMACENES S.A.', got %q", doc.Proveedor)
	}

	if doc.Tipo != "01" {
		t.Errorf("expected Tipo '01', got %q", doc.Tipo)
	}

	if doc.Valor != 283700.00 {
		t.Errorf("expected Valor 283700.00, got %f", doc.Valor)
	}

	expectedDate, _ := time.Parse("2006-01-02", "2025-04-24")
	if !doc.Fecha.Equal(expectedDate) {
		t.Errorf("expected Fecha %v, got %v", expectedDate, doc.Fecha)
	}
}

func TestClient_transformToDocument_InvalidDate(t *testing.T) {
	client := &Client{log: testutil.NewTestLogger()}

	nd := numrotDocument{
		FechaEmision: "invalid-date",
		EmisorNit:    "123456987",
		EmisorNombre: "Test",
	}

	_, err := client.transformToDocument(nd)
	if err == nil {
		t.Fatal("expected error for invalid date")
	}

	if !strings.Contains(err.Error(), "parse fecha emision") {
		t.Errorf("expected error to mention parse fecha emision, got %q", err.Error())
	}
}

func TestClient_RegisterEvent_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "/api/Radian/SetEvent") {
			t.Errorf("expected path to contain /api/Radian/SetEvent, got %s", r.URL.Path)
		}

		response := numrotSetEventResponse{
			Codigo:          "1000",
			NumeroDocumento: "FAC12345",
			Resultado: []numrotEventResult{
				{
					TipoEvento:      "030",
					Mensaje:         "Procesado Correctamente.",
					MensajeError:    "",
					CodigoRespuesta: "1000",
				},
			},
			MensajeError: "",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := &Client{
		key:        "test-key",
		secret:     "test-secret",
		radianURL:  server.URL,
		httpClient: server.Client(),
		log:        testutil.NewTestLogger(),
	}

	evt := event.Event{
		EventType:               event.EventTypeAcuse,
		DocumentNumber:          "FAC12345",
		NombreGenerador:         "Mauricio",
		ApellidoGenerador:       "Alemรกn",
		IdentificacionGenerador: "1061239585",
		EventGenerationDate:     time.Now(),
	}

	result, err := client.RegisterEvent(context.Background(), evt, "123456789", "PROVEDOR NOMBRE")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("expected result, got nil")
	}

	if result.Code != "1000" {
		t.Errorf("expected Code '1000', got %q", result.Code)
	}

	if result.NumeroDocumento != "FAC12345" {
		t.Errorf("expected NumeroDocumento 'FAC12345', got %q", result.NumeroDocumento)
	}

	if len(result.Resultado) != 1 {
		t.Fatalf("expected 1 resultado, got %d", len(result.Resultado))
	}

	if result.Resultado[0].TipoEvento != "030" {
		t.Errorf("expected TipoEvento '030', got %q", result.Resultado[0].TipoEvento)
	}
}

func TestClient_RegisterEvent_WithRejectionCode(t *testing.T) {
	rejectionCode := event.RejectionCodeInconsistencias
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := numrotSetEventResponse{
			Codigo:          "1000",
			NumeroDocumento: "FAC12345",
			Resultado: []numrotEventResult{
				{
					TipoEvento:      "031",
					Mensaje:         "Procesado Correctamente.",
					MensajeError:    "",
					CodigoRespuesta: "1000",
				},
			},
			MensajeError: "",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := &Client{
		key:        "test-key",
		secret:     "test-secret",
		radianURL:  server.URL,
		httpClient: server.Client(),
		log:        testutil.NewTestLogger(),
	}

	evt := event.Event{
		EventType:               event.EventTypeReclamo,
		DocumentNumber:          "FAC12345",
		NombreGenerador:         "Mauricio",
		ApellidoGenerador:       "Alemรกn",
		IdentificacionGenerador: "1061239585",
		RejectionCode:           &rejectionCode,
		EventGenerationDate:     time.Now(),
	}

	result, err := client.RegisterEvent(context.Background(), evt, "123456789", "PROVEDOR NOMBRE")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("expected result, got nil")
	}

	if result.Resultado[0].TipoEvento != "031" {
		t.Errorf("expected TipoEvento '031', got %q", result.Resultado[0].TipoEvento)
	}
}

func TestClient_RegisterEvent_EventRejected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := numrotSetEventResponse{
			Codigo:          "1001",
			NumeroDocumento: "FAC12345",
			Resultado: []numrotEventResult{
				{
					TipoEvento:      "032",
					Mensaje:         "Procesado Correctamente.",
					MensajeError:    "",
					CodigoRespuesta: "1000",
				},
				{
					TipoEvento:      "033",
					Mensaje:         "Evento rechazado.",
					MensajeError:    "Documento con errores en campos mandatorios.",
					CodigoRespuesta: "1001",
				},
			},
			MensajeError: "",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := &Client{
		key:        "test-key",
		secret:     "test-secret",
		radianURL:  server.URL,
		httpClient: server.Client(),
		log:        testutil.NewTestLogger(),
	}

	evt := event.Event{
		EventType:               event.EventTypeReciboBien,
		DocumentNumber:          "FAC12345",
		NombreGenerador:         "Mauricio",
		ApellidoGenerador:       "Alemรกn",
		IdentificacionGenerador: "1061239585",
		EventGenerationDate:     time.Now(),
	}

	result, err := client.RegisterEvent(context.Background(), evt, "123456789", "PROVEDOR NOMBRE")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("expected result, got nil")
	}

	if result.Code != "1001" {
		t.Errorf("expected Code '1001', got %q", result.Code)
	}

	if len(result.Resultado) != 2 {
		t.Fatalf("expected 2 resultados, got %d", len(result.Resultado))
	}
}

func TestClient_RegisterEvent_DocumentNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		response := numrotSetEventErrorResponse{
			Code:  400,
			Error: "No se encontrรณ el documento en la base de datos",
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := &Client{
		key:        "test-key",
		secret:     "test-secret",
		radianURL:  server.URL,
		httpClient: server.Client(),
		log:        testutil.NewTestLogger(),
	}

	evt := event.Event{
		EventType:               event.EventTypeAcuse,
		DocumentNumber:          "FAC99999",
		NombreGenerador:         "Mauricio",
		ApellidoGenerador:       "Alemรกn",
		IdentificacionGenerador: "1061239585",
		EventGenerationDate:     time.Now(),
	}

	_, err := client.RegisterEvent(context.Background(), evt, "123456789", "PROVEDOR NOMBRE")
	if err == nil {
		t.Fatal("expected error for document not found")
	}

	if !strings.Contains(err.Error(), "document not found") {
		t.Errorf("expected error to mention document not found, got %q", err.Error())
	}
}

func TestClient_RegisterEvent_MissingKeySecret(t *testing.T) {
	client := &Client{
		key:        "",
		secret:     "",
		radianURL:  "https://api.example.com",
		httpClient: &http.Client{},
		log:        testutil.NewTestLogger(),
	}

	evt := event.Event{
		EventType:               event.EventTypeAcuse,
		DocumentNumber:          "FAC12345",
		NombreGenerador:         "Mauricio",
		ApellidoGenerador:       "Alemรกn",
		IdentificacionGenerador: "1061239585",
		EventGenerationDate:     time.Now(),
	}

	_, err := client.RegisterEvent(context.Background(), evt, "123456789", "PROVEDOR NOMBRE")
	if err == nil {
		t.Fatal("expected error for missing key/secret")
	}

	if !strings.Contains(err.Error(), "key and secret are required") {
		t.Errorf("expected error to mention key and secret, got %q", err.Error())
	}
}

func TestClient_RegisterEvent_InvalidEventType(t *testing.T) {
	client := &Client{
		key:        "test-key",
		secret:     "test-secret",
		radianURL:  "https://api.example.com",
		httpClient: &http.Client{},
		log:        testutil.NewTestLogger(),
	}

	evt := event.Event{
		EventType:               event.EventType("INVALID"),
		DocumentNumber:          "FAC12345",
		NombreGenerador:         "Mauricio",
		ApellidoGenerador:       "Alemรกn",
		IdentificacionGenerador: "1061239585",
		EventGenerationDate:     time.Now(),
	}

	_, err := client.RegisterEvent(context.Background(), evt, "123456789", "PROVEDOR NOMBRE")
	if err == nil {
		t.Fatal("expected error for invalid event type")
	}

	if !strings.Contains(err.Error(), "translate event type") {
		t.Errorf("expected error to mention translate event type, got %q", err.Error())
	}
}

func TestClient_RegisterEvent_Non200Status(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("server error"))
	}))
	defer server.Close()

	client := &Client{
		key:        "test-key",
		secret:     "test-secret",
		radianURL:  server.URL,
		httpClient: server.Client(),
		log:        testutil.NewTestLogger(),
	}

	evt := event.Event{
		EventType:               event.EventTypeAcuse,
		DocumentNumber:          "FAC12345",
		NombreGenerador:         "Mauricio",
		ApellidoGenerador:       "Alemรกn",
		IdentificacionGenerador: "1061239585",
		EventGenerationDate:     time.Now(),
	}

	_, err := client.RegisterEvent(context.Background(), evt, "123456789", "PROVEDOR NOMBRE")
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}

	if !strings.Contains(err.Error(), "unexpected status code") {
		t.Errorf("expected error to mention status code, got %q", err.Error())
	}
}

func TestClient_RegisterEvent_UnmarshalError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	client := &Client{
		key:        "test-key",
		secret:     "test-secret",
		radianURL:  server.URL,
		httpClient: server.Client(),
		log:        testutil.NewTestLogger(),
	}

	evt := event.Event{
		EventType:               event.EventTypeAcuse,
		DocumentNumber:          "FAC12345",
		NombreGenerador:         "Mauricio",
		ApellidoGenerador:       "Alemรกn",
		IdentificacionGenerador: "1061239585",
		EventGenerationDate:     time.Now(),
	}

	_, err := client.RegisterEvent(context.Background(), evt, "123456789", "PROVEDOR NOMBRE")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}

	if !strings.Contains(err.Error(), "unmarshal response") {
		t.Errorf("expected error to mention unmarshal, got %q", err.Error())
	}
}

func TestClient_RegisterEvent_HTTPError(t *testing.T) {
	client := &Client{
		key:       "test-key",
		secret:    "test-secret",
		radianURL: "http://invalid-url-that-does-not-exist.local",
		httpClient: &http.Client{
			Timeout: 1, // Very short timeout to force error
		},
		log: testutil.NewTestLogger(),
	}

	evt := event.Event{
		EventType:               event.EventTypeAcuse,
		DocumentNumber:          "FAC12345",
		NombreGenerador:         "Mauricio",
		ApellidoGenerador:       "Alemรกn",
		IdentificacionGenerador: "1061239585",
		EventGenerationDate:     time.Now(),
	}

	_, err := client.RegisterEvent(context.Background(), evt, "123456789", "PROVEDOR NOMBRE")
	if err == nil {
		t.Fatal("expected error for HTTP request failure")
	}

	if !strings.Contains(err.Error(), "execute request") {
		t.Errorf("expected error to mention execute request, got %q", err.Error())
	}
}

func TestClient_RegisterEvent_DocumentNotFound_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid json"))
	}))
	defer server.Close()

	client := &Client{
		key:        "test-key",
		secret:     "test-secret",
		radianURL:  server.URL,
		httpClient: server.Client(),
		log:        testutil.NewTestLogger(),
	}

	evt := event.Event{
		EventType:               event.EventTypeAcuse,
		DocumentNumber:          "FAC99999",
		NombreGenerador:         "Mauricio",
		ApellidoGenerador:       "Alemรกn",
		IdentificacionGenerador: "1061239585",
		EventGenerationDate:     time.Now(),
	}

	_, err := client.RegisterEvent(context.Background(), evt, "123456789", "PROVEDOR NOMBRE")
	if err == nil {
		t.Fatal("expected error for document not found")
	}

	if !strings.Contains(err.Error(), "document not found") {
		t.Errorf("expected error to mention document not found, got %q", err.Error())
	}
}

func TestClient_RegisterDocument_Success_FC(t *testing.T) {
	// Setup auth server
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test-token"))
	}))
	defer authServer.Close()

	// Setup document registration server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("expected Authorization header with Bearer token")
		}

		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		// Use new format with _size and _preview
		previewContent := map[string]interface{}{
			"StatusCode":        "200",
			"DocumentNumber":    "SETT5604",
			"TrackId":           "lote-20240115-103000",
			"Uuid":              "test-uuid-12345",
			"StatusMessage":     "La Factura electrónica SETT5604, ha sido autorizada.",
			"StatusDescription": "Procesado Correctamente.",
			"ErrorMessage":      "Procesado Correctamente.",
			"ErrorReason":       []string{},
			"Warnings":          []string{},
		}
		previewJSON, _ := json.Marshal(previewContent)

		resp := map[string]interface{}{
			"_size":    1000,
			"_preview": string(previewJSON),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	auth := NewAuthManager(authServer.URL, "user", "pass", 1*time.Hour, authServer.Client(), testutil.NewTestLogger())
	client := &Client{
		baseURL:    server.URL,
		auth:       auth,
		httpClient: server.Client(),
		log:        testutil.NewTestLogger(),
	}

	req := invoice.DocumentRegistrationRequest{
		Documentos: invoice.DocumentsByType{
			FC: []invoice.OpenETLDocument{
				{
					TdeCodigo:               "01",
					TopCodigo:               "01",
					OfeIdentificacion:       "860011153",
					AdqIdentificacion:       "900123456",
					RfaPrefijo:              "SETT",
					RfaResolucion:           "18760000001",
					CdoConsecutivo:          "5604",
					CdoFecha:                "2023-06-29",
					CdoHora:                 "14:37:00",
					MonCodigo:               "COP",
					CdoValorSinImpuestos:    "176471.00",
					CdoImpuestos:            "33529.00",
					CdoTotal:                "210000.00",
					CdoRetencionesSugeridas: "0.00",
					CdoAnticipo:             "0.00",
					CdoRedondeo:             "0.00",
					Items: []invoice.OpenETLItem{
						{
							DdoTipoItem:       "BIEN",
							DdoSecuencia:      "1",
							DdoCodigo:         "PROD001",
							DdoDescripcionUno: "Producto de ejemplo",
							DdoCantidad:       "10",
							UndCodigo:         "UN",
							DdoValorUnitario:  "17647.10",
							DdoTotal:          "176471.00",
						},
					},
					Tributos: []invoice.OpenETLTributo{
						{
							DdoSecuencia: "1",
							TriCodigo:    "01",
							IidValor:     "33529.00",
							IidPorcentaje: &invoice.OpenETLTributoPorcentaje{
								IidBase:       "176471.00",
								IidPorcentaje: "19.00",
							},
						},
					},
				},
			},
			NC: []invoice.OpenETLDocument{},
			ND: []invoice.OpenETLDocument{},
		},
	}

	resp, err := client.RegisterDocument(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Message == "" {
		t.Errorf("expected non-empty message, got empty")
	}

	if resp.Lote == "" {
		t.Errorf("expected non-empty lote, got empty")
	}

	if len(resp.DocumentosProcesados) != 1 {
		t.Errorf("expected 1 processed document, got %d", len(resp.DocumentosProcesados))
	}

	if resp.DocumentosProcesados[0].RfaPrefijo != "SETT" {
		t.Errorf("expected prefijo 'SETT', got %q", resp.DocumentosProcesados[0].RfaPrefijo)
	}

	if resp.DocumentosProcesados[0].CdoConsecutivo != "5604" {
		t.Errorf("expected consecutivo '5604', got %q", resp.DocumentosProcesados[0].CdoConsecutivo)
	}

	if len(resp.DocumentosFallidos) != 0 {
		t.Errorf("expected 0 failed documents, got %d", len(resp.DocumentosFallidos))
	}
}

func TestClient_RegisterDocument_NoDocuments(t *testing.T) {
	// Setup auth server
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test-token"))
	}))
	defer authServer.Close()

	auth := NewAuthManager(authServer.URL, "user", "pass", 1*time.Hour, authServer.Client(), testutil.NewTestLogger())
	client := &Client{
		baseURL:    "http://example.com",
		auth:       auth,
		httpClient: authServer.Client(),
		log:        testutil.NewTestLogger(),
	}

	req := invoice.DocumentRegistrationRequest{
		Documentos: invoice.DocumentsByType{
			FC: []invoice.OpenETLDocument{},
			NC: []invoice.OpenETLDocument{},
			ND: []invoice.OpenETLDocument{},
		},
	}

	_, err := client.RegisterDocument(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for no documents")
	}

	if !strings.Contains(err.Error(), "no documents provided") {
		t.Errorf("expected error to mention no documents, got %q", err.Error())
	}
}

func TestClient_RegisterDocument_AuthError(t *testing.T) {
	// Setup failing auth server
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer authServer.Close()

	auth := NewAuthManager(authServer.URL, "user", "pass", 1*time.Hour, authServer.Client(), testutil.NewTestLogger())
	client := &Client{
		baseURL:    "http://example.com",
		auth:       auth,
		httpClient: authServer.Client(),
		log:        testutil.NewTestLogger(),
	}

	req := invoice.DocumentRegistrationRequest{
		Documentos: invoice.DocumentsByType{
			FC: []invoice.OpenETLDocument{
				{
					TdeCodigo:               "01",
					OfeIdentificacion:       "860011153",
					AdqIdentificacion:       "900123456",
					RfaPrefijo:              "SETT",
					RfaResolucion:           "18760000001",
					CdoConsecutivo:          "5604",
					CdoFecha:                "2023-06-29",
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

	_, err := client.RegisterDocument(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for authentication failure")
	}

	if !strings.Contains(err.Error(), "get authentication token") {
		t.Errorf("expected error to mention authentication, got %q", err.Error())
	}
}

// Tests for helper functions - DIAN FAJ25, FAK48, FAJ43b compliance

func TestGetStringOrDefault_WithValue(t *testing.T) {
	value := "Test Value"
	result := getStringOrDefault(&value, "default")
	if result != "Test Value" {
		t.Errorf("expected 'Test Value', got %q", result)
	}
}

func TestGetStringOrDefault_WithNil(t *testing.T) {
	result := getStringOrDefault(nil, "default")
	if result != "default" {
		t.Errorf("expected 'default', got %q", result)
	}
}

func TestGetStringOrDefault_WithEmptyString(t *testing.T) {
	value := ""
	result := getStringOrDefault(&value, "default")
	if result != "default" {
		t.Errorf("expected 'default' for empty string, got %q", result)
	}
}

func TestBuildCustomerPhysicalLocation_WithAllFields(t *testing.T) {
	razonSocial := "EMPRESA CLIENTE S.A.S."
	direccion := "Calle 100 #15-20 Oficina 501"
	municipioCodigo := "001"
	municipioNombre := "Bogotá D.C."
	departamentoCodigo := "11"
	departamentoNombre := "Bogotá D.C."
	paisCodigo := "CO"
	paisNombre := "Colombia"

	doc := invoice.OpenETLDocument{
		AdqRazonSocial:        &razonSocial,
		AdqDireccion:          &direccion,
		AdqMunicipioCodigo:    &municipioCodigo,
		AdqMunicipioNombre:    &municipioNombre,
		AdqDepartamentoCodigo: &departamentoCodigo,
		AdqDepartamentoNombre: &departamentoNombre,
		AdqPaisCodigo:         &paisCodigo,
		AdqPaisNombre:         &paisNombre,
	}

	location := buildCustomerPhysicalLocation(doc)

	if location == nil {
		t.Fatal("expected location, got nil")
	}

	// ID should be DIVIPOLA code: dep_codigo + mun_codigo = "11" + "001" = "11001"
	if location.ID != "11001" {
		t.Errorf("expected ID '11001' (DIVIPOLA: dep + mun), got %q", location.ID)
	}

	if location.CityName != "Bogotá D.C." {
		t.Errorf("expected CityName 'Bogotá D.C.', got %q", location.CityName)
	}

	if location.CountrySubentityCode != "11" {
		t.Errorf("expected CountrySubentityCode '11', got %q", location.CountrySubentityCode)
	}

	if location.CountrySubentity != "Bogotá D.C." {
		t.Errorf("expected CountrySubentity 'Bogotá D.C.', got %q", location.CountrySubentity)
	}

	if location.Line != "Calle 100 #15-20 Oficina 501" {
		t.Errorf("expected Line address, got %q", location.Line)
	}

	if location.IdentificationCode != "CO" {
		t.Errorf("expected IdentificationCode 'CO', got %q", location.IdentificationCode)
	}

	if location.Name != "Colombia" {
		t.Errorf("expected Name 'Colombia', got %q", location.Name)
	}
}

func TestBuildCustomerPhysicalLocation_WithDefaultCountry(t *testing.T) {
	direccion := "Calle 100 #15-20"
	municipioCodigo := "11001"

	doc := invoice.OpenETLDocument{
		AdqDireccion:       &direccion,
		AdqMunicipioCodigo: &municipioCodigo,
		// No country fields - should default to CO/Colombia
	}

	location := buildCustomerPhysicalLocation(doc)

	if location == nil {
		t.Fatal("expected location, got nil")
	}

	if location.IdentificationCode != "CO" {
		t.Errorf("expected default IdentificationCode 'CO', got %q", location.IdentificationCode)
	}

	if location.Name != "Colombia" {
		t.Errorf("expected default Name 'Colombia', got %q", location.Name)
	}
}

func TestBuildSupplierPhysicalLocation_WithAllFields(t *testing.T) {
	razonSocial := "EMPRESA EMISORA S.A.S."
	direccion := "Carrera 7 #32-16"
	municipioCodigo := "11001"
	municipioNombre := "Bogotá D.C."
	departamentoCodigo := "11"
	departamentoNombre := "Bogotá D.C."

	doc := invoice.OpenETLDocument{
		OfeRazonSocial:        &razonSocial,
		OfeDireccion:          &direccion,
		OfeMunicipioCodigo:    &municipioCodigo,
		OfeMunicipioNombre:    &municipioNombre,
		OfeDepartamentoCodigo: &departamentoCodigo,
		OfeDepartamentoNombre: &departamentoNombre,
	}

	location := buildSupplierPhysicalLocation(doc)

	if location == nil {
		t.Fatal("expected location, got nil")
	}

	if location.ID != "11001" {
		t.Errorf("expected ID '11001', got %q", location.ID)
	}

	if location.Line != "Carrera 7 #32-16" {
		t.Errorf("expected Line address, got %q", location.Line)
	}

	// Should always default to CO/Colombia for supplier
	if location.IdentificationCode != "CO" {
		t.Errorf("expected IdentificationCode 'CO', got %q", location.IdentificationCode)
	}
}

func TestClient_RegisterDocument_WithLocationData(t *testing.T) {
	// Setup auth server
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test-token"))
	}))
	defer authServer.Close()

	// Setup document registration server that validates location data
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Decode the request body to verify location data is included
		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}

		// Check that AccountingCustomerParty has PhysicalLocation
		customerParty, ok := reqBody["AccountingCustomerParty"].(map[string]interface{})
		if !ok {
			t.Error("expected AccountingCustomerParty in request")
		}

		// Verify Name is the razon social, not the NIT
		if name, ok := customerParty["Name"].(string); ok {
			if name != "EMPRESA CLIENTE S.A.S." {
				t.Errorf("expected customer Name 'EMPRESA CLIENTE S.A.S.', got %q", name)
			}
		}

		// Verify PhysicalLocation exists
		if _, ok := customerParty["PhysicalLocation"]; !ok {
			t.Error("expected PhysicalLocation in AccountingCustomerParty")
		}

		// Use new format with _size and _preview
		previewContent := map[string]interface{}{
			"StatusCode":        "200",
			"DocumentNumber":    "SETT5604",
			"TrackId":           "lote-20240115-103000",
			"Uuid":              "test-uuid-12345",
			"StatusMessage":     "La Factura electrónica SETT5604, ha sido autorizada.",
			"StatusDescription": "Procesado Correctamente.",
			"ErrorMessage":      "Procesado Correctamente.",
			"ErrorReason":       []string{},
			"Warnings":          []string{},
		}
		previewJSON, _ := json.Marshal(previewContent)

		resp := map[string]interface{}{
			"_size":    1000,
			"_preview": string(previewJSON),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	auth := NewAuthManager(authServer.URL, "user", "pass", 1*time.Hour, authServer.Client(), testutil.NewTestLogger())
	client := &Client{
		baseURL:    server.URL,
		auth:       auth,
		httpClient: server.Client(),
		log:        testutil.NewTestLogger(),
	}

	// Create document with full location data
	adqRazonSocial := "EMPRESA CLIENTE S.A.S."
	adqDireccion := "Calle 100 #15-20 Oficina 501"
	adqMunicipioCodigo := "11001"
	adqMunicipioNombre := "Bogotá D.C."
	adqDepartamentoCodigo := "11"
	adqDepartamentoNombre := "Bogotá D.C."
	ofeRazonSocial := "EMPRESA EMISORA S.A.S."
	ofeDireccion := "Carrera 7 #32-16"

	req := invoice.DocumentRegistrationRequest{
		Documentos: invoice.DocumentsByType{
			FC: []invoice.OpenETLDocument{
				{
					TdeCodigo:               "01",
					TopCodigo:               "01",
					OfeIdentificacion:       "860011153",
					OfeRazonSocial:          &ofeRazonSocial,
					OfeDireccion:            &ofeDireccion,
					AdqIdentificacion:       "900123456",
					AdqRazonSocial:          &adqRazonSocial,
					AdqDireccion:            &adqDireccion,
					AdqMunicipioCodigo:      &adqMunicipioCodigo,
					AdqMunicipioNombre:      &adqMunicipioNombre,
					AdqDepartamentoCodigo:   &adqDepartamentoCodigo,
					AdqDepartamentoNombre:   &adqDepartamentoNombre,
					RfaPrefijo:              "SETT",
					RfaResolucion:           "18760000001",
					CdoConsecutivo:          "5604",
					CdoFecha:                "2023-06-29",
					CdoHora:                 "14:37:00",
					MonCodigo:               "COP",
					CdoValorSinImpuestos:    "176471.00",
					CdoImpuestos:            "33529.00",
					CdoTotal:                "210000.00",
					CdoRetencionesSugeridas: "0.00",
					CdoAnticipo:             "0.00",
					CdoRedondeo:             "0.00",
					Items: []invoice.OpenETLItem{
						{
							DdoTipoItem:       "BIEN",
							DdoSecuencia:      "1",
							DdoCodigo:         "PROD001",
							DdoDescripcionUno: "Producto de ejemplo",
							DdoCantidad:       "10",
							UndCodigo:         "UN",
							DdoValorUnitario:  "17647.10",
							DdoTotal:          "176471.00",
						},
					},
					Tributos: []invoice.OpenETLTributo{
						{
							DdoSecuencia: "1",
							TriCodigo:    "01",
							IidValor:     "33529.00",
							IidPorcentaje: &invoice.OpenETLTributoPorcentaje{
								IidBase:       "176471.00",
								IidPorcentaje: "19.00",
							},
						},
					},
				},
			},
			NC: []invoice.OpenETLDocument{},
			ND: []invoice.OpenETLDocument{},
		},
	}

	resp, err := client.RegisterDocument(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Message == "" {
		t.Errorf("expected non-empty message, got empty")
	}

	if len(resp.DocumentosProcesados) == 0 {
		t.Errorf("expected at least 1 processed document, got %d", len(resp.DocumentosProcesados))
	}
}

func TestClient_RegisterDocument_WithNotesAndOrderReference(t *testing.T) {
	// Setup auth server
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test-token"))
	}))
	defer authServer.Close()

	// Setup document registration server that validates notes and order reference
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Decode the request body to verify notes and order reference
		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}

		// Check that Note array is included
		if notes, ok := reqBody["Note"].([]interface{}); ok {
			if len(notes) != 2 {
				t.Errorf("expected 2 notes, got %d", len(notes))
			}
		} else {
			t.Error("expected Note array in request")
		}

		// Check that OrderReference is included
		if orderRef, ok := reqBody["OrderReference"].(map[string]interface{}); ok {
			if id, ok := orderRef["ID"].(string); ok {
				if id != "PO-2025-001" {
					t.Errorf("expected OrderReference.ID 'PO-2025-001', got %q", id)
				}
			}
		} else {
			t.Error("expected OrderReference in request")
		}

		// Use new format with _size and _preview
		previewContent := map[string]interface{}{
			"StatusCode":        "200",
			"DocumentNumber":    "SETT5604",
			"TrackId":           "lote-20240115-103000",
			"Uuid":              "test-uuid-12345",
			"StatusMessage":     "La Factura electrónica SETT5604, ha sido autorizada.",
			"StatusDescription": "Procesado Correctamente.",
			"ErrorMessage":      "Procesado Correctamente.",
			"ErrorReason":       []string{},
			"Warnings":          []string{},
		}
		previewJSON, _ := json.Marshal(previewContent)

		resp := map[string]interface{}{
			"_size":    1000,
			"_preview": string(previewJSON),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	auth := NewAuthManager(authServer.URL, "user", "pass", 1*time.Hour, authServer.Client(), testutil.NewTestLogger())
	client := &Client{
		baseURL:    server.URL,
		auth:       auth,
		httpClient: server.Client(),
		log:        testutil.NewTestLogger(),
	}

	req := invoice.DocumentRegistrationRequest{
		Documentos: invoice.DocumentsByType{
			FC: []invoice.OpenETLDocument{
				{
					TdeCodigo:               "01",
					TopCodigo:               "01",
					OfeIdentificacion:       "860011153",
					AdqIdentificacion:       "900123456",
					RfaPrefijo:              "SETT",
					RfaResolucion:           "18760000001",
					CdoConsecutivo:          "5604",
					CdoFecha:                "2023-06-29",
					CdoHora:                 "14:37:00",
					MonCodigo:               "COP",
					CdoValorSinImpuestos:    "176471.00",
					CdoImpuestos:            "33529.00",
					CdoTotal:                "210000.00",
					CdoRetencionesSugeridas: "0.00",
					CdoAnticipo:             "0.00",
					CdoRedondeo:             "0.00",
					Note:                    []string{"Note 1", "Note 2"},
					OrderReference:          &invoice.OpenETLOrderReference{ID: "PO-2025-001"},
					Items: []invoice.OpenETLItem{
						{
							DdoTipoItem:       "BIEN",
							DdoSecuencia:      "1",
							DdoCodigo:         "PROD001",
							DdoDescripcionUno: "Producto de ejemplo",
							DdoCantidad:       "10",
							UndCodigo:         "UN",
							DdoValorUnitario:  "17647.10",
							DdoTotal:          "176471.00",
						},
					},
					Tributos: []invoice.OpenETLTributo{
						{
							DdoSecuencia: "1",
							TriCodigo:    "01",
							IidValor:     "33529.00",
							IidPorcentaje: &invoice.OpenETLTributoPorcentaje{
								IidBase:       "176471.00",
								IidPorcentaje: "19.00",
							},
						},
					},
				},
			},
			NC: []invoice.OpenETLDocument{},
			ND: []invoice.OpenETLDocument{},
		},
	}

	resp, err := client.RegisterDocument(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Message == "" {
		t.Errorf("expected non-empty message, got empty")
	}

	if len(resp.DocumentosProcesados) == 0 {
		t.Errorf("expected at least 1 processed document, got %d", len(resp.DocumentosProcesados))
	}
}

func TestNormalizeMonetaryValue(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string returns 0.00",
			input:    "",
			expected: "0.00",
		},
		{
			name:     "zero returns 0.00",
			input:    "0",
			expected: "0.00",
		},
		{
			name:     "0.00 returns 0.00",
			input:    "0.00",
			expected: "0.00",
		},
		{
			name:     "valid amount remains unchanged",
			input:    "1500000.00",
			expected: "1500000.00",
		},
		{
			name:     "valid amount without decimals remains unchanged",
			input:    "1500000",
			expected: "1500000",
		},
		{
			name:     "small amount remains unchanged",
			input:    "10.50",
			expected: "10.50",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeMonetaryValue(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeMonetaryValue(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestTransformOpenETLToNumrot_NC_WithIVA(t *testing.T) {
	client := &Client{log: testutil.NewTestLogger()}

	doc := invoice.OpenETLDocument{
		TdeCodigo:             "03",
		TopCodigo:             "22",
		OfeIdentificacion:     "860011153-3",
		AdqIdentificacion:     "900123456",
		RfaPrefijo:            "NC",
		RfaResolucion:         "",
		CdoConsecutivo:        "5604",
		CdoFecha:              "2023-06-29",
		CdoHora:               "14:37:00",
		MonCodigo:             "COP",
		CdoValorSinImpuestos:   "176471.00",
		CdoImpuestos:          "33529.00",
		CdoTotal:              "210000.00",
		CdoRetencionesSugeridas: "0.00",
		CdoAnticipo:           "0.00",
		CdoRedondeo:           "0.00",
		Items: []invoice.OpenETLItem{
			{
				DdoTipoItem:       "BIEN",
				DdoSecuencia:      "1",
				DdoCodigo:         "PROD001",
				DdoDescripcionUno: "Producto de ejemplo",
				DdoCantidad:       "10",
				UndCodigo:         "UN",
				DdoValorUnitario:  "17647.10",
				DdoTotal:          "176471.00",
			},
		},
		Tributos: []invoice.OpenETLTributo{
			{
				DdoSecuencia: "1",
				TriCodigo:    "01",
				IidValor:     "33529.00",
				IidPorcentaje: &invoice.OpenETLTributoPorcentaje{
					IidBase:       "176471.00",
					IidPorcentaje: "19.00",
				},
			},
		},
	}

	numrotInv, err := client.transformOpenETLToNumrot(context.Background(), doc, "NC")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify TaxTotal is included for NC with IVA
	if len(numrotInv.TaxTotal) == 0 {
		t.Error("expected TaxTotal to be included for NC document with IVA")
	}

	// Verify TaxTotal contains IVA
	if len(numrotInv.TaxTotal) > 0 {
		taxTotal := numrotInv.TaxTotal[0]
		if taxTotal.TaxAmount != "33529.00" {
			t.Errorf("expected TaxAmount to be 33529.00, got %s", taxTotal.TaxAmount)
		}
		if len(taxTotal.TaxSubtotal) == 0 {
			t.Error("expected TaxSubtotal to be included")
		} else {
			subtotal := taxTotal.TaxSubtotal[0]
			if subtotal.ID != "01" {
				t.Errorf("expected tax ID to be 01 (IVA), got %s", subtotal.ID)
			}
			if subtotal.TaxAmount != "33529.00" {
				t.Errorf("expected TaxAmount in subtotal to be 33529.00, got %s", subtotal.TaxAmount)
			}
		}
	}

	// Verify LegalMonetaryTotal fields for NC with IVA
	if numrotInv.LegalMonetaryTotal.TaxExclusiveAmount != "176471.00" {
		t.Errorf("expected TaxExclusiveAmount to be 176471.00 (cdo_valor_sin_impuestos), got %s", numrotInv.LegalMonetaryTotal.TaxExclusiveAmount)
	}
	if numrotInv.LegalMonetaryTotal.TaxInclusiveAmount != "210000.00" {
		t.Errorf("expected TaxInclusiveAmount to be 210000.00 (cdo_total), got %s", numrotInv.LegalMonetaryTotal.TaxInclusiveAmount)
	}
	if numrotInv.LegalMonetaryTotal.PayableAmount != "210000.00" {
		t.Errorf("expected PayableAmount to be 210000.00 (cdo_total), got %s", numrotInv.LegalMonetaryTotal.PayableAmount)
	}

	// Verify InvoiceLine includes TaxTotal for items with taxes
	if len(numrotInv.InvoiceLine) > 0 {
		line := numrotInv.InvoiceLine[0]
		if len(line.TaxTotal) == 0 {
			t.Error("expected InvoiceLine TaxTotal to be included for NC item with IVA")
		} else {
			lineTaxTotal := line.TaxTotal[0]
			if lineTaxTotal.TaxAmount != "33529.00" {
				t.Errorf("expected InvoiceLine TaxAmount to be 33529.00, got %s", lineTaxTotal.TaxAmount)
			}
		}
	}
}

func TestTransformOpenETLToNumrot_ND_WithIVA(t *testing.T) {
	client := &Client{log: testutil.NewTestLogger()}

	doc := invoice.OpenETLDocument{
		TdeCodigo:             "04",
		TopCodigo:             "32",
		OfeIdentificacion:     "860011153-3",
		AdqIdentificacion:     "900123456",
		RfaPrefijo:            "ND",
		RfaResolucion:         "",
		CdoConsecutivo:        "5605",
		CdoFecha:              "2023-06-30",
		CdoHora:               "15:00:00",
		MonCodigo:             "COP",
		CdoValorSinImpuestos:   "100000.00",
		CdoImpuestos:          "19000.00",
		CdoTotal:              "119000.00",
		CdoRetencionesSugeridas: "0.00",
		CdoAnticipo:           "0.00",
		CdoRedondeo:           "0.00",
		Items: []invoice.OpenETLItem{
			{
				DdoTipoItem:       "BIEN",
				DdoSecuencia:      "1",
				DdoCodigo:         "PROD002",
				DdoDescripcionUno: "Producto adicional",
				DdoCantidad:       "5",
				UndCodigo:         "UN",
				DdoValorUnitario:  "20000.00",
				DdoTotal:          "100000.00",
			},
		},
		Tributos: []invoice.OpenETLTributo{
			{
				DdoSecuencia: "1",
				TriCodigo:    "01",
				IidValor:     "19000.00",
				IidPorcentaje: &invoice.OpenETLTributoPorcentaje{
					IidBase:       "100000.00",
					IidPorcentaje: "19.00",
				},
			},
		},
	}

	numrotInv, err := client.transformOpenETLToNumrot(context.Background(), doc, "ND")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify TaxTotal is included for ND with IVA
	if len(numrotInv.TaxTotal) == 0 {
		t.Error("expected TaxTotal to be included for ND document with IVA")
	}

	// Verify TaxTotal contains IVA
	if len(numrotInv.TaxTotal) > 0 {
		taxTotal := numrotInv.TaxTotal[0]
		if taxTotal.TaxAmount != "19000.00" {
			t.Errorf("expected TaxAmount to be 19000.00, got %s", taxTotal.TaxAmount)
		}
		if len(taxTotal.TaxSubtotal) == 0 {
			t.Error("expected TaxSubtotal to be included")
		} else {
			subtotal := taxTotal.TaxSubtotal[0]
			if subtotal.ID != "01" {
				t.Errorf("expected tax ID to be 01 (IVA), got %s", subtotal.ID)
			}
			if subtotal.TaxAmount != "19000.00" {
				t.Errorf("expected TaxAmount in subtotal to be 19000.00, got %s", subtotal.TaxAmount)
			}
		}
	}

	// Verify LegalMonetaryTotal fields for ND with IVA
	if numrotInv.LegalMonetaryTotal.TaxExclusiveAmount != "100000.00" {
		t.Errorf("expected TaxExclusiveAmount to be 100000.00 (cdo_valor_sin_impuestos), got %s", numrotInv.LegalMonetaryTotal.TaxExclusiveAmount)
	}
	if numrotInv.LegalMonetaryTotal.TaxInclusiveAmount != "119000.00" {
		t.Errorf("expected TaxInclusiveAmount to be 119000.00 (cdo_total), got %s", numrotInv.LegalMonetaryTotal.TaxInclusiveAmount)
	}
	if numrotInv.LegalMonetaryTotal.PayableAmount != "119000.00" {
		t.Errorf("expected PayableAmount to be 119000.00 (cdo_total), got %s", numrotInv.LegalMonetaryTotal.PayableAmount)
	}

	// Verify InvoiceLine includes TaxTotal for items with taxes
	if len(numrotInv.InvoiceLine) > 0 {
		line := numrotInv.InvoiceLine[0]
		if len(line.TaxTotal) == 0 {
			t.Error("expected InvoiceLine TaxTotal to be included for ND item with IVA")
		} else {
			lineTaxTotal := line.TaxTotal[0]
			if lineTaxTotal.TaxAmount != "19000.00" {
				t.Errorf("expected InvoiceLine TaxAmount to be 19000.00, got %s", lineTaxTotal.TaxAmount)
			}
		}
	}
}

func TestTransformOpenETLToNumrot_NC_WithoutIVA(t *testing.T) {
	client := &Client{log: testutil.NewTestLogger()}

	doc := invoice.OpenETLDocument{
		TdeCodigo:             "03",
		TopCodigo:             "22",
		OfeIdentificacion:     "860011153-3",
		AdqIdentificacion:     "900123456",
		RfaPrefijo:            "NC",
		RfaResolucion:         "",
		CdoConsecutivo:        "5606",
		CdoFecha:              "2023-07-01",
		CdoHora:               "10:00:00",
		MonCodigo:             "COP",
		CdoValorSinImpuestos:   "50000.00",
		CdoImpuestos:          "0.00",
		CdoTotal:              "50000.00",
		CdoRetencionesSugeridas: "0.00",
		CdoAnticipo:           "0.00",
		CdoRedondeo:           "0.00",
		Items: []invoice.OpenETLItem{
			{
				DdoTipoItem:       "BIEN",
				DdoSecuencia:      "1",
				DdoCodigo:         "PROD003",
				DdoDescripcionUno: "Producto sin IVA",
				DdoCantidad:       "1",
				UndCodigo:         "UN",
				DdoValorUnitario:  "50000.00",
				DdoTotal:          "50000.00",
			},
		},
		Tributos: []invoice.OpenETLTributo{},
	}

	numrotInv, err := client.transformOpenETLToNumrot(context.Background(), doc, "NC")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify TaxTotal is NOT included when cdo_impuestos is "0.00" (omitempty will exclude it)
	// This is expected behavior - TaxTotal should only be included when there are actual taxes
	if len(numrotInv.TaxTotal) > 0 {
		t.Error("expected TaxTotal to be empty (not included) when cdo_impuestos is 0.00")
	}

	// Verify LegalMonetaryTotal fields for NC without IVA
	// TaxExclusiveAmount must be "0.00" when there are no taxes
	if numrotInv.LegalMonetaryTotal.TaxExclusiveAmount != "0.00" {
		t.Errorf("expected TaxExclusiveAmount to be 0.00 (no taxes), got %s", numrotInv.LegalMonetaryTotal.TaxExclusiveAmount)
	}
	if numrotInv.LegalMonetaryTotal.TaxInclusiveAmount != "50000.00" {
		t.Errorf("expected TaxInclusiveAmount to be 50000.00 (cdo_total), got %s", numrotInv.LegalMonetaryTotal.TaxInclusiveAmount)
	}
}
