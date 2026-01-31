package numrot

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"3tcapital/ms_facturacion_core/internal/core/acquirer"
	"3tcapital/ms_facturacion_core/internal/core/event"
	"3tcapital/ms_facturacion_core/internal/core/invoice"
	"3tcapital/ms_facturacion_core/internal/core/resolution"
)

// getMapKeys returns all keys from a map as a slice of strings
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// extractPrefixAndConsecutive extracts prefix and consecutive from a document number.
// Example: "SETT5608" -> prefix: "SETT", consecutive: "5608"
// Example: "FC12345" -> prefix: "FC", consecutive: "12345"
func extractPrefixAndConsecutive(documentNumber string) (prefijo, consecutivo string) {
	if documentNumber == "" {
		return "", ""
	}

	// Try to find where the numeric part starts
	for i := 0; i < len(documentNumber); i++ {
		if documentNumber[i] >= '0' && documentNumber[i] <= '9' {
			prefijo = documentNumber[:i]
			consecutivo = documentNumber[i:]
			return prefijo, consecutivo
		}
	}

	// If no numbers found, return the whole string as prefix
	return documentNumber, ""
}

// combineAcquirerEmails combines emails from multiple sources into a semicolon-separated string.
// It collects emails from:
//   - contactEmail: Email from AccountingContact (if provided)
//   - adqCorreo: Primary email from acquirer (if provided)
//   - adqCorreosNotificacion: Comma-separated notification emails from acquirer (if provided)
//
// The function removes duplicates, trims whitespace, and joins all emails with semicolons.
func combineAcquirerEmails(contactEmail string, adqCorreo *string, adqCorreosNotificacion *string) string {
	emailSet := make(map[string]bool)
	var emails []string

	// Add contact email if provided
	if contactEmail != "" {
		email := strings.TrimSpace(contactEmail)
		if email != "" && !emailSet[email] {
			emailSet[email] = true
			emails = append(emails, email)
		}
	}

	// Add primary acquirer email if provided
	if adqCorreo != nil && *adqCorreo != "" {
		email := strings.TrimSpace(*adqCorreo)
		if email != "" && !emailSet[email] {
			emailSet[email] = true
			emails = append(emails, email)
		}
	}

	// Add notification emails if provided (split by comma)
	if adqCorreosNotificacion != nil && *adqCorreosNotificacion != "" {
		notificationEmails := strings.Split(*adqCorreosNotificacion, ",")
		for _, email := range notificationEmails {
			email = strings.TrimSpace(email)
			if email != "" && !emailSet[email] {
				emailSet[email] = true
				emails = append(emails, email)
			}
		}
	}

	// Join all emails with semicolons
	return strings.Join(emails, ";")
}

// Client implements the InvoiceProvider interface for Numrot.
type Client struct {
	baseURL              string
	dsBaseURL            string // Base URL specifically for DS (Documento Soporte) documents. If empty, uses baseURL
	auth                 *AuthManager
	httpClient           HTTPClient
	log                  *slog.Logger
	key                  string
	secret               string
	radianURL            string
	acquirerRepo         acquirer.Repository // Optional: nil if database not configured
	concurrencyLimiter   *ConcurrentRequestLimiter
	rateLimiter          *RateLimiter
	batchSize            int
	maxConcurrentBatches int
	circuitBreaker       *CircuitBreaker
	// Resolution query settings (for testing environments)
	resolutionsEnabled   bool
	hardcodedInvoiceAuth string
	hardcodedStartDate   string
	hardcodedEndDate     string
	hardcodedPrefix      string
	hardcodedFrom        string
	hardcodedTo          string
	// InvoicePeriod settings for NC without reference
	ncInvoicePeriodStartDate string
	ncInvoicePeriodStartTime string
	ncInvoicePeriodEndDate   string
	ncInvoicePeriodEndTime   string
}

// NewClient creates a new Numrot API client.
func NewClient(baseURL string, auth *AuthManager, httpClient HTTPClient, log *slog.Logger, key, secret, radianURL string, acquirerRepo acquirer.Repository) invoice.Provider {
	return NewClientWithConcurrency(baseURL, auth, httpClient, log, key, secret, radianURL, acquirerRepo, 1000, 50, 100)
}

// NewClientWithConcurrency creates a new Numrot API client with concurrency configuration.
func NewClientWithConcurrency(baseURL string, auth *AuthManager, httpClient HTTPClient, log *slog.Logger, key, secret, radianURL string, acquirerRepo acquirer.Repository, maxConcurrent int, batchSize int, rateLimitRPS int) invoice.Provider {
	return NewClientWithConfig(baseURL, auth, httpClient, log, key, secret, radianURL, acquirerRepo, maxConcurrent, batchSize, rateLimitRPS, true, "", "", "", "", "", "")
}

// NewClientWithConfig creates a new Numrot API client with full configuration including resolution settings.
func NewClientWithConfig(baseURL string, auth *AuthManager, httpClient HTTPClient, log *slog.Logger, key, secret, radianURL string, acquirerRepo acquirer.Repository, maxConcurrent int, batchSize int, rateLimitRPS int, resolutionsEnabled bool, hardcodedInvoiceAuth, hardcodedStartDate, hardcodedEndDate, hardcodedPrefix, hardcodedFrom, hardcodedTo string) invoice.Provider {
	return NewClientWithDSBaseURL(baseURL, "", auth, httpClient, log, key, secret, radianURL, acquirerRepo, maxConcurrent, batchSize, rateLimitRPS, resolutionsEnabled, hardcodedInvoiceAuth, hardcodedStartDate, hardcodedEndDate, hardcodedPrefix, hardcodedFrom, hardcodedTo, "", "", "", "")
}

// NewClientWithDSBaseURL creates a new Numrot API client with full configuration including DS-specific base URL.
func NewClientWithDSBaseURL(baseURL, dsBaseURL string, auth *AuthManager, httpClient HTTPClient, log *slog.Logger, key, secret, radianURL string, acquirerRepo acquirer.Repository, maxConcurrent int, batchSize int, rateLimitRPS int, resolutionsEnabled bool, hardcodedInvoiceAuth, hardcodedStartDate, hardcodedEndDate, hardcodedPrefix, hardcodedFrom, hardcodedTo, ncInvoicePeriodStartDate, ncInvoicePeriodStartTime, ncInvoicePeriodEndDate, ncInvoicePeriodEndTime string) invoice.Provider {
	// If radianURL is not provided, use baseURL
	if radianURL == "" {
		radianURL = baseURL
	}

	// Validate and create limiters
	if err := ValidateConcurrencyConfig(maxConcurrent, batchSize); err != nil {
		log.Warn("Invalid concurrency config, using defaults", "error", err)
		maxConcurrent = 1000
		batchSize = 50
	}

	concurrencyLimiter := NewConcurrentRequestLimiter(maxConcurrent)
	rateLimiter := NewRateLimiter(rateLimitRPS)
	maxConcurrentBatches := maxConcurrent / batchSize
	if maxConcurrentBatches < 1 {
		maxConcurrentBatches = 1
	}

	// Create circuit breaker: open after 50% failure rate or 10 consecutive failures
	// Cooldown period: 30 seconds before attempting half-open
	circuitBreaker := NewCircuitBreaker(10, 0.5, 30*time.Second)

	return &Client{
		baseURL:                  baseURL,
		dsBaseURL:                dsBaseURL, // If empty, will use baseURL as fallback
		auth:                     auth,
		httpClient:               httpClient,
		log:                      log,
		key:                      key,
		secret:                   secret,
		radianURL:                radianURL,
		acquirerRepo:             acquirerRepo,
		concurrencyLimiter:       concurrencyLimiter,
		rateLimiter:              rateLimiter,
		batchSize:                batchSize,
		maxConcurrentBatches:     maxConcurrentBatches,
		circuitBreaker:           circuitBreaker,
		resolutionsEnabled:       resolutionsEnabled,
		hardcodedInvoiceAuth:     hardcodedInvoiceAuth,
		hardcodedStartDate:       hardcodedStartDate,
		hardcodedEndDate:         hardcodedEndDate,
		hardcodedPrefix:          hardcodedPrefix,
		hardcodedFrom:            hardcodedFrom,
		hardcodedTo:              hardcodedTo,
		ncInvoicePeriodStartDate: ncInvoicePeriodStartDate,
		ncInvoicePeriodStartTime: ncInvoicePeriodStartTime,
		ncInvoicePeriodEndDate:   ncInvoicePeriodEndDate,
		ncInvoicePeriodEndTime:   ncInvoicePeriodEndTime,
	}
}

// numrotResolutionResponse represents the response structure from Numrot API.
type numrotResolutionResponse struct {
	OperationCode        string              `json:"OperationCode"`
	OperationDescription string              `json:"OperationDescription"`
	NumberRangeResponse  []numrotNumberRange `json:"NumberRangeResponse"`
}

// numrotNumberRange represents a single resolution range in Numrot's response.
type numrotNumberRange struct {
	ResolutionNumber string `json:"ResolutionNumber"`
	ResolutionDate   string `json:"ResolutionDate"`
	Prefix           string `json:"Prefix"`
	FromNumber       int64  `json:"FromNumber"`
	ToNumber         int64  `json:"ToNumber"`
	ValidDateFrom    string `json:"ValidDateFrom"`
	ValidDateTo      string `json:"ValidDateTo"`
}

// GetResolutions retrieves all active resolutions for a given NIT from Numrot.
// If resolutions are disabled via configuration, returns hardcoded resolution data.
func (c *Client) GetResolutions(ctx context.Context, nit string) ([]resolution.Resolution, error) {
	// If resolutions are disabled, return hardcoded values
	if !c.resolutionsEnabled {
		c.log.Debug("Resolutions query disabled - using hardcoded values",
			"nit", nit,
			"hardcoded_invoice_auth", c.hardcodedInvoiceAuth,
			"hardcoded_prefix", c.hardcodedPrefix,
		)

		// Return empty slice if no hardcoded values configured
		if c.hardcodedInvoiceAuth == "" || c.hardcodedPrefix == "" {
			c.log.Warn("Resolutions disabled but hardcoded values not configured - returning empty slice",
				"nit", nit,
			)
			return []resolution.Resolution{}, nil
		}

		// Parse hardcoded dates
		validDateFrom, err := time.Parse("2006-01-02", c.hardcodedStartDate)
		if err != nil {
			c.log.Warn("Invalid hardcoded start date, using default",
				"start_date", c.hardcodedStartDate,
				"error", err,
			)
			validDateFrom = time.Date(2019, 1, 19, 0, 0, 0, 0, time.UTC)
		}

		validDateTo, err := time.Parse("2006-01-02", c.hardcodedEndDate)
		if err != nil {
			c.log.Warn("Invalid hardcoded end date, using default",
				"end_date", c.hardcodedEndDate,
				"error", err,
			)
			validDateTo = time.Date(2030, 1, 19, 0, 0, 0, 0, time.UTC)
		}

		// Parse hardcoded number range
		fromNumber := int64(1)
		if c.hardcodedFrom != "" {
			if parsed, err := strconv.ParseInt(c.hardcodedFrom, 10, 64); err == nil {
				fromNumber = parsed
			}
		}

		toNumber := int64(5000000)
		if c.hardcodedTo != "" {
			if parsed, err := strconv.ParseInt(c.hardcodedTo, 10, 64); err == nil {
				toNumber = parsed
			}
		}

		// Return hardcoded resolution
		return []resolution.Resolution{
			{
				ResolutionNumber: c.hardcodedInvoiceAuth,
				Prefix:           c.hardcodedPrefix,
				FromNumber:       fromNumber,
				ToNumber:         toNumber,
				ValidDateFrom:    validDateFrom,
				ValidDateTo:      validDateTo,
			},
		}, nil
	}

	// Normal flow: query resolutions from API
	token, err := c.auth.GetToken(ctx)
	if err != nil {
		c.log.Error("Failed to get Numrot authentication token", "error", err)
		return nil, fmt.Errorf("get authentication token: %w", err)
	}

	url := fmt.Sprintf("%s/api/Resoluciones/%s", c.baseURL, nit)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	c.log.Debug("Requesting resolutions from Numrot", "nit", nit, "url", url)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.log.Error("Failed to execute request to Numrot", "error", err, "url", url)
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	// Handle gzip compression if present
	var reader io.Reader = resp.Body
	if strings.Contains(resp.Header.Get("Content-Encoding"), "gzip") {
		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			c.log.Error("Failed to create gzip reader", "error", err)
			return nil, fmt.Errorf("create gzip reader: %w", err)
		}
		defer gzipReader.Close()
		reader = gzipReader
		c.log.Debug("Response is gzip compressed, decompressing")
	}

	body, err := io.ReadAll(reader)
	if err != nil {
		c.log.Error("Failed to read response body from Numrot", "error", err, "status", resp.StatusCode)
		return nil, fmt.Errorf("read response body: %w", err)
	}

	c.log.Debug("Numrot API response", "status", resp.StatusCode, "body_length", len(body), "content_encoding", resp.Header.Get("Content-Encoding"))

	if resp.StatusCode == http.StatusUnauthorized {
		// Token might be expired, clear cache and retry once
		c.auth.ClearToken()
		c.log.Warn("Token expired or invalid, clearing cache", "status", resp.StatusCode, "body", string(body))
		return nil, fmt.Errorf("authentication failed: token expired or invalid")
	}

	if resp.StatusCode != http.StatusOK {
		c.log.Error("Numrot API returned non-OK status", "status", resp.StatusCode, "body", string(body))
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var numrotResp numrotResolutionResponse
	if err := json.Unmarshal(body, &numrotResp); err != nil {
		c.log.Error("Failed to unmarshal Numrot response", "error", err, "body", string(body))
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	c.log.Debug("Numrot API response parsed", "operationCode", numrotResp.OperationCode, "operationDescription", numrotResp.OperationDescription, "resolutions_count", len(numrotResp.NumberRangeResponse))

	// Check operation code
	if numrotResp.OperationCode != "100" {
		c.log.Warn("Numrot API returned error code", "operationCode", numrotResp.OperationCode, "operationDescription", numrotResp.OperationDescription)
		return nil, fmt.Errorf("numrot API error: %s (code: %s)", numrotResp.OperationDescription, numrotResp.OperationCode)
	}

	// Transform Numrot response to domain entities
	resolutions := make([]resolution.Resolution, 0, len(numrotResp.NumberRangeResponse))
	for _, nr := range numrotResp.NumberRangeResponse {
		res, err := c.transformToResolution(nr)
		if err != nil {
			c.log.Warn("Failed to transform resolution", "error", err, "resolutionNumber", nr.ResolutionNumber)
			continue // Skip invalid entries but continue processing others
		}
		resolutions = append(resolutions, res)
	}

	return resolutions, nil
}

// transformToResolution converts a Numrot number range to a domain Resolution.
func (c *Client) transformToResolution(nr numrotNumberRange) (resolution.Resolution, error) {
	// Parse dates
	dateLayout := "2006-01-02"

	resolutionDate, err := time.Parse(dateLayout, nr.ResolutionDate)
	if err != nil {
		return resolution.Resolution{}, fmt.Errorf("parse resolution date: %w", err)
	}

	validDateFrom, err := time.Parse(dateLayout, nr.ValidDateFrom)
	if err != nil {
		return resolution.Resolution{}, fmt.Errorf("parse valid date from: %w", err)
	}

	validDateTo, err := time.Parse(dateLayout, nr.ValidDateTo)
	if err != nil {
		return resolution.Resolution{}, fmt.Errorf("parse valid date to: %w", err)
	}

	return resolution.Resolution{
		ResolutionNumber: nr.ResolutionNumber,
		ResolutionDate:   resolutionDate,
		Prefix:           nr.Prefix,
		FromNumber:       nr.FromNumber,
		ToNumber:         nr.ToNumber,
		ValidDateFrom:    validDateFrom,
		ValidDateTo:      validDateTo,
	}, nil
}

// numrotDocumentResponse represents the response structure from Numrot Radian API.
type numrotDocumentResponse struct {
	Code    int              `json:"Code"`
	Message string           `json:"Message"`
	Data    []numrotDocument `json:"Data"`
}

// numrotDocument represents a single document from Numrot.
type numrotDocument struct {
	NumeroFactura       string                 `json:"NumeroFactura"`
	ImpuestoFactura     float64                `json:"ImpuestoFactura"`
	TotalAntesImpuestos float64                `json:"TotalAntesImpuestos"`
	TotalBaseImponible  float64                `json:"TotalBaseImponible"`
	TotalMasImpuestos   float64                `json:"TotalMasImpuestos"`
	TotalDescuentos     float64                `json:"TotalDescuentos"`
	TotalFactura        float64                `json:"TotalFactura"`
	AdquirienteNit      string                 `json:"AdquirienteNit"`
	AdquirienteNombre   string                 `json:"AdquirienteNombre"`
	AdquirienteCorreo   string                 `json:"AdquirienteCorreo"`
	FechaEmision        string                 `json:"FechaEmision"`
	HoraEmision         string                 `json:"HoraEmision"`
	FechaVencimiento    string                 `json:"FechaVencimiento"`
	TipoFactura         string                 `json:"TipoFactura"`
	CUFE                string                 `json:"CUFE"`
	ReferenciaFactura   string                 `json:"ReferenciaFactura"`
	EmisorNombre        string                 `json:"EmisorNombre"`
	EmisorNit           string                 `json:"EmisorNit"`
	Detalles            []numrotDocumentDetail `json:"Detalles"`
	UrlPDF              string                 `json:"UrlPDF,omitempty"`
	UrlXML              string                 `json:"UrlXML,omitempty"`
}

// numrotDocumentDetail represents a document detail/item.
type numrotDocumentDetail struct {
	DescripcionItem    string  `json:"DescripcionItem"`
	CodigoItem         string  `json:"CodigoItem"`
	UnidadMedida       string  `json:"UnidadMedida"`
	Cantidad           float64 `json:"Cantidad"`
	PrecioUnitario     float64 `json:"PrecioUnitario"`
	PorcentajeImpuesto float64 `json:"PorcentajeImpuesto"`
	TotalImpuestos     float64 `json:"TotalImpuestos"`
	TotalDescuentos    float64 `json:"TotalDescuentos"`
	ValorTotalLinea    float64 `json:"ValorTotalLinea"`
}

// numrotDocumentRequest represents the request structure for Numrot Radian API.
type numrotDocumentRequest struct {
	Key         string `json:"Key"`
	Secret      string `json:"Secret"`
	CompanyNit  string `json:"CompanyNit"`
	InitialDate string `json:"InitialDate"`
	FinalDate   string `json:"FinalDate"`
}

// numrotDocumentByNumberRequest represents the request structure for Numrot Radian GetDocumentByNumber API.
type numrotDocumentByNumberRequest struct {
	Key            string `json:"Key"`
	Secret         string `json:"Secret"`
	CompanyNit     string `json:"CompanyNit"`
	DocumentNumber string `json:"DocumentNumber"`
	SupplierNit    string `json:"SupplierNit"`
}

// numrotReceivedDocumentsResponse represents the response structure from Numrot Radian DocumentsReceived API.
type numrotReceivedDocumentsResponse struct {
	Data []numrotReceivedDocument `json:"Data"`
}

// numrotReceivedDocument represents a single received document from Numrot DocumentsReceived API.
type numrotReceivedDocument struct {
	OFE         string `json:"ofe"`
	Proveedor   string `json:"proveedor"`
	Tipo        string `json:"tipo"`
	Prefijo     string `json:"prefijo"`
	Consecutivo string `json:"consecutivo"`
	CUFE        string `json:"cufe"`
	Fecha       string `json:"fecha"`
	Hora        string `json:"hora"`
	Valor       string `json:"valor"`
	UrlPDF      string `json:"UrlPDF,omitempty"`
	UrlXML      string `json:"UrlXML,omitempty"`
}

// GetDocuments retrieves documents/invoices from Numrot Radian API.
func (c *Client) GetDocuments(ctx context.Context, query invoice.DocumentQuery) ([]invoice.Document, error) {
	if c.key == "" || c.secret == "" {
		return nil, fmt.Errorf("key and secret are required for document queries")
	}

	url := fmt.Sprintf("%s/api/Radian/GetInfoDocument", c.radianURL)

	reqBody := numrotDocumentRequest{
		Key:         c.key,
		Secret:      c.secret,
		CompanyNit:  query.CompanyNit,
		InitialDate: query.InitialDate,
		FinalDate:   query.FinalDate,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	c.log.Debug("Requesting documents from Numrot Radian", "companyNit", query.CompanyNit, "url", url)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.log.Error("Failed to execute request to Numrot Radian", "error", err, "url", url)
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.log.Error("Failed to read response body from Numrot Radian", "error", err, "status", resp.StatusCode)
		return nil, fmt.Errorf("read response body: %w", err)
	}

	c.log.Debug("Numrot Radian API response", "status", resp.StatusCode, "body_length", len(body))

	// Handle 204 (no documents found) as a valid response
	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == 204 {
		c.log.Debug("No documents found", "status", resp.StatusCode)
		return []invoice.Document{}, nil
	}

	if resp.StatusCode != http.StatusOK {
		c.log.Error("Numrot Radian API returned non-OK status", "status", resp.StatusCode, "body", string(body))
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var numrotResp numrotDocumentResponse
	if err := json.Unmarshal(body, &numrotResp); err != nil {
		c.log.Error("Failed to unmarshal Numrot Radian response", "error", err, "body", string(body))
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	c.log.Debug("Numrot Radian API response parsed", "code", numrotResp.Code, "message", numrotResp.Message, "documents_count", len(numrotResp.Data))

	// Check response code (200 = success, 204 = no documents)
	if numrotResp.Code == 204 {
		return []invoice.Document{}, nil
	}

	if numrotResp.Code != 200 {
		c.log.Warn("Numrot Radian API returned error code", "code", numrotResp.Code, "message", numrotResp.Message)
		return nil, fmt.Errorf("numrot API error: %s (code: %d)", numrotResp.Message, numrotResp.Code)
	}

	// Transform Numrot response to domain entities
	documents := make([]invoice.Document, 0, len(numrotResp.Data))
	for _, nd := range numrotResp.Data {
		doc, err := c.transformToDocument(nd)
		if err != nil {
			c.log.Warn("Failed to transform document", "error", err, "numeroFactura", nd.NumeroFactura)
			continue // Skip invalid entries but continue processing others
		}
		documents = append(documents, doc)
	}

	return documents, nil
}

// GetDocumentByNumber retrieves a document/invoice by document number from Numrot Radian API.
func (c *Client) GetDocumentByNumber(ctx context.Context, query invoice.DocumentByNumberQuery) ([]invoice.Document, error) {
	if c.key == "" || c.secret == "" {
		return nil, fmt.Errorf("key and secret are required for document queries")
	}

	url := fmt.Sprintf("%s/api/Radian/GetDocumentByNumber", c.radianURL)

	reqBody := numrotDocumentByNumberRequest{
		Key:            c.key,
		Secret:         c.secret,
		CompanyNit:     query.CompanyNit,
		DocumentNumber: query.DocumentNumber,
		SupplierNit:    query.SupplierNit,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	c.log.Debug("Requesting document by number from Numrot Radian", "companyNit", query.CompanyNit, "documentNumber", query.DocumentNumber, "url", url)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.log.Error("Failed to execute request to Numrot Radian", "error", err, "url", url)
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.log.Error("Failed to read response body from Numrot Radian", "error", err, "status", resp.StatusCode)
		return nil, fmt.Errorf("read response body: %w", err)
	}

	c.log.Debug("Numrot Radian API response", "status", resp.StatusCode, "body_length", len(body))

	// Handle 204 (no documents found) as a valid response
	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == 204 {
		c.log.Debug("No documents found", "status", resp.StatusCode)
		return []invoice.Document{}, nil
	}

	if resp.StatusCode != http.StatusOK {
		c.log.Error("Numrot Radian API returned non-OK status", "status", resp.StatusCode, "body", string(body))
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var numrotResp numrotDocumentResponse
	if err := json.Unmarshal(body, &numrotResp); err != nil {
		c.log.Error("Failed to unmarshal Numrot Radian response", "error", err, "body", string(body))
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	c.log.Debug("Numrot Radian API response parsed", "code", numrotResp.Code, "message", numrotResp.Message, "documents_count", len(numrotResp.Data))

	// Check response code (200 = success, 204 = no documents)
	if numrotResp.Code == 204 {
		return []invoice.Document{}, nil
	}

	if numrotResp.Code != 200 {
		c.log.Warn("Numrot Radian API returned error code", "code", numrotResp.Code, "message", numrotResp.Message)
		return nil, fmt.Errorf("numrot API error: %s (code: %d)", numrotResp.Message, numrotResp.Code)
	}

	// Transform Numrot response to domain entities
	documents := make([]invoice.Document, 0, len(numrotResp.Data))
	for _, nd := range numrotResp.Data {
		doc, err := c.transformToDocument(nd)
		if err != nil {
			c.log.Warn("Failed to transform document", "error", err, "numeroFactura", nd.NumeroFactura)
			continue // Skip invalid entries but continue processing others
		}
		documents = append(documents, doc)
	}

	return documents, nil
}

// GetReceivedDocuments retrieves received documents/invoices from Numrot Radian API.
func (c *Client) GetReceivedDocuments(ctx context.Context, query invoice.DocumentQuery) ([]invoice.Document, error) {
	if c.key == "" || c.secret == "" {
		return nil, fmt.Errorf("key and secret are required for document queries")
	}

	url := fmt.Sprintf("%s/api/Radian/DocumentsReceived", c.radianURL)

	reqBody := numrotDocumentRequest{
		Key:         c.key,
		Secret:      c.secret,
		CompanyNit:  query.CompanyNit,
		InitialDate: query.InitialDate,
		FinalDate:   query.FinalDate,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	c.log.Debug("Requesting received documents from Numrot Radian", "companyNit", query.CompanyNit, "url", url)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.log.Error("Failed to execute request to Numrot Radian", "error", err, "url", url)
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.log.Error("Failed to read response body from Numrot Radian", "error", err, "status", resp.StatusCode)
		return nil, fmt.Errorf("read response body: %w", err)
	}

	c.log.Debug("Numrot Radian API response", "status", resp.StatusCode, "body_length", len(body))

	// Log raw response body (truncated if too long for debugging)
	bodyStr := string(body)
	if len(bodyStr) > 1000 {
		c.log.Debug("Raw response body (truncated)", "body", bodyStr[:1000]+"...", "full_length", len(bodyStr))
	} else {
		c.log.Debug("Raw response body", "body", bodyStr)
	}

	// Handle 204 (no documents found) as a valid response
	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == 204 {
		c.log.Debug("No received documents found", "status", resp.StatusCode)
		return []invoice.Document{}, nil
	}

	if resp.StatusCode != http.StatusOK {
		c.log.Error("Numrot Radian API returned non-OK status", "status", resp.StatusCode, "body", string(body))
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	var numrotResp numrotReceivedDocumentsResponse
	if err := json.Unmarshal(body, &numrotResp); err != nil {
		c.log.Error("Failed to unmarshal Numrot Radian response", "error", err, "body", string(body))
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	// Validate response structure
	if numrotResp.Data == nil {
		c.log.Warn("Numrot Radian API response has nil Data array", "body", bodyStr)
		return []invoice.Document{}, nil
	}

	c.log.Debug("Numrot Radian API response parsed", "documents_count", len(numrotResp.Data))

	// Log unmarshaled structure for debugging
	if len(numrotResp.Data) > 0 {
		c.log.Debug("Sample document from response",
			"ofe", numrotResp.Data[0].OFE,
			"proveedor", numrotResp.Data[0].Proveedor,
			"tipo", numrotResp.Data[0].Tipo,
			"consecutivo", numrotResp.Data[0].Consecutivo,
			"urlPDF", numrotResp.Data[0].UrlPDF,
			"urlXML", numrotResp.Data[0].UrlXML)
	}

	// Transform Numrot response to domain entities
	documents := make([]invoice.Document, 0, len(numrotResp.Data))
	transformationSuccessCount := 0
	transformationFailureCount := 0

	for i, nd := range numrotResp.Data {
		doc, err := c.transformReceivedDocumentToDocument(nd)
		if err != nil {
			transformationFailureCount++
			c.log.Warn("Failed to transform received document",
				"error", err,
				"index", i,
				"ofe", nd.OFE,
				"proveedor", nd.Proveedor,
				"consecutivo", nd.Consecutivo,
				"prefijo", nd.Prefijo,
				"fecha", nd.Fecha,
				"valor", nd.Valor)
			continue // Skip invalid entries but continue processing others
		}
		transformationSuccessCount++
		documents = append(documents, doc)
	}

	// Log transformation summary
	c.log.Info("Document transformation summary",
		"total_received", len(numrotResp.Data),
		"successful", transformationSuccessCount,
		"failed", transformationFailureCount)

	// Return error if all documents failed transformation
	if len(numrotResp.Data) > 0 && transformationSuccessCount == 0 {
		c.log.Error("All documents failed transformation", "total", len(numrotResp.Data), "failures", transformationFailureCount)
		return nil, fmt.Errorf("all %d documents failed transformation", len(numrotResp.Data))
	}

	return documents, nil
}

// GetDocumentInfo obtiene información de un documento por CUFE desde Numrot DocumentInfo API.
// Este método consulta el endpoint /api/DocumentInfo/{nit}/{cufe} que retorna información
// detallada del documento identificado por su CUFE.
func (c *Client) GetDocumentInfo(ctx context.Context, nit, cufe string) (*numrotDocumentInfoResponse, error) {
	// 1. Obtener token de autenticación (reutiliza AuthManager existente)
	token, err := c.auth.GetToken(ctx)
	if err != nil {
		c.log.Error("Failed to get Numrot auth token", "error", err)
		return nil, fmt.Errorf("get authentication token: %w", err)
	}

	// 2. Construir URL: {baseURL}/api/DocumentInfo/{nit}/{cufe}
	url := fmt.Sprintf("%s/api/DocumentInfo/%s/%s", c.baseURL, nit, cufe)

	// 3. Crear request GET con context
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// 4. Headers: Authorization Bearer + Accept-Encoding gzip
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	c.log.Debug("Requesting document info", "nit", nit, "cufe", cufe, "url", url)

	// 5. Ejecutar request con httpClient (incluye circuit breaker)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.log.Error("Request failed", "error", err, "url", url)
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	// 6. Manejar gzip compression (patrón existente)
	var reader io.Reader = resp.Body
	if strings.Contains(resp.Header.Get("Content-Encoding"), "gzip") {
		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			c.log.Error("Failed to create gzip reader", "error", err)
			return nil, fmt.Errorf("create gzip reader: %w", err)
		}
		defer gzipReader.Close()
		reader = gzipReader
		c.log.Debug("Response is gzip compressed, decompressing")
	}

	// 7. Leer response body
	body, err := io.ReadAll(reader)
	if err != nil {
		c.log.Error("Failed to read response body", "error", err, "status", resp.StatusCode)
		return nil, fmt.Errorf("read response body: %w", err)
	}

	c.log.Debug("Numrot DocumentInfo API response", "status", resp.StatusCode, "body_length", len(body))

	// 8. Manejar códigos de estado HTTP
	if resp.StatusCode == http.StatusUnauthorized {
		c.auth.ClearToken()
		c.log.Warn("Token expired or invalid, clearing cache", "status", resp.StatusCode)
		return nil, fmt.Errorf("authentication failed: token expired")
	}

	if resp.StatusCode == http.StatusNotFound {
		c.log.Debug("Document not found", "cufe", cufe)
		return nil, fmt.Errorf("document not found: %s", cufe)
	}

	if resp.StatusCode != http.StatusOK {
		c.log.Error("Non-OK status", "status", resp.StatusCode, "body", string(body))
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	// 9. Parsear JSON response
	var numrotResp numrotDocumentInfoResponse
	if err := json.Unmarshal(body, &numrotResp); err != nil {
		c.log.Error("Unmarshal failed", "error", err, "body", string(body))
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	c.log.Debug("Numrot DocumentInfo API response parsed",
		"statusCode", numrotResp.StatusCode,
		"statusDescription", numrotResp.StatusDescription,
		"document_count", len(numrotResp.DocumentInfo))

	// 10. Validar StatusCode de Numrot
	if numrotResp.StatusCode != "200" {
		c.log.Warn("Numrot DocumentInfo API returned error",
			"statusCode", numrotResp.StatusCode,
			"statusDescription", numrotResp.StatusDescription)
		return nil, fmt.Errorf("numrot error: %s (code: %s)",
			numrotResp.StatusDescription, numrotResp.StatusCode)
	}

	// 11. Validar al menos un documento
	if len(numrotResp.DocumentInfo) == 0 {
		c.log.Debug("No document info found for CUFE", "cufe", cufe)
		return nil, fmt.Errorf("no document found for CUFE: %s", cufe)
	}

	return &numrotResp, nil
}

// numrotSearchEstadosDIANResponse represents the response from Numrot SearchEstadosDIAN API
// This endpoint returns a flat structure with document validation data from DIAN
type numrotSearchEstadosDIANResponse struct {
	Uuid              string   `json:"Uuid"`              // CUFE/CUDE del documento
	QrText            string   `json:"QrText"`            // Texto del código QR con información del documento
	TrackId           string   `json:"TrackId"`           // ID de seguimiento (generalmente igual al UUID)
	Warnings          []string `json:"Warnings"`          // Advertencias de validación
	StatusCode        string   `json:"StatusCode"`        // Código de estado (ej: "200")
	ErrorReason       []string `json:"ErrorReason"`       // Razones de error (vacío si exitoso)
	ErrorMessage      string   `json:"ErrorMessage"`      // Mensaje de error o éxito
	StatusMessage     string   `json:"StatusMessage"`     // Mensaje de estado detallado
	StatusDescription string   `json:"StatusDescription"` // Descripción del estado
	Document          string   `json:"Document"`          // Base64-encoded PDF document (when includePdf=true)
	// Campos opcionales que pueden venir en la respuesta
	DocumentInfo []NumrotSearchEstadosDIANData `json:"DocumentInfo,omitempty"` // Puede venir vacío o con info estructurada
}

// NumrotSearchEstadosDIANData represents a single document in SearchEstadosDIAN response
type NumrotSearchEstadosDIANData struct {
	DocumentTypeId   string                `json:"DocumentTypeId"`
	DocumentTypeName string                `json:"DocumentTypeName"`
	Emisor           numrotEmisor          `json:"Emisor"`
	Estado           map[string]string     `json:"Estado"`
	NumeroDocumento  numrotNumeroDocumento `json:"NumeroDocumento"`
	Receptor         numrotReceptor        `json:"Receptor"`
	TotalEImpuestos  numrotTotalEImpuestos `json:"TotalEImpuestos"`
	UUID             string                `json:"UUID"`
	QR               string                `json:"QR,omitempty"`
	SignatureValue   string                `json:"SignatureValue,omitempty"`
	Resolucion       string                `json:"Resolucion,omitempty"`
	HoraDocumento    string                `json:"HoraDocumento,omitempty"`
	// Eventos y estados
	Eventos          []interface{} `json:"Eventos"`
	UltimoEstado     interface{}   `json:"UltimoEstado,omitempty"`
	HistoricoEstados []interface{} `json:"HistoricoEstados,omitempty"`
	// Campos opcionales
	DocumentTags    []interface{} `json:"DocumentTags"`
	Referencias     []interface{} `json:"Referencias"`
	ValidacionesDoc []interface{} `json:"ValidacionesDoc"`
	LegitimoTenedor struct {
		Nombre string `json:"Nombre"`
	} `json:"LegitimoTenedor"`
}

// SearchEstadosDIAN obtiene información de un documento con estados desde Numrot SearchEstadosDIAN API.
// Este método consulta el endpoint /api/searchestadosdian/{nit}/{documento} que retorna información
// detallada del documento incluyendo estados y eventos.
// Host: https://numrotapiprueba.net (configured via NUMROT_BASE_URL)
func (c *Client) SearchEstadosDIAN(ctx context.Context, nit, documento string) (*numrotSearchEstadosDIANResponse, error) {
	// 1. Obtener token de autenticación (reutiliza AuthManager existente)
	token, err := c.auth.GetToken(ctx)
	if err != nil {
		c.log.Error("Failed to get Numrot auth token", "error", err)
		return nil, fmt.Errorf("get authentication token: %w", err)
	}

	url := fmt.Sprintf("%s/api/searchestadosdian/%s/%s?includeXml=true&includePdf=true", c.baseURL, nit, documento)

	// 3. Crear request GET con context
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// 4. Headers: Authorization Bearer + Accept-Encoding gzip
	req.Header.Set("Accept-Encoding", "gzip")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	c.log.Debug("Requesting document estados from SearchEstadosDIAN", "nit", nit, "documento", documento, "url", url)

	// 5. Ejecutar request con httpClient (incluye circuit breaker)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.log.Error("Request failed", "error", err, "url", url)
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	// 6. Manejar gzip compression (patrón existente)
	var reader io.Reader = resp.Body
	if strings.Contains(resp.Header.Get("Content-Encoding"), "gzip") {
		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			c.log.Error("Failed to create gzip reader", "error", err)
			return nil, fmt.Errorf("create gzip reader: %w", err)
		}
		defer gzipReader.Close()
		reader = gzipReader
		c.log.Debug("Response is gzip compressed, decompressing")
	}

	// 7. Leer response body
	body, err := io.ReadAll(reader)
	if err != nil {
		c.log.Error("Failed to read response body", "error", err, "status", resp.StatusCode)
		return nil, fmt.Errorf("read response body: %w", err)
	}

	c.log.Debug("Numrot SearchEstadosDIAN API response", "status", resp.StatusCode, "body_length", len(body))

	// 8. Manejar códigos de estado HTTP
	if resp.StatusCode == http.StatusUnauthorized {
		c.auth.ClearToken()
		c.log.Warn("Token expired or invalid, clearing cache", "status", resp.StatusCode)
		return nil, fmt.Errorf("authentication failed: token expired")
	}

	if resp.StatusCode == http.StatusNotFound {
		c.log.Debug("Document not found", "documento", documento)
		return nil, fmt.Errorf("document not found: %s", documento)
	}

	if resp.StatusCode != http.StatusOK {
		c.log.Error("Non-OK status", "status", resp.StatusCode, "body", string(body))
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	// 9. Parsear JSON response
	var numrotResp numrotSearchEstadosDIANResponse
	if err := json.Unmarshal(body, &numrotResp); err != nil {
		c.log.Error("Unmarshal failed", "error", err, "body", string(body))
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	c.log.Debug("Numrot SearchEstadosDIAN API response parsed",
		"statusCode", numrotResp.StatusCode,
		"statusDescription", numrotResp.StatusDescription,
		"uuid", numrotResp.Uuid,
		"document_count", len(numrotResp.DocumentInfo))

	// 10. Validar StatusCode de Numrot
	if numrotResp.StatusCode != "200" {
		c.log.Warn("Numrot SearchEstadosDIAN API returned error",
			"statusCode", numrotResp.StatusCode,
			"statusDescription", numrotResp.StatusDescription,
			"errorMessage", numrotResp.ErrorMessage,
			"errorReason", numrotResp.ErrorReason)
		return nil, fmt.Errorf("numrot error: %s (code: %s)",
			numrotResp.StatusDescription, numrotResp.StatusCode)
	}

	// 11. Validar que tengamos información del documento (Uuid o DocumentInfo)
	// La respuesta puede venir en dos formatos:
	// 1. Formato plano con Uuid, QrText, etc. (respuesta de validación DIAN)
	// 2. Formato estructurado con DocumentInfo (respuesta antigua)
	if numrotResp.Uuid == "" && len(numrotResp.DocumentInfo) == 0 {
		c.log.Debug("No document info found in response", "documento", documento)
		return nil, fmt.Errorf("no document found: %s", documento)
	}

	return &numrotResp, nil
}

// parseQrTextData extrae información del QrText de la respuesta de DIAN
// Formato: "NumFac: SETT56046 FecFac: 2025-12-23 HorFac: 14:37:00-05:00 NitFac: 860011153 DocAdq: 38858 ValFac: 176471.00 ValIva: 33529.00..."
func parseQrTextData(qrText string) (prefijo, consecutivo, fecha, hora, emisorNit, receptorNit string, valorTotal, valorIva float64) {
	// Parsear el QrText usando strings.Split
	parts := strings.Fields(qrText)

	for i := 0; i < len(parts)-1; i++ {
		key := strings.TrimSuffix(parts[i], ":")
		value := parts[i+1]

		switch key {
		case "NumFac":
			// NumFac puede ser "SETT56046" o "56046" (con o sin prefijo)
			// Intentar separar prefijo y consecutivo
			numFac := value
			// Si contiene letras al inicio, esas son el prefijo
			var prefijoLocal string
			var consecutivoLocal string
			for j, c := range numFac {
				if c >= '0' && c <= '9' {
					prefijoLocal = numFac[:j]
					consecutivoLocal = numFac[j:]
					break
				}
			}
			if prefijoLocal != "" {
				prefijo = prefijoLocal
				consecutivo = consecutivoLocal
			} else {
				consecutivo = numFac
			}
		case "FecFac":
			fecha = value
		case "HorFac":
			hora = value
		case "NitFac":
			emisorNit = value
		case "DocAdq":
			receptorNit = value
		case "ValTolFac":
			// Convertir string a float64
			if v, err := strconv.ParseFloat(value, 64); err == nil {
				valorTotal = v
			}
		case "ValIva":
			if v, err := strconv.ParseFloat(value, 64); err == nil {
				valorIva = v
			}
		}
	}

	return
}

// GetDocumentByProviderParams obtiene información de un documento usando parámetros de proveedor.
// Usa el endpoint SearchEstadosDIAN de Numrot que retorna información completa incluyendo estados.
func (c *Client) GetDocumentByProviderParams(ctx context.Context, proveedor, consecutivo, prefijo, ofe, tipo string) (*numrotDocumentInfoResponse, error) {
	// 1. Construir número de documento: prefijo + consecutivo
	documentNumber := prefijo + consecutivo

	// 2. Usar el NIT del OFE (ofe) para consultar SearchEstadosDIAN
	// Nota: El emisorNit debería ser el OFE
	emisorNit := ofe
	if emisorNit == "" {
		emisorNit = proveedor
	}

	// 3. Llamar a SearchEstadosDIAN para obtener información completa con estados
	searchResp, err := c.SearchEstadosDIAN(ctx, emisorNit, documentNumber)
	if err != nil {
		c.log.Error("Failed to search estados DIAN", "error", err, "documentNumber", documentNumber, "ofe", ofe)
		return nil, fmt.Errorf("search estados DIAN: %w", err)
	}

	// 4. Manejar dos formatos de respuesta:
	// Formato 1: DocumentInfo viene poblado (respuesta estructurada)
	// Formato 2: DocumentInfo vacío pero Uuid tiene valor (respuesta de validación DIAN)

	var docInfoData NumrotDocumentInfoData

	if len(searchResp.DocumentInfo) > 0 {
		// Formato 1: Usar DocumentInfo directamente
		docInfoData = NumrotDocumentInfoData{
			DocumentTypeId:   searchResp.DocumentInfo[0].DocumentTypeId,
			DocumentTypeName: searchResp.DocumentInfo[0].DocumentTypeName,
			Emisor:           searchResp.DocumentInfo[0].Emisor,
			Estado:           searchResp.DocumentInfo[0].Estado,
			NumeroDocumento:  searchResp.DocumentInfo[0].NumeroDocumento,
			Receptor:         searchResp.DocumentInfo[0].Receptor,
			TotalEImpuestos:  searchResp.DocumentInfo[0].TotalEImpuestos,
			UUID:             searchResp.DocumentInfo[0].UUID,
			QR:               searchResp.DocumentInfo[0].QR,
			SignatureValue:   searchResp.DocumentInfo[0].SignatureValue,
			Resolucion:       searchResp.DocumentInfo[0].Resolucion,
			HoraDocumento:    searchResp.DocumentInfo[0].HoraDocumento,
			Eventos:          searchResp.DocumentInfo[0].Eventos,
			DocumentTags:     searchResp.DocumentInfo[0].DocumentTags,
			Referencias:      searchResp.DocumentInfo[0].Referencias,
			ValidacionesDoc:  searchResp.DocumentInfo[0].ValidacionesDoc,
			LegitimoTenedor:  searchResp.DocumentInfo[0].LegitimoTenedor,
		}
	} else if searchResp.Uuid != "" {
		// Formato 2: Parsear QrText y construir DocumentInfo
		c.log.Debug("Using flat response format, parsing QrText", "uuid", searchResp.Uuid)

		// Parsear QrText para extraer datos del documento
		qrPrefijo, qrConsecutivo, fecha, hora, qrEmisorNit, qrReceptorNit, valorTotal, valorIva := parseQrTextData(searchResp.QrText)

		// Usar valores parseados o los parámetros originales como fallback
		if qrPrefijo == "" {
			qrPrefijo = prefijo
		}
		if qrConsecutivo == "" {
			qrConsecutivo = consecutivo
		}
		if qrEmisorNit == "" {
			qrEmisorNit = ofe
		}
		if qrReceptorNit == "" {
			qrReceptorNit = proveedor
		}

		// Construir NumrotDocumentInfoData a partir de los datos planos
		docInfoData = NumrotDocumentInfoData{
			DocumentTypeId:   tipo,
			DocumentTypeName: "", // No disponible en respuesta plana
			Emisor: numrotEmisor{
				Nombre:    "",
				NumeroDoc: qrEmisorNit,
			},
			Estado: map[string]string{
				"Estado":    searchResp.StatusMessage,
				"Resultado": searchResp.StatusCode,
			},
			NumeroDocumento: numrotNumeroDocumento{
				FechaEmision: fecha,
				Folio:        qrConsecutivo,
				Serie:        qrPrefijo,
			},
			Receptor: numrotReceptor{
				Nombre:    "",
				NumeroDoc: qrReceptorNit,
				TipoDoc:   "",
			},
			TotalEImpuestos: numrotTotalEImpuestos{
				Iva:   valorIva,
				Total: valorTotal,
			},
			UUID:          searchResp.Uuid,
			QR:            searchResp.QrText,
			HoraDocumento: hora,
			Eventos:       []interface{}{},
			DocumentTags:  []interface{}{},
			Referencias:   []interface{}{},
		}
	} else {
		c.log.Debug("No document found in any format", "documentNumber", documentNumber, "ofe", ofe)
		return nil, fmt.Errorf("document not found: %s", documentNumber)
	}

	// 5. Construir respuesta en formato DocumentInfo
	docInfo := &numrotDocumentInfoResponse{
		StatusCode:        searchResp.StatusCode,
		StatusDescription: searchResp.StatusDescription,
		DocumentInfo:      []NumrotDocumentInfoData{docInfoData},
	}

	return docInfo, nil
}

// transformReceivedDocumentToDocument converts a Numrot received document to a domain Document.
func (c *Client) transformReceivedDocumentToDocument(nd numrotReceivedDocument) (invoice.Document, error) {
	// Validate required fields before parsing
	if nd.OFE == "" {
		return invoice.Document{}, fmt.Errorf("missing required field: ofe")
	}
	if nd.Proveedor == "" {
		return invoice.Document{}, fmt.Errorf("missing required field: proveedor")
	}
	if nd.Tipo == "" {
		return invoice.Document{}, fmt.Errorf("missing required field: tipo")
	}
	if nd.Consecutivo == "" {
		return invoice.Document{}, fmt.Errorf("missing required field: consecutivo")
	}
	if nd.CUFE == "" {
		return invoice.Document{}, fmt.Errorf("missing required field: cufe")
	}
	if nd.Fecha == "" {
		return invoice.Document{}, fmt.Errorf("missing required field: fecha")
	}
	if nd.Hora == "" {
		return invoice.Document{}, fmt.Errorf("missing required field: hora")
	}
	if nd.Valor == "" {
		return invoice.Document{}, fmt.Errorf("missing required field: valor")
	}

	// Parse date with detailed error message
	// Validate required fields before parsing
	if nd.OFE == "" {
		return invoice.Document{}, fmt.Errorf("missing required field: ofe")
	}
	if nd.Proveedor == "" {
		return invoice.Document{}, fmt.Errorf("missing required field: proveedor")
	}
	if nd.Tipo == "" {
		return invoice.Document{}, fmt.Errorf("missing required field: tipo")
	}
	if nd.Consecutivo == "" {
		return invoice.Document{}, fmt.Errorf("missing required field: consecutivo")
	}
	if nd.CUFE == "" {
		return invoice.Document{}, fmt.Errorf("missing required field: cufe")
	}
	if nd.Fecha == "" {
		return invoice.Document{}, fmt.Errorf("missing required field: fecha")
	}
	if nd.Hora == "" {
		return invoice.Document{}, fmt.Errorf("missing required field: hora")
	}
	if nd.Valor == "" {
		return invoice.Document{}, fmt.Errorf("missing required field: valor")
	}
	if nd.UrlPDF == "" {
		return invoice.Document{}, fmt.Errorf("missing required field: urlPDF")
	}
	if nd.UrlXML == "" {
		return invoice.Document{}, fmt.Errorf("missing required field: urlXML")
	}

	// Parse date with detailed error message
	dateLayout := "2006-01-02"
	fecha, err := time.Parse(dateLayout, nd.Fecha)
	if err != nil {
		return invoice.Document{}, fmt.Errorf("parse fecha [%s]: expected format YYYY-MM-DD: %w", nd.Fecha, err)
	}

	// Parse valor as float64 with detailed error message
	valor, err := strconv.ParseFloat(nd.Valor, 64)
	if err != nil {
		return invoice.Document{}, fmt.Errorf("parse valor [%s]: invalid numeric format: %w", nd.Valor, err)
	}

	// Handle empty prefijo gracefully (it's allowed to be empty)
	prefijo := nd.Prefijo
	if prefijo == "" {
		// Empty prefijo is valid, keep it as empty string
	}

	return invoice.Document{
		OFE:         nd.OFE,
		Proveedor:   nd.Proveedor,
		Tipo:        nd.Tipo,
		Prefijo:     prefijo,
		Consecutivo: nd.Consecutivo,
		CUFE:        nd.CUFE,
		Fecha:       fecha,
		Hora:        nd.Hora,
		Valor:       valor,
		Marca:       false, // DocumentsReceived response doesn't include marca field
		UrlPDF:      nd.UrlPDF,
		UrlXML:      nd.UrlXML,
	}, nil
}

// transformToDocument converts a Numrot document to a domain Document.
func (c *Client) transformToDocument(nd numrotDocument) (invoice.Document, error) {
	// Parse date
	dateLayout := "2006-01-02"
	fecha, err := time.Parse(dateLayout, nd.FechaEmision)
	if err != nil {
		return invoice.Document{}, fmt.Errorf("parse fecha emision: %w", err)
	}

	// Parse NumeroFactura to extract prefix, consecutive, and CUFE
	// NumeroFactura format may vary, so we'll try to extract what we can
	// For now, we'll use the full number as consecutive and set defaults for prefix/CUFE
	prefijo := ""
	consecutivo := nd.NumeroFactura

	// Extract CUFE: CUFE is required - DocumentInfo endpoint only accepts CUFE, not document numbers
	// If CUFE is not present, return an error instead of using fallbacks
	cufe := nd.CUFE
	if cufe == "" {
		return invoice.Document{}, fmt.Errorf("missing required field: CUFE (document number: %s)", nd.NumeroFactura)
	}

	// Determine marca (mark) - for now, set to false
	// This could be determined from document status or other fields
	marca := false

	return invoice.Document{
		OFE:         nd.EmisorNit,
		Proveedor:   nd.EmisorNombre,
		Tipo:        nd.TipoFactura,
		Prefijo:     prefijo,
		Consecutivo: consecutivo,
		CUFE:        cufe,
		Fecha:       fecha,
		Hora:        nd.HoraEmision,
		Valor:       nd.TotalFactura,
		Marca:       marca,
		UrlPDF:      nd.UrlPDF,
		UrlXML:      nd.UrlXML,
	}, nil
}

// numrotSetEventRequest represents the request structure for Numrot SetEvent API.
type numrotSetEventRequest struct {
	Key                     string   `json:"Key"`
	Secret                  string   `json:"Secret"`
	EmisorNit               string   `json:"EmisorNit"`
	RazonSocial             string   `json:"RazonSocial"`
	DocumentoNumeroCompleto string   `json:"DocumentoNumeroCompleto"`
	CodigoRadian            []string `json:"CodigoRadian"`
	FechaGeneracionEvento   string   `json:"FechaGeneracionEvento"`
	NombreGenerador         string   `json:"NombreGenerador"`
	ApellidoGenerador       string   `json:"ApellidoGenerador"`
	IdentificacionGenerador string   `json:"IdentificacionGenerador"`
	CodigoRechazo           string   `json:"CodigoRechazo,omitempty"`
}

// numrotSetEventResponse represents the response structure from Numrot SetEvent API.
type numrotSetEventResponse struct {
	Codigo          string              `json:"Codigo"`
	NumeroDocumento string              `json:"NumeroDocumento"`
	Resultado       []numrotEventResult `json:"Resultado"`
	MensajeError    string              `json:"MensajeError"`
}

// numrotEventResult represents a single event result in the response.
type numrotEventResult struct {
	TipoEvento      string `json:"TipoEvento"`
	Mensaje         string `json:"Mensaje"`
	MensajeError    string `json:"MensajeError"`
	CodigoRespuesta string `json:"CodigoRespuesta"`
}

// numrotSetEventErrorResponse represents error response from Numrot SetEvent API.
type numrotSetEventErrorResponse struct {
	Code  int    `json:"code"`
	Error string `json:"error"`
}

// RegisterEvent registers a Radian event for a document.
func (c *Client) RegisterEvent(ctx context.Context, evt event.Event, emisorNit, razonSocial string) (*invoice.EventRegistrationResult, error) {
	if c.key == "" || c.secret == "" {
		return nil, fmt.Errorf("key and secret are required for event registration")
	}

	// Translate event type to Radian code
	radianCode, err := event.ToRadianCode(evt.EventType)
	if err != nil {
		return nil, fmt.Errorf("translate event type: %w", err)
	}

	// Format event generation date
	dateFormat := "2006-01-02 15:04:05"
	fechaGeneracionEvento := evt.EventGenerationDate.Format(dateFormat)

	// Build request
	reqBody := numrotSetEventRequest{
		Key:                     c.key,
		Secret:                  c.secret,
		EmisorNit:               emisorNit,
		RazonSocial:             razonSocial,
		DocumentoNumeroCompleto: evt.DocumentNumber,
		CodigoRadian:            []string{string(radianCode)},
		FechaGeneracionEvento:   fechaGeneracionEvento,
		NombreGenerador:         evt.NombreGenerador,
		ApellidoGenerador:       evt.ApellidoGenerador,
		IdentificacionGenerador: evt.IdentificacionGenerador,
	}

	// Add rejection code only for RECLAMO events
	if evt.RejectionCode != nil {
		reqBody.CodigoRechazo = string(*evt.RejectionCode)
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/Radian/SetEvent", c.radianURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	c.log.Debug("Registering event with Numrot Radian", "eventType", evt.EventType, "documentNumber", evt.DocumentNumber, "url", url)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.log.Error("Failed to execute request to Numrot Radian SetEvent", "error", err, "url", url)
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.log.Error("Failed to read response body from Numrot Radian SetEvent", "error", err, "status", resp.StatusCode)
		return nil, fmt.Errorf("read response body: %w", err)
	}

	c.log.Debug("Numrot Radian SetEvent API response", "status", resp.StatusCode, "body_length", len(body))

	// Handle 400 (document not found) error response
	if resp.StatusCode == http.StatusBadRequest {
		var errorResp numrotSetEventErrorResponse
		if err := json.Unmarshal(body, &errorResp); err == nil {
			c.log.Warn("Numrot Radian SetEvent returned document not found", "code", errorResp.Code, "error", errorResp.Error)
			return nil, fmt.Errorf("document not found: %s", errorResp.Error)
		}
		// If unmarshal fails, return generic error
		return nil, fmt.Errorf("document not found: %s", string(body))
	}

	// Handle other non-OK status codes
	if resp.StatusCode != http.StatusOK {
		c.log.Error("Numrot Radian SetEvent API returned non-OK status", "status", resp.StatusCode, "body", string(body))
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var numrotResp numrotSetEventResponse
	if err := json.Unmarshal(body, &numrotResp); err != nil {
		c.log.Error("Failed to unmarshal Numrot Radian SetEvent response", "error", err, "body", string(body))
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	c.log.Debug("Numrot Radian SetEvent API response parsed", "codigo", numrotResp.Codigo, "numeroDocumento", numrotResp.NumeroDocumento, "resultado_count", len(numrotResp.Resultado))

	// Transform Numrot response to domain response
	resultado := make([]invoice.EventResult, 0, len(numrotResp.Resultado))
	for _, nr := range numrotResp.Resultado {
		resultado = append(resultado, invoice.EventResult{
			TipoEvento:      nr.TipoEvento,
			Mensaje:         nr.Mensaje,
			MensajeError:    nr.MensajeError,
			CodigoRespuesta: nr.CodigoRespuesta,
		})
	}

	return &invoice.EventRegistrationResult{
		Code:            numrotResp.Codigo,
		NumeroDocumento: numrotResp.NumeroDocumento,
		Resultado:       resultado,
		MensajeError:    numrotResp.MensajeError,
	}, nil
}

// numrotRegisterDocumentRequest represents the request structure for Numrot SendDIAN API.
type numrotRegisterDocumentRequest struct {
	Invoice []numrotInvoice `json:"Invoice"`
}

// numrotInvoice represents a single invoice in Numrot format.
type numrotInvoice struct {
	InvoiceControl              numrotInvoiceControl                `json:"InvoiceControl"`
	CustomizationID             string                              `json:"CustomizationID"`
	ProfileExecutionID          string                              `json:"ProfileExecutionID"`
	ID                          string                              `json:"ID"`
	IssueDate                   string                              `json:"IssueDate"`
	IssueTime                   string                              `json:"IssueTime"`
	DueDate                     *string                             `json:"DueDate,omitempty"`
	InvoiceTypeCode             string                              `json:"InvoiceTypeCode"`
	Note                        []string                            `json:"Note,omitempty"`
	DocumentCurrencyCode        string                              `json:"DocumentCurrencyCode"`
	LineCountNumeric            string                              `json:"LineCountNumeric"`
	OrderReference              *numrotOrderReference               `json:"OrderReference,omitempty"`
	DiscrepancyResponse         []numrotDiscrepancyResponse         `json:"DiscrepancyResponse,omitempty"`
	InvoiceDocumentReference    *numrotInvoiceDocumentReference     `json:"InvoiceDocumentReference,omitempty"`
	AdditionalDocumentReference []numrotAdditionalDocumentReference `json:"AdditionalDocumentReference,omitempty"`
	AccountingSupplierParty     numrotAccountingParty               `json:"AccountingSupplierParty"`
	AccountingCustomerParty     numrotAccountingParty               `json:"AccountingCustomerParty"`
	Delivery                    *numrotDelivery                     `json:"Delivery,omitempty"`
	PaymentMeans                []numrotPaymentMeans                `json:"PaymentMeans,omitempty"`
	PrePaidPayment              []numrotPrePaidPayment              `json:"PrePaidPayment,omitempty"`
	PaymentExchangeRate         *numrotPaymentExchangeRate          `json:"PaymentExchangeRate,omitempty"`
	InvoicePeriod               *numrotDocumentInvoicePeriod        `json:"InvoicePeriod,omitempty"`
	TaxTotal                    []numrotTaxTotal                    `json:"TaxTotal,omitempty"`
	LegalMonetaryTotal          numrotLegalMonetaryTotal            `json:"LegalMonetaryTotal"`
	InvoiceLine                 []numrotInvoiceLine                 `json:"InvoiceLine"`
}

// numrotInvoiceControl represents invoice control/resolution information.
type numrotInvoiceControl struct {
	InvoiceAuthorization string `json:"InvoiceAuthorization"`
	StartDate            string `json:"StartDate"`
	EndDate              string `json:"EndDate"`
	Prefix               string `json:"Prefix"`
	From                 string `json:"From"`
	To                   string `json:"To"`
}

// numrotOrderReference represents order reference information.
type numrotOrderReference struct {
	ID string `json:"ID"`
}

// numrotDiscrepancyResponse represents discrepancy response for NC/ND.
type numrotDiscrepancyResponse struct {
	ReferenceID  string   `json:"ReferenceID"`
	ResponseCode string   `json:"ResponseCode"`
	Description  []string `json:"Description"`
}

// numrotInvoiceDocumentReference represents reference to original invoice for NC/ND.
type numrotInvoiceDocumentReference struct {
	ID string `json:"ID"`
}

// numrotAdditionalDocumentReference represents additional document references.
type numrotAdditionalDocumentReference struct {
	ID               string  `json:"ID"`
	IssueDate        *string `json:"IssueDate,omitempty"`
	DocumentTypeCode string  `json:"DocumentTypeCode"`
	DocumentType     *string `json:"DocumentType,omitempty"`
}

// numrotAccountingParty represents supplier or customer party information.
type numrotAccountingParty struct {
	AdditionalAccountID        string                  `json:"AdditionalAccountID"`
	IndustryClassificationCode *string                 `json:"IndustryClassificationCode,omitempty"`
	ID                         *string                 `json:"ID,omitempty"`
	Name                       string                  `json:"Name"`
	SchemeName                 *string                 `json:"schemeName,omitempty"`
	PhysicalLocation           *numrotPhysicalLocation `json:"PhysicalLocation,omitempty"`
	PartyTaxScheme             numrotPartyTaxScheme    `json:"PartyTaxScheme"`
	PartyLegalEntity           interface{}             `json:"PartyLegalEntity"`
	Contact                    *numrotContact          `json:"Contact,omitempty"`
}

// numrotPhysicalLocation represents physical location information.
type numrotPhysicalLocation struct {
	ID                   string `json:"ID"`
	CityName             string `json:"CityName"`
	PostalZone           string `json:"PostalZone"`
	CountrySubentity     string `json:"CountrySubentity"`
	CountrySubentityCode string `json:"CountrySubentityCode"`
	Line                 string `json:"Line"`
	IdentificationCode   string `json:"IdentificationCode"`
	Name                 string `json:"Name"`
}

// numrotPartyTaxScheme represents party tax scheme information.
type numrotPartyTaxScheme struct {
	RegistrationName    string                  `json:"RegistrationName"`
	CompanyID           string                  `json:"CompanyID"`
	SchemeID            *string                 `json:"schemeID,omitempty"`
	SchemeName          *string                 `json:"schemeName,omitempty"`
	TaxLevelCode        string                  `json:"TaxLevelCode"`
	RegistrationAddress *numrotPhysicalLocation `json:"RegistrationAddress,omitempty"`
	TaxScheme           numrotTaxScheme         `json:"TaxScheme"`
}

// numrotTaxScheme represents tax scheme information.
type numrotTaxScheme struct {
	ID   string `json:"ID"`
	Name string `json:"Name"`
}

// numrotPartyLegalEntity represents party legal entity information.
type numrotPartyLegalEntity struct {
	RegistrationName string  `json:"RegistrationName"`
	CompanyID        string  `json:"CompanyID"`
	SchemeID         *string `json:"schemeID,omitempty"`
	SchemeName       *string `json:"schemeName,omitempty"`
	ID               *string `json:"ID,omitempty"`
}

// numrotContact represents contact information.
type numrotContact struct {
	Name           string  `json:"Name"`
	Telephone      string  `json:"Telephone"`
	Telefax        string  `json:"Telefax"`
	ElectronicMail string  `json:"ElectronicMail"`
	Note           *string `json:"Note,omitempty"`
}

// numrotDelivery represents delivery information.
type numrotDelivery struct {
	ActualDeliveryDate string                 `json:"ActualDeliveryDate"`
	DeliveryAddress    numrotPhysicalLocation `json:"DeliveryAddress"`
}

// numrotPaymentMeans represents payment means information.
type numrotPaymentMeans struct {
	ID               string   `json:"ID"`
	PaymentMeansCode string   `json:"PaymentMeansCode"`
	PaymentDueDate   *string  `json:"PaymentDueDate,omitempty"`
	PaymentID        []string `json:"PaymentID"`
}

// numrotPrePaidPayment represents prepaid payment information.
type numrotPrePaidPayment struct {
	ID           string `json:"ID"`
	CurrencyID   string `json:"currencyID"`
	PaidAmount   string `json:"PaidAmount"`
	ReceivedDate string `json:"ReceivedDate"`
}

// numrotPaymentExchangeRate represents payment exchange rate information.
type numrotPaymentExchangeRate struct {
	SourceCurrencyCode     string `json:"SourceCurrencyCode"`
	SourceCurrencyBaseRate string `json:"SourceCurrencyBaseRate"`
	TargetCurrencyCode     string `json:"TargetCurrencyCode"`
	TargetCurrencyBaseRate string `json:"TargetCurrencyBaseRate"`
	CalculationRate        string `json:"CalculationRate"`
	Date                   string `json:"Date"`
}

// numrotTaxTotal represents tax total information.
type numrotTaxTotal struct {
	TaxAmount      string              `json:"TaxAmount"`
	RoundingAmount *string             `json:"RoundingAmount,omitempty"`
	CurrencyID     string              `json:"currencyID"`
	TaxSubtotal    []numrotTaxSubtotal `json:"TaxSubtotal"`
}

// numrotTaxSubtotal represents tax subtotal information.
type numrotTaxSubtotal struct {
	TaxableAmount string `json:"TaxableAmount"`
	TaxAmount     string `json:"TaxAmount"`
	Percent       string `json:"Percent"`
	CurrencyID    string `json:"currencyID"`
	ID            string `json:"ID"`
	Name          string `json:"Name"`
}

// numrotLegalMonetaryTotal represents legal monetary total information.
type numrotLegalMonetaryTotal struct {
	LineExtensionAmount  string  `json:"LineExtensionAmount"`
	TaxExclusiveAmount   string  `json:"TaxExclusiveAmount"`
	TaxInclusiveAmount   string  `json:"TaxInclusiveAmount"`
	AllowanceTotalAmount string  `json:"AllowanceTotalAmount"`
	PrePaidAmount        *string `json:"PrePaidAmount,omitempty"`
	PayableAmount        string  `json:"PayableAmount"`
	CurrencyID           string  `json:"currencyID"`
}

// numrotInvoiceLine represents an invoice line item.
type numrotInvoiceLine struct {
	ID                       string               `json:"ID"`
	Note                     []string             `json:"Note,omitempty"`
	InvoicedQuantity         string               `json:"InvoicedQuantity"`
	InvoicedQuantityUnitCode string               `json:"unitCode,omitempty"`
	LineExtensionAmount      string               `json:"LineExtensionAmount"`
	CurrencyID               string               `json:"currencyID"`
	InvoicePeriod            *numrotInvoicePeriod `json:"InvoicePeriod,omitempty"`
	TaxTotal                 []numrotTaxTotal     `json:"TaxTotal,omitempty"`
	Item                     numrotItem           `json:"Item"`
	Price                    numrotPrice          `json:"Price"`
}

// numrotItem represents item information.
type numrotItem struct {
	Description                string                    `json:"Description"`
	SellersItemIdentification  *numrotItemIdentification `json:"SellersItemIdentification,omitempty"`
	StandardItemIdentification *numrotItemIdentification `json:"StandardItemIdentification,omitempty"`
}

// numrotItemIdentification represents item identification.
type numrotItemIdentification struct {
	ID         string  `json:"ID"`
	SchemeID   *string `json:"schemeID,omitempty"`
	SchemeName *string `json:"schemeName,omitempty"`
}

// numrotPrice represents price information.
type numrotPrice struct {
	PriceAmount          string `json:"PriceAmount"`
	CurrencyID           string `json:"currencyID"`
	BaseQuantity         string `json:"BaseQuantity"`
	BaseQuantityUnitCode string `json:"unitCode,omitempty"`
}

// numrotInvoicePeriod represents invoice period information for DS items.
type numrotInvoicePeriod struct {
	StartDate       string `json:"StartDate"`
	DescriptionCode string `json:"DescriptionCode"`
	Description     string `json:"Description"`
}

// numrotDocumentInvoicePeriod represents invoice period information at document level for NC without reference.
type numrotDocumentInvoicePeriod struct {
	StartDate string `json:"StartDate"`
	StartTime string `json:"StartTime"`
	EndDate   string `json:"EndDate"`
	EndTime   string `json:"EndTime"`
}

// numrotSendDIANResponse represents the actual response wrapper from Numrot SendDIAN API.
type numrotSendDIANResponse struct {
	Size    int    `json:"_size"`
	Preview string `json:"_preview"`
}

// numrotSendDIANDocumentResponse represents the document response inside _preview from SendDIAN API.
type numrotSendDIANDocumentResponse struct {
	StatusCode        string   `json:"StatusCode"`
	DocumentNumber    string   `json:"DocumentNumber"`
	TrackId           string   `json:"TrackId"`
	Uuid              string   `json:"Uuid"`
	QrText            string   `json:"QrText"`
	StatusMessage     string   `json:"StatusMessage"`
	StatusDescription string   `json:"StatusDescription"`
	Document          string   `json:"Document"`
	ErrorMessage      string   `json:"ErrorMessage"`
	ErrorReason       []string `json:"ErrorReason"`
	Warnings          []string `json:"Warnings"`
	Pdfdocument       string   `json:"Pdfdocument"`
	CdoID             int      `json:"cdo_id,omitempty"` // Document ID from Numrot
}

// numrotRegisterDocumentResponse represents the legacy response format from Numrot SendDIAN API (for backward compatibility).
type numrotRegisterDocumentResponse struct {
	Message              string                    `json:"message"`
	Lote                 string                    `json:"lote"`
	DocumentosProcesados []numrotProcessedDocument `json:"documentos_procesados"`
	DocumentosFallidos   []numrotFailedDocument    `json:"documentos_fallidos"`
}

// numrotProcessedDocument represents a successfully processed document.
type numrotProcessedDocument struct {
	CdoID              int    `json:"cdo_id"`
	RfaPrefijo         string `json:"rfa_prefijo"`
	CdoConsecutivo     string `json:"cdo_consecutivo"`
	FechaProcesamiento string `json:"fecha_procesamiento"`
	HoraProcesamiento  string `json:"hora_procesamiento"`
}

// numrotFailedDocument represents a failed document.
type numrotFailedDocument struct {
	Documento          string      `json:"documento"`
	Consecutivo        string      `json:"consecutivo"`
	Prefijo            string      `json:"prefijo"`
	Errors             interface{} `json:"errors"`
	FechaProcesamiento string      `json:"fecha_procesamiento"`
	HoraProcesamiento  string      `json:"hora_procesamiento"`
}

// RegisterDocument registers documents with Numrot SendDIAN API or documentSinc API (for DS).
func (c *Client) RegisterDocument(ctx context.Context, req invoice.DocumentRegistrationRequest) (*invoice.DocumentRegistrationResponse, error) {
	// Determine document type and get documents
	var documents []invoice.OpenETLDocument
	var documentType string

	if len(req.Documentos.FC) > 0 {
		documents = req.Documentos.FC
		documentType = "FC"
	} else if len(req.Documentos.NC) > 0 {
		documents = req.Documentos.NC
		documentType = "NC"
	} else if len(req.Documentos.ND) > 0 {
		documents = req.Documentos.ND
		documentType = "ND"
	} else if len(req.Documentos.DS) > 0 {
		documents = req.Documentos.DS
		documentType = "DS"
	} else {
		return nil, fmt.Errorf("no documents provided")
	}

	// Numrot API only accepts one document per request, so we need to process each document individually
	// If we have multiple documents, use concurrent processing (one request per document)
	if len(documents) > 1 {
		return c.registerDocumentsConcurrent(ctx, documents, documentType)
	}

	// For single document, use the original method (skipLimiters=false since we haven't acquired yet)
	return c.registerDocumentsSingleBatch(ctx, req, documents, documentType, false)
}

// registerDocumentsConcurrent processes documents concurrently (one request per document)
// Numrot API only accepts one document per request, so each document is sent individually
func (c *Client) registerDocumentsConcurrent(ctx context.Context, documents []invoice.OpenETLDocument, documentType string) (*invoice.DocumentRegistrationResponse, error) {
	startTime := time.Now()

	// Use concurrency limiter if available, otherwise use a simple semaphore
	var limiterStats LimiterStats
	if c.concurrencyLimiter != nil {
		limiterStats = c.concurrencyLimiter.Stats()
		c.log.Info("Processing documents concurrently (one per request)",
			"total_documents", len(documents),
			"concurrency_limit", limiterStats.MaxConcurrent)
	} else {
		c.log.Info("Processing documents concurrently (one per request)",
			"total_documents", len(documents))
	}

	// Create channels for document processing
	type docResult struct {
		processed *invoice.ProcessedDocument
		failed    *invoice.FailedDocument
		index     int
	}

	docResultChan := make(chan docResult, len(documents))

	// Track peak stats during processing
	var peakActiveRequests int
	var statsMutex sync.Mutex
	updatePeakStats := func() {
		if c.concurrencyLimiter != nil {
			stats := c.concurrencyLimiter.Stats()
			statsMutex.Lock()
			if stats.ActiveCount > peakActiveRequests {
				peakActiveRequests = stats.ActiveCount
			}
			statsMutex.Unlock()
		}
	}

	// Use worker pool pattern instead of goroutine-per-document
	// This provides better resource management and prevents overwhelming the system
	numWorkers := 10 // Default number of workers
	if c.concurrencyLimiter != nil {
		numWorkers = c.concurrencyLimiter.MaxConcurrent()
		if numWorkers > len(documents) {
			numWorkers = len(documents) // Don't create more workers than documents
		}
		if numWorkers == 0 {
			numWorkers = 10 // Fallback to default
		}
	}

	// Create work channel
	workChan := make(chan struct {
		index    int
		document invoice.OpenETLDocument
	}, len(documents))

	// Send all documents to work channel
	for i, doc := range documents {
		workChan <- struct {
			index    int
			document invoice.OpenETLDocument
		}{index: i, document: doc}
	}
	close(workChan)

	// Start worker pool
	var wg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			// Add panic recovery to ensure workers don't crash silently
			defer func() {
				if r := recover(); r != nil {
					c.log.Error("Worker panic recovered",
						"worker_id", workerID,
						"panic", r)
					// Panic recovery: try to send failure for any documents being processed
					// This is a safety net, but we can't know which document was being processed
				}
			}()

			for work := range workChan {
				docIndex := work.index
				document := work.document

				// Process each document with proper cleanup using anonymous function
				// This ensures defer is executed per-document, not per-worker
				func() {
					// Acquire rate limiter token FIRST (before concurrency limiter)
					// This prevents too many goroutines from waiting on rate limit
					if c.rateLimiter != nil {
						if err := c.rateLimiter.Acquire(ctx); err != nil {
							now := time.Now()
							fechaProcesamiento := now.Format("2006-01-02")
							horaProcesamiento := now.Format("15:04:05")
							docResultChan <- docResult{
								failed: &invoice.FailedDocument{
									Documento:          documentType,
									Consecutivo:        document.CdoConsecutivo,
									Prefijo:            document.RfaPrefijo,
									Errors:             []string{fmt.Sprintf("Failed to acquire rate limit token: %v", err)},
									FechaProcesamiento: fechaProcesamiento,
									HoraProcesamiento:  horaProcesamiento,
								},
								index: docIndex,
							}
							return // Exit anonymous function, continue to next work item
						}
					}

					// Acquire concurrency limiter slot AFTER rate limiter
					if c.concurrencyLimiter != nil {
						if err := c.concurrencyLimiter.Acquire(ctx); err != nil {
							// Context cancelled or error acquiring slot
							now := time.Now()
							fechaProcesamiento := now.Format("2006-01-02")
							horaProcesamiento := now.Format("15:04:05")
							docResultChan <- docResult{
								failed: &invoice.FailedDocument{
									Documento:          documentType,
									Consecutivo:        document.CdoConsecutivo,
									Prefijo:            document.RfaPrefijo,
									Errors:             []string{fmt.Sprintf("Failed to acquire concurrency slot: %v", err)},
									FechaProcesamiento: fechaProcesamiento,
									HoraProcesamiento:  horaProcesamiento,
								},
								index: docIndex,
							}
							return // Exit anonymous function, continue to next work item
						}
						// Release limiter immediately after processing this document (not when worker exits)
						defer func() {
							c.concurrencyLimiter.Release()
							updatePeakStats() // Update peak stats after release
						}()
						updatePeakStats() // Update peak stats after acquire
					}

					// Process single document with circuit breaker protection
					singleDocReq := invoice.DocumentRegistrationRequest{
						Documentos: invoice.DocumentsByType{},
					}
					switch documentType {
					case "FC":
						singleDocReq.Documentos.FC = []invoice.OpenETLDocument{document}
					case "NC":
						singleDocReq.Documentos.NC = []invoice.OpenETLDocument{document}
					case "ND":
						singleDocReq.Documentos.ND = []invoice.OpenETLDocument{document}
					case "DS":
						singleDocReq.Documentos.DS = []invoice.OpenETLDocument{document}
					}

					// Execute with circuit breaker protection
					var response *invoice.DocumentRegistrationResponse
					var err error
					if c.circuitBreaker != nil {
						err = c.circuitBreaker.Execute(ctx, func() error {
							// Pass skipLimiters=true since we already acquired limiters above
							resp, execErr := c.registerDocumentsSingleBatch(ctx, singleDocReq, []invoice.OpenETLDocument{document}, documentType, true)
							if execErr != nil {
								return execErr
							}
							response = resp
							return nil
						})

						// Check if circuit breaker is open
						if err != nil {
							// Check if error is circuit breaker open
							if err == ErrCircuitBreakerOpen || err.Error() == ErrCircuitBreakerOpen.Error() {
								now := time.Now()
								fechaProcesamiento := now.Format("2006-01-02")
								horaProcesamiento := now.Format("15:04:05")
								docResultChan <- docResult{
									failed: &invoice.FailedDocument{
										Documento:          documentType,
										Consecutivo:        document.CdoConsecutivo,
										Prefijo:            document.RfaPrefijo,
										Errors:             []string{"Circuit breaker is open - too many failures detected"},
										FechaProcesamiento: fechaProcesamiento,
										HoraProcesamiento:  horaProcesamiento,
									},
									index: docIndex,
								}
								return // Exit anonymous function, continue to next work item
							}
							// If it's not a circuit breaker error, continue with normal error handling below
						}
					} else {
						// Pass skipLimiters=true since we already acquired limiters above
						response, err = c.registerDocumentsSingleBatch(ctx, singleDocReq, []invoice.OpenETLDocument{document}, documentType, true)
					}

					now := time.Now()
					fechaProcesamiento := now.Format("2006-01-02")
					horaProcesamiento := now.Format("15:04:05")

					if err != nil {
						// Error processing document
						docResultChan <- docResult{
							failed: &invoice.FailedDocument{
								Documento:          documentType,
								Consecutivo:        document.CdoConsecutivo,
								Prefijo:            document.RfaPrefijo,
								Errors:             []string{err.Error()},
								FechaProcesamiento: fechaProcesamiento,
								HoraProcesamiento:  horaProcesamiento,
							},
							index: docIndex,
						}
						return // Exit anonymous function, continue to next work item
					}

					// Always send a result, even if response is nil
					if response != nil {
						if len(response.DocumentosProcesados) > 0 {
							docResultChan <- docResult{
								processed: &response.DocumentosProcesados[0],
								index:     docIndex,
							}
						}
						if len(response.DocumentosFallidos) > 0 {
							docResultChan <- docResult{
								failed: &response.DocumentosFallidos[0],
								index:  docIndex,
							}
						}
						// If response exists but has no results, send as failed
						if len(response.DocumentosProcesados) == 0 && len(response.DocumentosFallidos) == 0 {
							docResultChan <- docResult{
								failed: &invoice.FailedDocument{
									Documento:          documentType,
									Consecutivo:        document.CdoConsecutivo,
									Prefijo:            document.RfaPrefijo,
									Errors:             []string{"Response received but no processed or failed documents"},
									FechaProcesamiento: fechaProcesamiento,
									HoraProcesamiento:  horaProcesamiento,
								},
								index: docIndex,
							}
						}
					} else {
						// Response is nil but no error was caught - send as failed
						docResultChan <- docResult{
							failed: &invoice.FailedDocument{
								Documento:          documentType,
								Consecutivo:        document.CdoConsecutivo,
								Prefijo:            document.RfaPrefijo,
								Errors:             []string{"No response received from document processing"},
								FechaProcesamiento: fechaProcesamiento,
								HoraProcesamiento:  horaProcesamiento,
							},
							index: docIndex,
						}
					}
				}() // End of anonymous function - defer will execute here
			}
		}(w)
	}

	// Wait for all documents to complete
	go func() {
		wg.Wait()
		close(docResultChan)
	}()

	// Collect results
	allProcessed := make([]invoice.ProcessedDocument, 0)
	allFailed := make([]invoice.FailedDocument, 0)
	var firstLote string
	resultsReceived := 0
	expectedResults := len(documents)

	// Create a map to store results by index to maintain order
	resultMap := make(map[int]docResult)

	// Add timeout mechanism to prevent infinite waiting
	// Use a reasonable timeout: 10 minutes max wait time
	timeout := time.After(10 * time.Minute)
	collectionDone := false

	for resultsReceived < expectedResults && !collectionDone {
		select {
		case result, ok := <-docResultChan:
			if !ok {
				// Channel closed - exit loop
				c.log.Warn("Result channel closed before all results received",
					"received", resultsReceived, "expected", expectedResults)
				collectionDone = true
				break
			}
			resultsReceived++
			resultMap[result.index] = result

		case <-ctx.Done():
			// Context cancelled
			c.log.Warn("Context cancelled while processing documents",
				"received", resultsReceived, "expected", expectedResults)
			collectionDone = true
			break

		case <-timeout:
			// Timeout waiting for results
			c.log.Error("Timeout waiting for all results",
				"received", resultsReceived, "expected", expectedResults)
			collectionDone = true
			break
		}
	}

	// If we exited early due to channel closure or timeout, mark remaining documents as failed
	if collectionDone && resultsReceived < expectedResults {
		for i := 0; i < len(documents); i++ {
			if _, exists := resultMap[i]; !exists {
				// Document never sent a result - mark as failed
				allFailed = append(allFailed, invoice.FailedDocument{
					Documento:          documentType,
					Consecutivo:        documents[i].CdoConsecutivo,
					Prefijo:            documents[i].RfaPrefijo,
					Errors:             []string{"Document processing incomplete - no result received"},
					FechaProcesamiento: time.Now().Format("2006-01-02"),
					HoraProcesamiento:  time.Now().Format("15:04:05"),
				})
			}
		}
	}

	// Process results in order
	for i := 0; i < len(documents); i++ {
		if result, ok := resultMap[i]; ok {
			if result.processed != nil {
				if firstLote == "" {
					// Use first processed document's lote info if available
					firstLote = fmt.Sprintf("lote-%s-%s", result.processed.FechaProcesamiento, result.processed.HoraProcesamiento)
				}
				allProcessed = append(allProcessed, *result.processed)
			}
			if result.failed != nil {
				allFailed = append(allFailed, *result.failed)
			}
		}
	}

	// Build final response
	var message string
	if len(allProcessed) > 0 && len(allFailed) == 0 {
		message = "Documentos procesados exitosamente"
	} else if len(allFailed) > 0 && len(allProcessed) == 0 {
		message = "Error al procesar documentos"
	} else if len(allProcessed) > 0 && len(allFailed) > 0 {
		message = "Algunos documentos fueron procesados, otros fallaron"
	} else {
		message = "Procesamiento completado"
	}

	if firstLote == "" {
		now := time.Now()
		firstLote = fmt.Sprintf("lote-%s", now.Format("20060102-150405"))
	}

	// Log performance metrics
	duration := time.Since(startTime)
	throughput := float64(len(allProcessed)) / duration.Seconds()
	successRate := float64(len(allProcessed)) / float64(len(documents)) * 100

	// Calculate average time per document
	avgTimePerDoc := duration.Seconds() / float64(len(documents))
	if len(allProcessed) > 0 {
		avgTimePerDoc = duration.Seconds() / float64(len(allProcessed))
	}

	if c.concurrencyLimiter != nil {
		stats := c.concurrencyLimiter.Stats()
		statsMutex.Lock()
		finalPeakActive := peakActiveRequests
		statsMutex.Unlock()

		// Calculate rate limiter utilization if available
		rateLimitInfo := ""
		if c.rateLimiter != nil {
			rateLimitInfo = fmt.Sprintf(" (rate_limit: %d RPS)", c.rateLimiter.Rate())
		}

		// Calculate utilization based on peak, not current (which is 0 after completion)
		utilizationPercent := 0.0
		if stats.MaxConcurrent > 0 {
			utilizationPercent = float64(finalPeakActive) / float64(stats.MaxConcurrent) * 100
		}

		// Add circuit breaker stats if available
		circuitBreakerInfo := ""
		if c.circuitBreaker != nil {
			cbStats := c.circuitBreaker.Stats()
			stateStr := "closed"
			switch cbStats.State {
			case CircuitBreakerOpen:
				stateStr = "open"
			case CircuitBreakerHalfOpen:
				stateStr = "half-open"
			}
			circuitBreakerInfo = fmt.Sprintf(" (circuit_breaker: %s, failure_rate: %.2f%%)", stateStr, cbStats.FailureRate*100)
		}

		c.log.Info("Concurrent document processing completed",
			"total_documents", len(documents),
			"processed", len(allProcessed),
			"failed", len(allFailed),
			"duration_seconds", duration.Seconds(),
			"duration_formatted", duration.String(),
			"throughput_docs_per_sec", fmt.Sprintf("%.2f", throughput),
			"avg_time_per_doc_seconds", fmt.Sprintf("%.3f", avgTimePerDoc),
			"success_rate_percent", fmt.Sprintf("%.2f", successRate),
			"max_concurrent_requests", stats.MaxConcurrent,
			"peak_active_requests", finalPeakActive,
			"current_active_requests", stats.ActiveCount,
			"total_requests_acquired", stats.TotalAcquired,
			"concurrency_utilization_percent", fmt.Sprintf("%.2f", utilizationPercent),
			"rate_limit_info", rateLimitInfo,
			"circuit_breaker_info", circuitBreakerInfo)
	} else {
		c.log.Info("Concurrent document processing completed",
			"total_documents", len(documents),
			"processed", len(allProcessed),
			"failed", len(allFailed),
			"duration_seconds", duration.Seconds(),
			"duration_formatted", duration.String(),
			"throughput_docs_per_sec", fmt.Sprintf("%.2f", throughput),
			"avg_time_per_doc_seconds", fmt.Sprintf("%.3f", avgTimePerDoc),
			"success_rate_percent", fmt.Sprintf("%.2f", successRate))
	}

	// Ensure slices are never nil (use empty slice instead)
	if allProcessed == nil {
		allProcessed = make([]invoice.ProcessedDocument, 0)
	}
	if allFailed == nil {
		allFailed = make([]invoice.FailedDocument, 0)
	}

	return &invoice.DocumentRegistrationResponse{
		Message:              message,
		Lote:                 firstLote,
		DocumentosProcesados: allProcessed,
		DocumentosFallidos:   allFailed,
	}, nil
}

// parseSendDIANResponse parses the response from Numrot SendDIAN/documentSinc API and converts it to DocumentRegistrationResponse.
// This function handles multiple response formats:
// - Direct numrotSendDIANDocumentResponse (single document)
// - Array of numrotSendDIANDocumentResponse (multiple documents)
// - Wrapped in numrotSendDIANResponse with _preview field
// - Legacy numrotRegisterDocumentResponse format
func (c *Client) parseSendDIANResponse(body []byte, documents []invoice.OpenETLDocument, documentType string) (*invoice.DocumentRegistrationResponse, error) {
	// Get current date and time for processing
	now := time.Now()
	loc, err := time.LoadLocation("America/Bogota")
	if err != nil {
		// Fallback to UTC-5 offset if timezone data is not available
		loc = time.FixedZone("America/Bogota", -5*60*60)
	}
	nowColombia := now.In(loc)
	fechaProcesamiento := nowColombia.Format("2006-01-02")
	horaProcesamiento := nowColombia.Format("15:04:05")

	procesados := make([]invoice.ProcessedDocument, 0)
	fallidos := make([]invoice.FailedDocument, 0)
	var lote string // Will be set from first document's TrackId/Uuid

	// Helper function to process a single document response
	processDocumentResponse := func(docResp numrotSendDIANDocumentResponse, index int) {
		// Capture lote from first document's TrackId or Uuid
		if lote == "" {
			lote = docResp.TrackId
			if lote == "" {
				lote = docResp.Uuid
			}
		}

		// Extract prefix and consecutive from DocumentNumber
		prefijo, consecutivo := extractPrefixAndConsecutive(docResp.DocumentNumber)

		// Map to request document if possible (by matching DocumentNumber or by index)
		if prefijo == "" || consecutivo == "" {
			// Try to match by DocumentNumber with request documents
			matched := false
			for _, reqDoc := range documents {
				expectedDocNumber := reqDoc.RfaPrefijo + reqDoc.CdoConsecutivo
				if docResp.DocumentNumber == expectedDocNumber {
					prefijo = reqDoc.RfaPrefijo
					consecutivo = reqDoc.CdoConsecutivo
					matched = true
					break
				}
			}

			// If no match, use index-based fallback
			if !matched && index >= 0 && index < len(documents) {
				prefijo = documents[index].RfaPrefijo
				consecutivo = documents[index].CdoConsecutivo
			}

			// Final fallback
			if prefijo == "" && consecutivo == "" {
				if len(documents) > 0 {
					prefijo = documents[0].RfaPrefijo
					consecutivo = documents[0].CdoConsecutivo
				} else {
					prefijo = docResp.DocumentNumber
					consecutivo = ""
				}
			}
		}

		// Determine if document was processed successfully or failed
		// StatusCode "200" = success, anything else = failure
		statusCodeInt := 0
		if docResp.StatusCode != "" {
			fmt.Sscanf(docResp.StatusCode, "%d", &statusCodeInt)
		}

		isFailed := statusCodeInt != 200

		if isFailed {
			// Build error messages
			errorMessages := make([]string, 0)
			if docResp.ErrorMessage != "" {
				errorMessages = append(errorMessages, docResp.ErrorMessage)
			}
			errorMessages = append(errorMessages, docResp.ErrorReason...)
			// Include warnings in error messages for failed documents
			errorMessages = append(errorMessages, docResp.Warnings...)
			if len(errorMessages) == 0 {
				errorMessages = append(errorMessages, "Error desconocido")
			}

			fallidos = append(fallidos, invoice.FailedDocument{
				Documento:          documentType,
				Consecutivo:        consecutivo,
				Prefijo:            prefijo,
				Errors:             errorMessages,
				FechaProcesamiento: fechaProcesamiento,
				HoraProcesamiento:  horaProcesamiento,
			})
			c.log.Warn("Document failed",
				"document_number", docResp.DocumentNumber,
				"prefijo", prefijo,
				"consecutivo", consecutivo,
				"errors", errorMessages)
		} else {
			// Document processed successfully (StatusCode 200)
			// Extract cdo_id from response, use consecutivo as fallback if not provided
			cdoID := docResp.CdoID
			if cdoID == 0 {
				// Try to extract numeric ID from TrackId if it's numeric
				if docResp.TrackId != "" {
					if trackIDInt, err := strconv.Atoi(docResp.TrackId); err == nil {
						cdoID = trackIDInt
					}
				}
				// If still 0, use consecutivo as fallback
				if cdoID == 0 && consecutivo != "" {
					if consecutivoInt, err := strconv.Atoi(consecutivo); err == nil {
						cdoID = consecutivoInt
					}
				}
			}

			procesados = append(procesados, invoice.ProcessedDocument{
				CdoID:              cdoID,
				RfaPrefijo:         prefijo,
				CdoConsecutivo:     consecutivo,
				FechaProcesamiento: fechaProcesamiento,
				HoraProcesamiento:  horaProcesamiento,
				XmlBase64:          docResp.Document,
				PdfBase64:          docResp.Pdfdocument,
			})
			c.log.Info("Document processed successfully",
				"document_number", docResp.DocumentNumber,
				"prefijo", prefijo,
				"consecutivo", consecutivo)
		}
	}

	// Try to parse directly as numrotSendDIANDocumentResponse (single document) - this is numrot's actual format
	var docResp numrotSendDIANDocumentResponse
	if err := json.Unmarshal(body, &docResp); err == nil && docResp.StatusCode != "" {
		c.log.Info("Parsed response as single document (direct format)",
			"status_code", docResp.StatusCode,
			"document_number", docResp.DocumentNumber,
			"status_message", docResp.StatusMessage)

		processDocumentResponse(docResp, 0)

		// Build message based on results
		var message string
		if len(procesados) > 0 && len(fallidos) == 0 {
			message = "Documentos procesados exitosamente"
		} else if len(fallidos) > 0 && len(procesados) == 0 {
			message = "Error al procesar documentos"
		} else if len(procesados) > 0 && len(fallidos) > 0 {
			message = "Algunos documentos fueron procesados, otros fallaron"
		} else {
			message = "Procesamiento completado"
		}

		// If lote is still empty, generate timestamp-based lote
		if lote == "" {
			now := time.Now()
			lote = fmt.Sprintf("lote-%s", now.Format("20060102-150405"))
		}

		// Ensure slices are never nil (use empty slice instead)
		if procesados == nil {
			procesados = make([]invoice.ProcessedDocument, 0)
		}
		if fallidos == nil {
			fallidos = make([]invoice.FailedDocument, 0)
		}

		return &invoice.DocumentRegistrationResponse{
			Message:              message,
			Lote:                 lote,
			DocumentosProcesados: procesados,
			DocumentosFallidos:   fallidos,
		}, nil
	}

	// Try to parse as array of numrotSendDIANDocumentResponse (multiple documents)
	var docRespArray []numrotSendDIANDocumentResponse
	if err := json.Unmarshal(body, &docRespArray); err == nil && len(docRespArray) > 0 {
		c.log.Info("Parsed response as array of documents", "document_count", len(docRespArray))

		// Process each document response
		for i, docResp := range docRespArray {
			c.log.Info("Processing document response",
				"index", i,
				"status_code", docResp.StatusCode,
				"document_number", docResp.DocumentNumber,
				"status_message", docResp.StatusMessage,
				"error_reason_count", len(docResp.ErrorReason),
				"warnings_count", len(docResp.Warnings))

			processDocumentResponse(docResp, i)
		}

		c.log.Info("Final document classification",
			"procesados_count", len(procesados),
			"fallidos_count", len(fallidos))

		// Build message based on results
		var message string
		if len(procesados) > 0 && len(fallidos) == 0 {
			message = "Documentos procesados exitosamente"
		} else if len(fallidos) > 0 && len(procesados) == 0 {
			message = "Error al procesar documentos"
		} else if len(procesados) > 0 && len(fallidos) > 0 {
			message = "Algunos documentos fueron procesados, otros fallaron"
		} else {
			message = "Procesamiento completado"
		}

		// If lote is still empty, generate timestamp-based lote
		if lote == "" {
			now := time.Now()
			lote = fmt.Sprintf("lote-%s", now.Format("20060102-150405"))
		}

		// Ensure slices are never nil (use empty slice instead)
		if procesados == nil {
			procesados = make([]invoice.ProcessedDocument, 0)
		}
		if fallidos == nil {
			fallidos = make([]invoice.FailedDocument, 0)
		}

		return &invoice.DocumentRegistrationResponse{
			Message:              message,
			Lote:                 lote,
			DocumentosProcesados: procesados,
			DocumentosFallidos:   fallidos,
		}, nil
	}

	// Try to parse as wrapper format (with _size and _preview) - for backward compatibility
	var sendDIANResp numrotSendDIANResponse
	if err := json.Unmarshal(body, &sendDIANResp); err == nil && sendDIANResp.Preview != "" {
		c.log.Info("Detected wrapper format (with _size and _preview)", "size", sendDIANResp.Size, "preview_length", len(sendDIANResp.Preview))

		// Try to parse _preview as array first (for multiple documents)
		var docRespArray []numrotSendDIANDocumentResponse
		if err := json.Unmarshal([]byte(sendDIANResp.Preview), &docRespArray); err == nil && len(docRespArray) > 0 {
			c.log.Info("Parsed _preview as array", "document_count", len(docRespArray))

			// Process each document response
			for i, docResp := range docRespArray {
				c.log.Info("Processing document response",
					"index", i,
					"status_code", docResp.StatusCode,
					"document_number", docResp.DocumentNumber,
					"status_message", docResp.StatusMessage,
					"error_reason_count", len(docResp.ErrorReason),
					"warnings_count", len(docResp.Warnings))

				processDocumentResponse(docResp, i)
			}
		} else {
			// Try to parse as single document
			var docResp numrotSendDIANDocumentResponse
			if err := json.Unmarshal([]byte(sendDIANResp.Preview), &docResp); err != nil {
				c.log.Error("Failed to unmarshal _preview JSON (both array and single)", "error", err, "preview_length", len(sendDIANResp.Preview))
				return nil, fmt.Errorf("unmarshal _preview: %w", err)
			}

			c.log.Info("Parsed _preview as single document",
				"status_code", docResp.StatusCode,
				"document_number", docResp.DocumentNumber,
				"status_message", docResp.StatusMessage,
				"error_reason_count", len(docResp.ErrorReason),
				"warnings_count", len(docResp.Warnings))

			processDocumentResponse(docResp, 0)
		}

		c.log.Info("Final document classification",
			"procesados_count", len(procesados),
			"fallidos_count", len(fallidos))

		// Build message based on results
		var message string
		if len(procesados) > 0 && len(fallidos) == 0 {
			message = "Documentos procesados exitosamente"
		} else if len(fallidos) > 0 && len(procesados) == 0 {
			message = "Error al procesar documentos"
		} else if len(procesados) > 0 && len(fallidos) > 0 {
			message = "Algunos documentos fueron procesados, otros fallaron"
		} else {
			message = "Procesamiento completado"
		}

		// If lote is still empty, generate timestamp-based lote
		if lote == "" {
			now := time.Now()
			lote = fmt.Sprintf("lote-%s", now.Format("20060102-150405"))
		}

		// Ensure slices are never nil (use empty slice instead)
		if procesados == nil {
			procesados = make([]invoice.ProcessedDocument, 0)
		}
		if fallidos == nil {
			fallidos = make([]invoice.FailedDocument, 0)
		}

		return &invoice.DocumentRegistrationResponse{
			Message:              message,
			Lote:                 lote,
			DocumentosProcesados: procesados,
			DocumentosFallidos:   fallidos,
		}, nil
	}

	// Fallback to legacy format for backward compatibility
	c.log.Info("Trying legacy response format")
	var numrotResp numrotRegisterDocumentResponse
	if err := json.Unmarshal(body, &numrotResp); err != nil {
		c.log.Error("Failed to unmarshal Numrot SendDIAN/documentSinc response (all formats)", "error", err, "body_length", len(body))
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	c.log.Info("Numrot SendDIAN/documentSinc API response parsed (legacy format)",
		"message", numrotResp.Message,
		"lote", numrotResp.Lote,
		"procesados_count", len(numrotResp.DocumentosProcesados),
		"fallidos_count", len(numrotResp.DocumentosFallidos))

	// Transform response to domain format
	procesados = make([]invoice.ProcessedDocument, 0, len(numrotResp.DocumentosProcesados))
	for _, pd := range numrotResp.DocumentosProcesados {
		procesados = append(procesados, invoice.ProcessedDocument{
			CdoID:              pd.CdoID,
			RfaPrefijo:         pd.RfaPrefijo,
			CdoConsecutivo:     pd.CdoConsecutivo,
			FechaProcesamiento: pd.FechaProcesamiento,
			HoraProcesamiento:  pd.HoraProcesamiento,
			XmlBase64:          "", // Legacy format doesn't have XML/PDF
			PdfBase64:          "", // Legacy format doesn't have XML/PDF
		})
		c.log.Debug("Processed document", "cdo_id", pd.CdoID, "prefijo", pd.RfaPrefijo, "consecutivo", pd.CdoConsecutivo)
	}

	fallidos = make([]invoice.FailedDocument, 0, len(numrotResp.DocumentosFallidos))
	for _, fd := range numrotResp.DocumentosFallidos {
		errors := make([]string, 0)
		switch v := fd.Errors.(type) {
		case []string:
			errors = v
		case []interface{}:
			for _, e := range v {
				if errStr, ok := e.(string); ok {
					errors = append(errors, errStr)
				} else {
					errors = append(errors, fmt.Sprintf("%v", e))
				}
			}
		case string:
			errors = []string{v}
		default:
			errors = []string{fmt.Sprintf("%v", v)}
		}

		fallidos = append(fallidos, invoice.FailedDocument{
			Documento:          fd.Documento,
			Consecutivo:        fd.Consecutivo,
			Prefijo:            fd.Prefijo,
			Errors:             errors,
			FechaProcesamiento: fd.FechaProcesamiento,
			HoraProcesamiento:  fd.HoraProcesamiento,
		})
		c.log.Debug("Failed document", "documento", fd.Documento, "prefijo", fd.Prefijo, "consecutivo", fd.Consecutivo, "errors_count", len(errors))
	}

	// Build message
	message := numrotResp.Message
	if message == "" {
		if len(procesados) > 0 && len(fallidos) == 0 {
			message = "Documentos procesados exitosamente"
		} else if len(fallidos) > 0 && len(procesados) == 0 {
			message = "Error al procesar documentos"
		} else if len(procesados) > 0 && len(fallidos) > 0 {
			message = "Algunos documentos fueron procesados, otros fallaron"
		} else {
			message = "Procesamiento completado"
		}
	}

	// Always return the response with processed and failed documents
	// Ensure slices are never nil (use empty slice instead)
	if procesados == nil {
		procesados = make([]invoice.ProcessedDocument, 0)
	}
	if fallidos == nil {
		fallidos = make([]invoice.FailedDocument, 0)
	}

	c.log.Info("Returning document registration response",
		"procesados", len(procesados),
		"fallidos", len(fallidos),
		"message", message,
		"lote", numrotResp.Lote)

	return &invoice.DocumentRegistrationResponse{
		Message:              message,
		Lote:                 numrotResp.Lote,
		DocumentosProcesados: procesados,
		DocumentosFallidos:   fallidos,
	}, nil
}

// registerDSDocument registers a DS (Documento Soporte) document with Numrot documentSinc API.
func (c *Client) registerDSDocument(ctx context.Context, doc invoice.OpenETLDocument, token string) (*invoice.DocumentRegistrationResponse, error) {
	// Transform document to Numrot format
	numrotInv, err := c.transformOpenETLToNumrot(ctx, doc, "DS")
	if err != nil {
		c.log.Error("Failed to transform DS document", "error", err, "prefijo", doc.RfaPrefijo, "consecutivo", doc.CdoConsecutivo)
		return nil, fmt.Errorf("transform document %s%s: %w", doc.RfaPrefijo, doc.CdoConsecutivo, err)
	}

	// Build request payload
	jsonData, err := json.Marshal(numrotInv)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Build documentSinc URL: /api/documentSinc/{nit}/{documento}
	// nit: NIT del OFE (sin DV)
	// documento: Número completo del documento (prefijo + consecutivo)
	// For DS: adq_identificacion in request is the actual ofe_identificacion
	// Use DS-specific base URL if configured, otherwise fallback to baseURL
	dsURL := c.dsBaseURL
	if dsURL == "" {
		dsURL = c.baseURL
	}
	// For DS, use adq_identificacion (which is the actual ofe_identificacion) instead of ofe_identificacion
	url := buildDocumentSincURL(dsURL, doc.AdqIdentificacion, doc.RfaPrefijo, doc.CdoConsecutivo)

	// Create HTTP request
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	httpReq.Header.Set("Accept-Encoding", "gzip")

	c.log.Debug("Registering DS document with Numrot documentSinc", "url", url, "prefijo", doc.RfaPrefijo, "consecutivo", doc.CdoConsecutivo)

	// Execute request with retry logic for timeout errors
	var resp *http.Response
	var apiLatency time.Duration
	maxRetries := 3
	retryDelays := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		apiStartTime := time.Now()
		resp, err = c.httpClient.Do(httpReq)
		apiLatency = time.Since(apiStartTime)

		if err == nil {
			break
		}

		isTimeout := false
		if err != nil {
			errStr := err.Error()
			isTimeout = strings.Contains(errStr, "timeout") ||
				strings.Contains(errStr, "deadline exceeded") ||
				strings.Contains(errStr, "Client.Timeout")
		}

		if !isTimeout || attempt >= maxRetries {
			c.log.Error("Failed to execute request to Numrot documentSinc",
				"error", err,
				"url", url,
				"api_latency_ms", apiLatency.Milliseconds(),
				"attempt", attempt+1,
				"max_retries", maxRetries+1,
				"is_timeout", isTimeout)
			return nil, fmt.Errorf("execute request: %w", err)
		}

		c.log.Warn("Request timeout, retrying",
			"url", url,
			"attempt", attempt+1,
			"max_retries", maxRetries+1,
			"retry_delay", retryDelays[attempt])

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled during retry: %w", ctx.Err())
		case <-time.After(retryDelays[attempt]):
			httpReq.Body = io.NopCloser(bytes.NewBuffer(jsonData))
		}
	}

	if resp == nil {
		return nil, fmt.Errorf("failed to get response after %d attempts", maxRetries+1)
	}
	defer resp.Body.Close()

	c.log.Debug("Numrot documentSinc API request completed",
		"api_latency_ms", apiLatency.Milliseconds(),
		"api_latency_seconds", apiLatency.Seconds(),
		"status_code", resp.StatusCode)

	// Handle gzip compression if present
	var reader io.Reader = resp.Body
	if strings.Contains(resp.Header.Get("Content-Encoding"), "gzip") {
		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			c.log.Error("Failed to create gzip reader", "error", err)
			return nil, fmt.Errorf("create gzip reader: %w", err)
		}
		defer gzipReader.Close()
		reader = gzipReader
		c.log.Debug("Response is gzip compressed, decompressing")
	}

	// Read response body
	body, err := io.ReadAll(reader)
	if err != nil {
		c.log.Error("Failed to read response body from Numrot documentSinc", "error", err, "status", resp.StatusCode)
		return nil, fmt.Errorf("read response body: %w", err)
	}

	c.log.Debug("Numrot documentSinc API response", "status", resp.StatusCode, "body_length", len(body))

	if resp.StatusCode == http.StatusUnauthorized {
		c.auth.ClearToken()
		c.log.Warn("Token expired or invalid, clearing cache", "status", resp.StatusCode, "body_length", len(body))
		return nil, fmt.Errorf("authentication failed: token expired or invalid")
	}

	if resp.StatusCode != http.StatusOK {
		c.log.Error("Numrot documentSinc API returned non-OK status", "status", resp.StatusCode, "body_length", len(body))
		return nil, fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	// Parse documentSinc response (same format as SendDIAN)
	return c.parseSendDIANResponse(body, []invoice.OpenETLDocument{doc}, "DS")
}

// registerDocumentsSingleBatch processes a single batch of documents (original implementation)
// skipLimiters: if true, skips acquiring limiters (used when already acquired in concurrent path)
func (c *Client) registerDocumentsSingleBatch(ctx context.Context, req invoice.DocumentRegistrationRequest, documents []invoice.OpenETLDocument, documentType string, skipLimiters bool) (*invoice.DocumentRegistrationResponse, error) {
	// Acquire concurrency limiter slot if available and not already acquired
	if !skipLimiters && c.concurrencyLimiter != nil {
		if err := c.concurrencyLimiter.Acquire(ctx); err != nil {
			return nil, fmt.Errorf("failed to acquire concurrency slot: %w", err)
		}
		defer c.concurrencyLimiter.Release()

		stats := c.concurrencyLimiter.Stats()
		if stats.ActiveCount > 900 {
			c.log.Warn("High concurrency usage detected",
				"active_requests", stats.ActiveCount,
				"max_concurrent", stats.MaxConcurrent,
				"available", stats.Available)
		}
	}

	// Acquire rate limiter token FIRST (before concurrency limiter) if not already acquired
	if !skipLimiters && c.rateLimiter != nil {
		if err := c.rateLimiter.Acquire(ctx); err != nil {
			return nil, fmt.Errorf("failed to acquire rate limit token: %w", err)
		}
	}

	token, err := c.auth.GetToken(ctx)
	if err != nil {
		c.log.Error("Failed to get Numrot authentication token", "error", err)
		return nil, fmt.Errorf("get authentication token: %w", err)
	}

	// DS documents use a different endpoint (SendEnr) than FC/NC/ND (SendDIAN)
	if documentType == "DS" {
		return c.registerDSDocument(ctx, documents[0], token)
	}

	// Transform documents to Numrot format
	numrotInvoices := make([]numrotInvoice, 0, len(documents))
	for _, doc := range documents {
		numrotInv, err := c.transformOpenETLToNumrot(ctx, doc, documentType)
		if err != nil {
			c.log.Error("Failed to transform document", "error", err, "prefijo", doc.RfaPrefijo, "consecutivo", doc.CdoConsecutivo)
			return nil, fmt.Errorf("transform document %s%s: %w", doc.RfaPrefijo, doc.CdoConsecutivo, err)
		}
		numrotInvoices = append(numrotInvoices, numrotInv)
	}

	// Build request - Numrot API only accepts one document per request
	// Always send single invoice object directly
	if len(numrotInvoices) != 1 {
		return nil, fmt.Errorf("internal error: registerDocumentsSingleBatch should only process one document at a time")
	}
	jsonData, err := json.Marshal(numrotInvoices[0])
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// FC, NC, and ND documents use the SendDIAN endpoint
	// Endpoint: /api/SendDIAN/Json/Pdf
	// Host: https://numrotapiprueba.net (configured via NUMROT_BASE_URL)
	url := fmt.Sprintf("%s/api/SendDIAN/Json/Pdf", c.baseURL)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	httpReq.Header.Set("Accept-Encoding", "gzip")

	c.log.Debug("Registering documents with Numrot SendDIAN", "documentType", documentType, "count", len(documents), "url", url)

	// Execute request with retry logic for timeout errors
	var resp *http.Response
	var apiLatency time.Duration
	maxRetries := 3
	retryDelays := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}

	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Measure API latency
		apiStartTime := time.Now()
		resp, err = c.httpClient.Do(httpReq)
		apiLatency = time.Since(apiStartTime)

		if err == nil {
			// Success, break retry loop
			break
		}

		// Check if error is a timeout and we should retry
		isTimeout := false
		if err != nil {
			errStr := err.Error()
			isTimeout = strings.Contains(errStr, "timeout") ||
				strings.Contains(errStr, "deadline exceeded") ||
				strings.Contains(errStr, "Client.Timeout")
		}

		// Don't retry on non-timeout errors or if we've exhausted retries
		if !isTimeout || attempt >= maxRetries {
			c.log.Error("Failed to execute request to Numrot SendDIAN",
				"error", err,
				"url", url,
				"api_latency_ms", apiLatency.Milliseconds(),
				"attempt", attempt+1,
				"max_retries", maxRetries+1,
				"is_timeout", isTimeout)
			return nil, fmt.Errorf("execute request: %w", err)
		}

		// Log retry attempt
		c.log.Warn("Request timeout, retrying",
			"url", url,
			"attempt", attempt+1,
			"max_retries", maxRetries+1,
			"retry_delay", retryDelays[attempt])

		// Wait before retry with exponential backoff
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled during retry: %w", ctx.Err())
		case <-time.After(retryDelays[attempt]):
			// Recreate request body (it was consumed in previous attempt)
			httpReq.Body = io.NopCloser(bytes.NewBuffer(jsonData))
		}
	}

	if resp == nil {
		return nil, fmt.Errorf("failed to get response after %d attempts", maxRetries+1)
	}
	defer resp.Body.Close()

	// Log API latency for performance monitoring
	c.log.Debug("Numrot API request completed",
		"api_latency_ms", apiLatency.Milliseconds(),
		"api_latency_seconds", apiLatency.Seconds(),
		"status_code", resp.StatusCode)

	// Handle gzip compression if present
	var reader io.Reader = resp.Body
	if strings.Contains(resp.Header.Get("Content-Encoding"), "gzip") {
		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			c.log.Error("Failed to create gzip reader", "error", err)
			return nil, fmt.Errorf("create gzip reader: %w", err)
		}
		defer gzipReader.Close()
		reader = gzipReader
		c.log.Debug("Response is gzip compressed, decompressing")
	}

	body, err := io.ReadAll(reader)
	if err != nil {
		c.log.Error("Failed to read response body from Numrot SendDIAN", "error", err, "status", resp.StatusCode)
		return nil, fmt.Errorf("read response body: %w", err)
	}

	c.log.Debug("Numrot SendDIAN API response", "status", resp.StatusCode, "body_length", len(body))

	if resp.StatusCode == http.StatusUnauthorized {
		// Token might be expired, clear cache
		c.auth.ClearToken()
		c.log.Warn("Token expired or invalid, clearing cache", "status", resp.StatusCode, "body_length", len(body))
		return nil, fmt.Errorf("authentication failed: token expired or invalid")
	}

	if resp.StatusCode != http.StatusOK {
		c.log.Error("Numrot SendDIAN API returned non-OK status", "status", resp.StatusCode, "body_length", len(body))
		return nil, fmt.Errorf("unexpected status code %d", resp.StatusCode)
	}

	// Log response info (body is saved to provider_audit_log, not logged to console)
	c.log.Debug("Numrot SendDIAN API response received", "status", resp.StatusCode, "body_length", len(body))

	// Determine document type from request
	docType := "FC"
	if len(req.Documentos.NC) > 0 {
		docType = "NC"
	} else if len(req.Documentos.ND) > 0 {
		docType = "ND"
	} else if len(req.Documentos.DS) > 0 {
		docType = "DS"
	}

	// Parse SendDIAN response using reusable function
	return c.parseSendDIANResponse(body, documents, docType)
}

// transformOpenETLToNumrot transforms an OpenETL document to Numrot format.
func (c *Client) transformOpenETLToNumrot(ctx context.Context, doc invoice.OpenETLDocument, documentType string) (numrotInvoice, error) {
	// Map document type code
	// Use top_codigo for CustomizationID and tde_codigo for InvoiceTypeCode
	invoiceTypeCode := doc.TdeCodigo
	customizationID := doc.TopCodigo

	// If top_codigo is empty, use default values based on documentType
	if customizationID == "" {
		if documentType == "NC" {
			customizationID = "22"
		} else if documentType == "ND" {
			customizationID = "32"
		} else if documentType == "DS" {
			// DS uses CustomizationID "10" (same as FC) and InvoiceTypeCode "05"
			customizationID = "10"
			invoiceTypeCode = "05"
		} else {
			customizationID = "10" // Default for FC and DS
		}
	} else if documentType == "DS" {
		// DS always uses InvoiceTypeCode "05" regardless of tde_codigo
		invoiceTypeCode = "05"
	}

	// Build invoice control from resolution data
	// For NC/ND: all fields must be empty strings except Prefix which is "NC" or "ND"
	// For FC/DS: use resolution data or hardcoded values if resolutions are disabled
	var invoiceControl numrotInvoiceControl
	if documentType == "NC" || documentType == "ND" {
		// For NC and ND, all fields must be empty strings except Prefix
		// If rfa_prefijo is empty, default to "NC" for Credit Notes or "ND" for Debit Notes
		prefix := doc.RfaPrefijo
		if prefix == "" {
			if documentType == "NC" {
				prefix = "NC"
			} else {
				prefix = "ND"
			}
		}
		invoiceControl = numrotInvoiceControl{
			InvoiceAuthorization: "",
			StartDate:            "",
			EndDate:              "",
			Prefix:               prefix,
			From:                 "",
			To:                   "",
		}
	} else {
		// For FC and DS, use resolution data
		invoiceControl = numrotInvoiceControl{
			InvoiceAuthorization: doc.RfaResolucion,
			StartDate:            getStringValue(doc.RfaFechaInicio),
			EndDate:              getStringValue(doc.RfaFechaFin),
			Prefix:               doc.RfaPrefijo,
			From:                 getStringValue(doc.RfaNumeroInicio),
			To:                   getStringValue(doc.RfaNumeroFin),
		}
	}

	// For FC/DS, use hardcoded values if resolutions are disabled
	if documentType != "NC" && documentType != "ND" {
		if !c.resolutionsEnabled {
			if invoiceControl.InvoiceAuthorization == "" && c.hardcodedInvoiceAuth != "" {
				invoiceControl.InvoiceAuthorization = c.hardcodedInvoiceAuth
				c.log.Debug("Using hardcoded InvoiceAuthorization", "value", c.hardcodedInvoiceAuth)
			}
			if invoiceControl.StartDate == "" && c.hardcodedStartDate != "" {
				invoiceControl.StartDate = c.hardcodedStartDate
				c.log.Debug("Using hardcoded StartDate", "value", c.hardcodedStartDate)
			}
			if invoiceControl.EndDate == "" && c.hardcodedEndDate != "" {
				invoiceControl.EndDate = c.hardcodedEndDate
				c.log.Debug("Using hardcoded EndDate", "value", c.hardcodedEndDate)
			}
			if invoiceControl.Prefix == "" && c.hardcodedPrefix != "" {
				invoiceControl.Prefix = c.hardcodedPrefix
				c.log.Debug("Using hardcoded Prefix", "value", c.hardcodedPrefix)
			}
			if invoiceControl.From == "" && c.hardcodedFrom != "" {
				invoiceControl.From = c.hardcodedFrom
				c.log.Debug("Using hardcoded From", "value", c.hardcodedFrom)
			}
			if invoiceControl.To == "" && c.hardcodedTo != "" {
				invoiceControl.To = c.hardcodedTo
				c.log.Debug("Using hardcoded To", "value", c.hardcodedTo)
			}
		}
	}

	// Build ID
	invoiceID := doc.RfaPrefijo + doc.CdoConsecutivo

	// Add timezone to IssueTime
	issueTime := doc.CdoHora
	if !strings.Contains(issueTime, "-") && !strings.Contains(issueTime, "+") {
		issueTime = issueTime + "-05:00"
	}

	// Build payment means
	paymentMeans := make([]numrotPaymentMeans, 0, len(doc.CdoMediosPago))
	for _, mp := range doc.CdoMediosPago {
		// Use fpa_codigo as ID, fallback to "1" if empty
		paymentID := mp.FpaCodigo
		if paymentID == "" {
			paymentID = "1"
		}
		paymentMeans = append(paymentMeans, numrotPaymentMeans{
			ID:               paymentID,
			PaymentMeansCode: mp.MpaCodigo,
			PaymentDueDate:   mp.MenFechaVencimiento,
			PaymentID:        []string{mp.MpaCodigo},
		})
	}

	// Build payment exchange rate
	// For DS documents, always include PaymentExchangeRate
	// For FC/NC/ND, only include if different currencies are used
	var paymentExchangeRate *numrotPaymentExchangeRate
	if documentType == "DS" {
		// DS: Always include PaymentExchangeRate with default values
		// Use purchase date from first item if available, otherwise use document date
		exchangeDate := doc.CdoFecha
		if len(doc.Items) > 0 && doc.Items[0].DdoFechaCompra != nil && doc.Items[0].DdoFechaCompra.FechaCompra != "" {
			exchangeDate = doc.Items[0].DdoFechaCompra.FechaCompra
		}
		paymentExchangeRate = &numrotPaymentExchangeRate{
			SourceCurrencyCode:     doc.MonCodigo,
			SourceCurrencyBaseRate: "1.00",
			TargetCurrencyCode:     doc.MonCodigo,
			TargetCurrencyBaseRate: "1.00",
			CalculationRate:        "1",
			Date:                   exchangeDate,
		}
	}
	// For FC/NC/ND, PaymentExchangeRate is optional and should be omitted for same currency

	// Build prepaid payments if there's an anticipo
	var prePaidPayments []numrotPrePaidPayment
	if doc.CdoAnticipo != "" && doc.CdoAnticipo != "0" && doc.CdoAnticipo != "0.00" {
		// Parse anticipo amount
		anticipoAmount, err := parseFloat(doc.CdoAnticipo)
		if err == nil && anticipoAmount > 0 {
			prePaidPayments = []numrotPrePaidPayment{
				{
					ID:           "1",
					CurrencyID:   doc.MonCodigo,
					PaidAmount:   doc.CdoAnticipo,
					ReceivedDate: doc.CdoFecha, // Use issue date as received date
				},
			}
		}
	}

	// Build tax totals for FC, NC, and ND when taxes are present
	// DS documents do not include taxes
	var taxTotals []numrotTaxTotal
	if documentType == "FC" || documentType == "NC" || documentType == "ND" {
		taxTotals = c.buildTaxTotals(doc, documentType)
	}

	// Build legal monetary total
	// For DS: TaxExclusiveAmount must always be "0.00"
	// For FC/NC/ND: TaxExclusiveAmount equals LineExtensionAmount (cdo_valor_sin_impuestos) only if there are taxes
	// If no taxes (taxTotals is empty), TaxExclusiveAmount must be "0.00"
	var taxExclusiveAmount string
	if documentType == "DS" {
		taxExclusiveAmount = "0.00"
	} else {
		// FC, NC, and ND documents: TaxExclusiveAmount equals LineExtensionAmount only if there are taxes
		if len(taxTotals) == 0 {
			// No taxes: TaxExclusiveAmount must be "0.00"
			taxExclusiveAmount = "0.00"
		} else {
			// Has taxes: TaxExclusiveAmount equals LineExtensionAmount
			taxExclusiveAmount = doc.CdoValorSinImpuestos
		}
	}

	// TaxInclusiveAmount and PayableAmount use cdo_total (includes taxes for FC/NC/ND)
	taxInclusiveAmount := doc.CdoTotal
	payableAmount := doc.CdoTotal

	legalMonetaryTotal := numrotLegalMonetaryTotal{
		LineExtensionAmount:  doc.CdoValorSinImpuestos,
		TaxExclusiveAmount:   taxExclusiveAmount,
		TaxInclusiveAmount:   taxInclusiveAmount,
		AllowanceTotalAmount: "0.00",
		PayableAmount:        payableAmount,
		CurrencyID:           doc.MonCodigo,
	}

	// For DS documents, PrePaidAmount can be empty string (as shown in working examples)
	// For FC/NC/ND documents, normalize and include PrePaidAmount
	if documentType == "DS" {
		// DS documents: include PrePaidAmount as-is (empty string is acceptable)
		prePaidValue := doc.CdoAnticipo
		legalMonetaryTotal.PrePaidAmount = &prePaidValue
	} else {
		// FC/NC/ND documents: normalize empty values to "0.00"
		normalizedPrePaid := normalizeMonetaryValue(doc.CdoAnticipo)
		legalMonetaryTotal.PrePaidAmount = &normalizedPrePaid
	}

	// Build invoice lines
	invoiceLines := make([]numrotInvoiceLine, 0, len(doc.Items))
	for i, item := range doc.Items {
		invoiceLine := c.buildInvoiceLine(item, doc, documentType, strconv.Itoa(i+1))
		invoiceLines = append(invoiceLines, invoiceLine)
	}

	// Parse supplier NIT with DV (fixes DIAN FAJ24)
	// Validate that OfeIdentificacion is not empty before parsing
	if doc.OfeIdentificacion == "" {
		return numrotInvoice{}, fmt.Errorf("ofe_identificacion is required")
	}

	supplierBaseNIT, supplierDV := parseNITWithDV(doc.OfeIdentificacion)

	// Determine schemeID: use DV if provided, otherwise use environment-based value
	schemeID := "2" // Default to test
	if supplierDV != "" {
		// Use the DV from the NIT
		schemeID = supplierDV
	} else if doc.CdoAmbiente != nil && *doc.CdoAmbiente == "1" {
		// Production fallback if no DV provided
		schemeID = "6"
	}

	// Use base NIT in CompanyID fields (without DV)
	supplierCompanyID := supplierBaseNIT
	if supplierCompanyID == "" {
		// Fallback if parsing fails - use original NIT
		supplierCompanyID = doc.OfeIdentificacion
	}

	// Build supplier party with location data (fixes DIAN FAJ25, FAK48, FAJ43b)
	// For DS documents, use provider data (from adq_* fields after enrichment)
	// For FC/NC/ND documents, use OFE data (from ofe_* fields)
	var supplierParty numrotAccountingParty
	if documentType == "DS" {
		// DS: Use provider data
		// Note: For DS, ofe_identificacion in request is pro_identificacion of provider
		// Provider data is enriched into adq_* fields, but the original ofe_identificacion
		// (which is pro_identificacion) remains in doc.OfeIdentificacion
		providerName := getStringOrDefault(doc.AdqRazonSocial, doc.OfeIdentificacion)
		providerCompanyID := doc.OfeIdentificacion // This is pro_identificacion from the request

		// Build OFE location (hardcoded) for DS supplier party
		// PhysicalLocation should use OFE hardcoded data, not provider data
		ofeLocation := &numrotPhysicalLocation{
			ID:                   "05380",
			CityName:             "LA ESTRELLA",
			PostalZone:           "55468",
			CountrySubentity:     "ANTIOQUIA",
			CountrySubentityCode: "05",
			Line:                 "CLL 50 - 96",
			IdentificationCode:   "CO",
			Name:                 "Colombia",
		}

		// Parse provider NIT with DV if available
		providerBaseNIT, _ := parseNITWithDV(providerCompanyID)
		if providerBaseNIT == "" {
			providerBaseNIT = providerCompanyID
		}

		// For DS, schemeID should be empty string (not "2" or "6")
		var providerSchemeID *string // nil = empty in JSON

		providerSchemeName := "31"
		supplierParty = numrotAccountingParty{
			AdditionalAccountID: "1",
			Name:                providerName,
			SchemeName:          &providerSchemeName,
			PhysicalLocation:    ofeLocation,
			PartyTaxScheme: numrotPartyTaxScheme{
				RegistrationName:    providerName,
				CompanyID:           providerBaseNIT,
				SchemeID:            providerSchemeID, // Empty for DS
				SchemeName:          &providerSchemeName,
				TaxLevelCode:        "R-99-PN",
				RegistrationAddress: ofeLocation,
				TaxScheme: numrotTaxScheme{
					ID:   "01",
					Name: "IVA",
				},
			},
			PartyLegalEntity: map[string]string{
				"ID": "SEDS",
			},
		}
	} else {
		// FC/NC/ND: Use OFE data from ofe_* fields
		supplierName := getStringOrDefault(doc.OfeRazonSocial, supplierCompanyID)
		supplierLocation := buildSupplierPhysicalLocation(doc)
		supplierSchemeName := "31"

		// Build Contact for FC, NC, and ND documents
		var supplierContact *numrotContact
		if documentType == "FC" || documentType == "NC" || documentType == "ND" {
			supplierContact = &numrotContact{
				Name:           "",
				Telephone:      "",
				Telefax:        "",
				ElectronicMail: "fact.electronica.positiva@3tcapital.co",
			}
		}

		supplierParty = numrotAccountingParty{
			AdditionalAccountID: "1",
			Name:                supplierName,
			SchemeName:          &supplierSchemeName,
			PhysicalLocation:    supplierLocation,
			PartyTaxScheme: numrotPartyTaxScheme{
				RegistrationName:    supplierName,
				CompanyID:           supplierCompanyID,
				SchemeID:            &schemeID,
				SchemeName:          &supplierSchemeName,
				TaxLevelCode:        "R-99-PN",
				RegistrationAddress: supplierLocation,
				TaxScheme: numrotTaxScheme{
					ID:   "01",
					Name: "IVA",
				},
			},
			PartyLegalEntity: numrotPartyLegalEntity{
				RegistrationName: supplierName,
				CompanyID:        supplierCompanyID,
				SchemeID:         &schemeID,
				SchemeName:       &supplierSchemeName,
				ID:               &doc.RfaPrefijo,
			},
			Contact: supplierContact,
		}
	}

	// Build customer party with location data (fixes DIAN FAJ25, FAK48, FAJ43b)
	// For DS documents, use OFE data (hardcoded "Positiva")
	// For FC/NC/ND documents, use acquirer data
	var customerParty numrotAccountingParty
	if documentType == "DS" {
		// DS: Use OFE data (hardcoded "Positiva")
		// Note: For DS, adq_identificacion in request is ofe_identificacion
		// So we use doc.AdqIdentificacion which contains the ofe_identificacion
		ofeName := "Positiva"
		ofeCompanyID := doc.AdqIdentificacion // This is ofe_identificacion from the request
		ofeSchemeID := "6"
		ofeSchemeName := "31"

		// Parse OFE NIT with DV if available
		ofeBaseNIT, ofeDV := parseNITWithDV(ofeCompanyID)
		if ofeBaseNIT == "" {
			ofeBaseNIT = ofeCompanyID
		}
		if ofeDV != "" {
			ofeSchemeID = ofeDV
		}

		customerParty = numrotAccountingParty{
			AdditionalAccountID: "1",
			Name:                ofeName,
			SchemeName:          &ofeSchemeName,
			// Omit PhysicalLocation for DS
			PartyTaxScheme: numrotPartyTaxScheme{
				RegistrationName: ofeName,
				CompanyID:        ofeBaseNIT,
				SchemeID:         &ofeSchemeID,
				SchemeName:       &ofeSchemeName,
				TaxLevelCode:     "R-99-PN",
				// Omit RegistrationAddress for DS
				TaxScheme: numrotTaxScheme{
					ID:   "ZZ",
					Name: "No aplica",
				},
			},
			// Omit PartyLegalEntity for DS
		}
	} else {
		// FC/NC/ND: Use acquirer data
		customerName := getStringOrDefault(doc.AdqRazonSocial, doc.AdqIdentificacion)
		customerLocation := buildCustomerPhysicalLocation(doc)

		// Get acquirer data if repository is available
		var customerSchemeName string = "13" // Default fallback
		var additionalAccountID string = "2" // Default fallback
		var customerContact *numrotContact

		if c.acquirerRepo != nil {
			acq, err := c.acquirerRepo.FindByID(ctx, doc.OfeIdentificacion, doc.AdqIdentificacion, "")
			if err == nil && acq != nil {
				customerSchemeName = acq.TdoCodigo
				additionalAccountID = acq.TojCodigo

				// Build contact information from acquirer data
				// Priority: AccountingContact from Contactos array, fallback to main acquirer fields
				var contactName, contactTelephone, contactFax, contactEmail string

				// Try to find AccountingContact first
				for _, contact := range acq.Contactos {
					if contact.Tipo == "AccountingContact" {
						contactName = contact.Nombre
						contactTelephone = contact.Telefono
						contactEmail = contact.Correo
						break
					}
				}

				// Fallback to main acquirer fields if no AccountingContact found
				if contactName == "" {
					if acq.AdqNombreContacto != nil {
						contactName = *acq.AdqNombreContacto
					} else {
						contactName = acq.AdqRazonSocial
					}
				}
				if contactTelephone == "" && acq.AdqTelefono != nil {
					contactTelephone = *acq.AdqTelefono
				}
				if acq.AdqFax != nil {
					contactFax = *acq.AdqFax
				}

				// Combine emails from multiple sources: AccountingContact email, adq_correo, and adq_correos_notificacion
				// Emails are separated by semicolons in the ElectronicMail field
				combinedEmails := combineAcquirerEmails(contactEmail, acq.AdqCorreo, acq.AdqCorreosNotificacion)

				// Only create Contact if we have at least a telephone
				if contactTelephone != "" {
					customerContact = &numrotContact{
						Name:           contactName,
						Telephone:      contactTelephone,
						Telefax:        contactFax,
						ElectronicMail: combinedEmails,
					}
				}
			}
		}

		customerParty = numrotAccountingParty{
			AdditionalAccountID: additionalAccountID,
			ID:                  &doc.AdqIdentificacion,
			Name:                customerName,
			SchemeName:          &customerSchemeName,
			PhysicalLocation:    customerLocation,
			Contact:             customerContact,
			PartyTaxScheme: numrotPartyTaxScheme{
				RegistrationName:    customerName,
				CompanyID:           doc.AdqIdentificacion,
				SchemeName:          &customerSchemeName,
				TaxLevelCode:        "R-99-PN",
				RegistrationAddress: customerLocation,
				TaxScheme: numrotTaxScheme{
					ID:   "ZZ",
					Name: "No aplica",
				},
			},
			PartyLegalEntity: []numrotPartyLegalEntity{
				{
					RegistrationName: customerName,
					CompanyID:        doc.AdqIdentificacion,
					SchemeName:       &customerSchemeName,
				},
			},
		}
	}

	// Determine environment (1=Production, 2=Test)
	profileExecutionID := "2" // Default to test
	if doc.CdoAmbiente != nil && *doc.CdoAmbiente == "1" {
		profileExecutionID = "1"
	}

	// Build OrderReference if provided
	var orderReference *numrotOrderReference
	if doc.OrderReference != nil && doc.OrderReference.ID != "" {
		orderReference = &numrotOrderReference{
			ID: doc.OrderReference.ID,
		}
	}

	// Use Note array from document
	var notes []string
	if len(doc.Note) > 0 {
		notes = doc.Note
	}

	// Build InvoicePeriod for NC without reference (CustomizationID "22")
	var invoicePeriod *numrotDocumentInvoicePeriod
	if customizationID == "22" && c.ncInvoicePeriodStartDate != "" && c.ncInvoicePeriodEndDate != "" {
		invoicePeriod = &numrotDocumentInvoicePeriod{
			StartDate: c.ncInvoicePeriodStartDate,
			StartTime: c.ncInvoicePeriodStartTime,
			EndDate:   c.ncInvoicePeriodEndDate,
			EndTime:   c.ncInvoicePeriodEndTime,
		}
	}

	// Build DiscrepancyResponse and InvoiceDocumentReference
	// Supports CustomizationID "20" and "30" only, NOT "22" (NC) or "32" (ND)
	var discrepancyResponse []numrotDiscrepancyResponse
	var invoiceDocumentReference *numrotInvoiceDocumentReference

	if (customizationID == "20" || customizationID == "30") &&
		doc.FacturaReferencia != nil &&
		doc.CdoConceptosCorreccion != nil {

		// Construir ReferenceID concatenando prefijo_fc + numero_factura_fc
		referenceID := doc.FacturaReferencia.PrefijoFC + doc.FacturaReferencia.NumeroFacturaFC

		// Validar que tenemos todos los campos necesarios
		if referenceID != "" &&
			doc.CdoConceptosCorreccion.CcoCodigo != "" &&
			doc.CdoConceptosCorreccion.CdoObservacionCorreccion != "" {

			// Construir DiscrepancyResponse (array con un elemento)
			discrepancyResponse = []numrotDiscrepancyResponse{
				{
					ReferenceID:  referenceID,
					ResponseCode: doc.CdoConceptosCorreccion.CcoCodigo,
					Description:  []string{doc.CdoConceptosCorreccion.CdoObservacionCorreccion},
				},
			}

			// Construir InvoiceDocumentReference
			invoiceDocumentReference = &numrotInvoiceDocumentReference{
				ID: referenceID,
			}
		}
	}

	numrotInv := numrotInvoice{
		InvoiceControl:           invoiceControl,
		CustomizationID:          customizationID,
		ProfileExecutionID:       profileExecutionID,
		ID:                       invoiceID,
		IssueDate:                doc.CdoFecha,
		IssueTime:                issueTime,
		DueDate:                  doc.CdoVencimiento,
		InvoiceTypeCode:          invoiceTypeCode,
		Note:                     notes,
		DocumentCurrencyCode:     doc.MonCodigo,
		LineCountNumeric:         fmt.Sprintf("%d", len(doc.Items)),
		OrderReference:           orderReference,
		AccountingSupplierParty:  supplierParty,
		AccountingCustomerParty:  customerParty,
		PaymentMeans:             paymentMeans,
		PrePaidPayment:           prePaidPayments,
		PaymentExchangeRate:      paymentExchangeRate,
		InvoicePeriod:            invoicePeriod,
		DiscrepancyResponse:      discrepancyResponse,
		InvoiceDocumentReference: invoiceDocumentReference,
		LegalMonetaryTotal:       legalMonetaryTotal,
		InvoiceLine:              invoiceLines,
	}

	// Include TaxTotal for FC, NC, and ND documents when taxes are present (not for DS)
	if documentType == "FC" || documentType == "NC" || documentType == "ND" {
		numrotInv.TaxTotal = taxTotals
	}

	return numrotInv, nil
}

// buildTaxTotals builds tax totals from OpenETL tributos.
func (c *Client) buildTaxTotals(doc invoice.OpenETLDocument, documentType string) []numrotTaxTotal {
	if len(doc.Tributos) == 0 {
		// If no tributos and cdo_impuestos is zero, return empty slice (omitempty will exclude it)
		if doc.CdoImpuestos == "" || doc.CdoImpuestos == "0" || doc.CdoImpuestos == "0.00" {
			return []numrotTaxTotal{}
		}
		// Return default tax total with document-level tax
		// RoundingAmount is omitted at document level if empty
		var roundingAmount *string
		if doc.CdoRedondeo != "" && doc.CdoRedondeo != "0" && doc.CdoRedondeo != "0.00" {
			normalized := normalizeMonetaryValue(doc.CdoRedondeo)
			roundingAmount = &normalized
		}
		return []numrotTaxTotal{
			{
				TaxAmount:      doc.CdoImpuestos,
				RoundingAmount: roundingAmount,
				CurrencyID:     doc.MonCodigo,
				TaxSubtotal: []numrotTaxSubtotal{
					{
						TaxableAmount: doc.CdoValorSinImpuestos,
						TaxAmount:     doc.CdoImpuestos,
						Percent:       "0.00",
						CurrencyID:    doc.MonCodigo,
						ID:            "01",
						Name:          "IVA",
					},
				},
			},
		}
	}

	// Group tributos by tax code
	taxMap := make(map[string]*numrotTaxTotal)

	for _, tributo := range doc.Tributos {
		taxCode := tributo.TriCodigo
		if taxCode == "" {
			taxCode = "01" // Default to IVA
		}

		if _, exists := taxMap[taxCode]; !exists {
			// RoundingAmount is omitted at document level if empty
			var roundingAmount *string
			if doc.CdoRedondeo != "" && doc.CdoRedondeo != "0" && doc.CdoRedondeo != "0.00" {
				normalized := normalizeMonetaryValue(doc.CdoRedondeo)
				roundingAmount = &normalized
			}
			taxMap[taxCode] = &numrotTaxTotal{
				TaxAmount:      "0.00",
				RoundingAmount: roundingAmount,
				CurrencyID:     doc.MonCodigo,
				TaxSubtotal:    []numrotTaxSubtotal{},
			}
		}

		percent := "0.00"
		taxableAmount := "0.00"
		if tributo.IidPorcentaje != nil {
			percent = tributo.IidPorcentaje.IidPorcentaje
			taxableAmount = tributo.IidPorcentaje.IidBase
		}

		taxSubtotal := numrotTaxSubtotal{
			TaxableAmount: taxableAmount,
			TaxAmount:     tributo.IidValor,
			Percent:       percent,
			CurrencyID:    doc.MonCodigo,
			ID:            taxCode,
			Name:          c.getTaxName(taxCode),
		}

		taxMap[taxCode].TaxSubtotal = append(taxMap[taxCode].TaxSubtotal, taxSubtotal)
	}

	// Convert map to slice and calculate totals
	taxTotals := make([]numrotTaxTotal, 0, len(taxMap))
	for _, taxTotal := range taxMap {
		// Calculate total tax amount by summing all subtotals
		totalTaxAmount := 0.0
		for _, subtotal := range taxTotal.TaxSubtotal {
			// Parse tax amount from string
			if amount, err := parseFloat(subtotal.TaxAmount); err == nil {
				totalTaxAmount += amount
			}
		}
		taxTotal.TaxAmount = formatFloat(totalTaxAmount)
		// Only include TaxTotal if TaxAmount is greater than zero
		if totalTaxAmount > 0 {
			taxTotals = append(taxTotals, *taxTotal)
		}
	}

	// If no tax totals were built, use document-level tax (only if cdo_impuestos > 0)
	if len(taxTotals) == 0 {
		// Check if cdo_impuestos is zero
		if doc.CdoImpuestos == "" || doc.CdoImpuestos == "0" || doc.CdoImpuestos == "0.00" {
			return []numrotTaxTotal{}
		}
		// RoundingAmount is omitted at document level if empty
		var roundingAmount *string
		if doc.CdoRedondeo != "" && doc.CdoRedondeo != "0" && doc.CdoRedondeo != "0.00" {
			normalized := normalizeMonetaryValue(doc.CdoRedondeo)
			roundingAmount = &normalized
		}
		return []numrotTaxTotal{
			{
				TaxAmount:      doc.CdoImpuestos,
				RoundingAmount: roundingAmount,
				CurrencyID:     doc.MonCodigo,
				TaxSubtotal: []numrotTaxSubtotal{
					{
						TaxableAmount: doc.CdoValorSinImpuestos,
						TaxAmount:     doc.CdoImpuestos,
						Percent:       "0.00",
						CurrencyID:    doc.MonCodigo,
						ID:            "01",
						Name:          "IVA",
					},
				},
			},
		}
	}

	return taxTotals
}

// buildInvoiceLine builds an invoice line from an OpenETL item.
func (c *Client) buildInvoiceLine(item invoice.OpenETLItem, doc invoice.OpenETLDocument, documentType string, id string) numrotInvoiceLine {
	// Find taxes for this item
	// FC, NC, and ND include taxes in lines when Tributos are present
	// DS documents do not include taxes in lines
	itemTaxes := make([]numrotTaxTotal, 0)
	if documentType != "DS" {
		for _, tributo := range doc.Tributos {
			if tributo.DdoSecuencia == item.DdoSecuencia {
				percent := "0.00"
				taxableAmount := item.DdoTotal
				if tributo.IidPorcentaje != nil {
					percent = tributo.IidPorcentaje.IidPorcentaje
					taxableAmount = tributo.IidPorcentaje.IidBase
				}

				// Only include tax if amount is greater than zero
				taxAmount, err := parseFloat(tributo.IidValor)
				if err == nil && taxAmount > 0 {
					// InvoiceLine TaxTotal always includes RoundingAmount
					roundingAmount := "0.00"
					itemTax := numrotTaxTotal{
						TaxAmount:      tributo.IidValor,
						RoundingAmount: &roundingAmount,
						CurrencyID:     doc.MonCodigo,
						TaxSubtotal: []numrotTaxSubtotal{
							{
								TaxableAmount: taxableAmount,
								TaxAmount:     tributo.IidValor,
								Percent:       percent,
								CurrencyID:    doc.MonCodigo,
								ID:            tributo.TriCodigo,
								Name:          c.getTaxName(tributo.TriCodigo),
							},
						},
					}
					itemTaxes = append(itemTaxes, itemTax)
				}
			}
		}
	}

	var sellersID *numrotItemIdentification
	var standardID *numrotItemIdentification

	if item.DdoCodigo != "" {
		sellersID = &numrotItemIdentification{ID: item.DdoCodigo}
		// For DS documents, add schemeID and schemeName to StandardItemIdentification
		if documentType == "DS" {
			schemeID := "999"
			schemeName := "Estándar de adopción del contribuyente"
			standardID = &numrotItemIdentification{
				ID:         item.DdoCodigo,
				SchemeID:   &schemeID,
				SchemeName: &schemeName,
			}
		} else {
			standardID = &numrotItemIdentification{ID: item.DdoCodigo}
		}
	}

	// Map unit code from OpenETL to UBL standard codes
	unitCode := mapUnitCode(item.UndCodigo)

	// Build InvoicePeriod for DS items if ddo_fecha_compra is present
	var invoicePeriod *numrotInvoicePeriod
	if item.DdoFechaCompra != nil && item.DdoFechaCompra.FechaCompra != "" {
		invoicePeriod = &numrotInvoicePeriod{
			StartDate:       item.DdoFechaCompra.FechaCompra,
			DescriptionCode: item.DdoFechaCompra.Codigo,
			Description:     "Por operación", // Default description for DS
		}
		// Use codigo as description if it's not "1" (which means "Por operación")
		if item.DdoFechaCompra.Codigo != "1" {
			invoicePeriod.Description = item.DdoFechaCompra.Codigo
		}
	}

	// For DS documents, add Note field with empty strings
	var note []string
	if documentType == "DS" {
		note = []string{"", ""}
	}

	invoiceLine := numrotInvoiceLine{
		ID:                       id,
		Note:                     note,
		InvoicedQuantity:         item.DdoCantidad,
		InvoicedQuantityUnitCode: unitCode,
		LineExtensionAmount:      item.DdoValorUnitario,
		CurrencyID:               doc.MonCodigo,
		InvoicePeriod:            invoicePeriod,
		Item: numrotItem{
			Description:                item.DdoDescripcionUno,
			SellersItemIdentification:  sellersID,
			StandardItemIdentification: standardID,
		},
		Price: numrotPrice{
			PriceAmount:          item.DdoValorUnitario,
			CurrencyID:           doc.MonCodigo,
			BaseQuantity:         item.DdoCantidad,
			BaseQuantityUnitCode: unitCode,
		},
	}

	// Include TaxTotal if there are taxes for this item
	if len(itemTaxes) > 0 {
		invoiceLine.TaxTotal = itemTaxes
	}

	return invoiceLine
}

// getTaxName returns the tax name for a given tax code.
func (c *Client) getTaxName(taxCode string) string {
	taxNames := map[string]string{
		"01": "IVA",
		"02": "Consumo",
		"03": "ICA",
		"04": "INC",
		"05": "ReteIVA",
		"06": "ReteFuente",
		"07": "ReteICA",
	}

	if name, exists := taxNames[taxCode]; exists {
		return name
	}
	return "Impuesto"
}

// getStringValue returns the string value from a pointer, or empty string if nil.
func getStringValue(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

// parseFloat parses a string to float64.
func parseFloat(s string) (float64, error) {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}

// formatFloat formats a float64 to string with 2 decimal places.
func formatFloat(f float64) string {
	return fmt.Sprintf("%.2f", f)
}

// normalizeMonetaryValue normalizes empty monetary values to "0.00".
// Returns "0.00" if input is empty, "0", or "0.00", otherwise returns the original value.
func normalizeMonetaryValue(value string) string {
	if value == "" || value == "0" || value == "0.00" {
		return "0.00"
	}
	return value
}

// mapUnitCode maps OpenETL unit codes to UBL standard codes.
func mapUnitCode(openETLCode string) string {
	// Map common Colombian unit codes to UBL codes
	unitCodeMap := map[string]string{
		"UN":  "94",  // Unit/Unidad
		"KG":  "KGM", // Kilogram
		"GR":  "GRM", // Gram
		"LT":  "LTR", // Liter
		"MT":  "MTR", // Meter
		"M2":  "MTK", // Square meter
		"M3":  "MTQ", // Cubic meter
		"HR":  "HUR", // Hour
		"MIN": "MIN", // Minute
		"DIA": "DAY", // Day
		"PAR": "PR",  // Pair
		"DOC": "DZN", // Dozen (12 units)
		"CM":  "CMT", // Centimeter
		"MM":  "MMT", // Millimeter
	}

	if ublCode, exists := unitCodeMap[openETLCode]; exists {
		return ublCode
	}
	// If not in map, return as is (might need validation)
	return openETLCode
}

// getStringOrDefault returns the pointer value or default if nil.
func getStringOrDefault(ptr *string, defaultVal string) string {
	if ptr == nil || *ptr == "" {
		return defaultVal
	}
	return *ptr
}

// buildCustomerPhysicalLocation builds location data for customer party from OpenETL document.
// Returns nil if no location data is provided.
func buildCustomerPhysicalLocation(doc invoice.OpenETLDocument) *numrotPhysicalLocation {
	// Default country values for Colombia
	countryCode := "CO"
	countryName := "Colombia"

	if doc.AdqPaisCodigo != nil && *doc.AdqPaisCodigo != "" {
		countryCode = *doc.AdqPaisCodigo
	}
	if doc.AdqPaisNombre != nil && *doc.AdqPaisNombre != "" {
		countryName = *doc.AdqPaisNombre
	}

	// Use postal code for PostalZone if available, otherwise fallback to municipality code
	postalZone := getStringValue(doc.AdqCpoCodigo)
	if postalZone == "" {
		postalZone = getStringValue(doc.AdqMunicipioCodigo)
	}

	// Build DIVIPOLA code (dep_codigo + mun_codigo) for ID field
	depCodigo := getStringValue(doc.AdqDepartamentoCodigo)
	munCodigo := getStringValue(doc.AdqMunicipioCodigo)
	divipolaCode := depCodigo + munCodigo
	// Fallback to municipality code if DIVIPOLA cannot be constructed
	if divipolaCode == "" {
		divipolaCode = munCodigo
	}

	// Build location with available data
	location := &numrotPhysicalLocation{
		ID:                   divipolaCode,
		CityName:             getStringValue(doc.AdqMunicipioNombre),
		PostalZone:           postalZone,
		CountrySubentity:     getStringValue(doc.AdqDepartamentoNombre),
		CountrySubentityCode: getStringValue(doc.AdqDepartamentoCodigo),
		Line:                 getStringValue(doc.AdqDireccion),
		IdentificationCode:   countryCode,
		Name:                 countryName,
	}

	return location
}

// buildSupplierPhysicalLocation builds location data for supplier party from OpenETL document.
// Returns nil if no location data is provided.
func buildSupplierPhysicalLocation(doc invoice.OpenETLDocument) *numrotPhysicalLocation {
	// Default country values for Colombia
	countryCode := "CO"
	countryName := "Colombia"

	// Build location with available data
	location := &numrotPhysicalLocation{
		ID:                   getStringValue(doc.OfeMunicipioCodigo),
		CityName:             getStringValue(doc.OfeMunicipioNombre),
		PostalZone:           getStringValue(doc.OfeMunicipioCodigo), // Use municipality code as postal zone
		CountrySubentity:     getStringValue(doc.OfeDepartamentoNombre),
		CountrySubentityCode: getStringValue(doc.OfeDepartamentoCodigo),
		Line:                 getStringValue(doc.OfeDireccion),
		IdentificationCode:   countryCode,
		Name:                 countryName,
	}

	return location
}

// buildDocumentSincURL builds the URL for Numrot documentSinc API endpoint.
// Format: /api/documentSinc/{nit}/{documento} or /documentSinc/{nit}/{documento} if baseURL already includes /api
// nit: NIT del OFE (sin DV)
// documento: Número completo del documento (prefijo + consecutivo)
func buildDocumentSincURL(baseURL, ofeIdentificacion, prefijo, consecutivo string) string {
	// Extract base NIT (without DV) for URL
	baseNIT, _ := parseNITWithDV(ofeIdentificacion)
	documento := prefijo + consecutivo

	// Check if baseURL already includes /api (e.g., https://numrotapiprueba.net/api)
	// If so, don't add /api again to avoid double /api/api
	baseURL = strings.TrimSuffix(baseURL, "/")
	if strings.HasSuffix(baseURL, "/api") {
		return fmt.Sprintf("%s/documentSinc/%s/%s", baseURL, baseNIT, documento)
	}

	return fmt.Sprintf("%s/api/documentSinc/%s/%s", baseURL, baseNIT, documento)
}

// parseNITWithDV parses a NIT that may include a check digit (DV).
// Returns the base NIT (without DV) and the DV digit.
// If no DV is provided, returns the original NIT as baseNIT and empty string for DV.
// Examples:
//   - "860011153-1" -> baseNIT: "860011153", dv: "1"
//   - "860011153" -> baseNIT: "860011153", dv: ""
func parseNITWithDV(nit string) (baseNIT string, dv string) {
	if nit == "" {
		return "", ""
	}

	// Split by "-" to separate NIT from DV
	parts := strings.Split(nit, "-")

	if len(parts) == 1 {
		// No DV provided, return NIT as is
		return nit, ""
	}

	if len(parts) == 2 {
		// DV provided
		baseNIT = strings.TrimSpace(parts[0])
		dv = strings.TrimSpace(parts[1])
		return baseNIT, dv
	}

	// Multiple dashes - take first part as NIT, last part as DV
	// This handles edge cases like "860011153-1-extra"
	baseNIT = strings.TrimSpace(parts[0])
	dv = strings.TrimSpace(parts[len(parts)-1])
	return baseNIT, dv
}
