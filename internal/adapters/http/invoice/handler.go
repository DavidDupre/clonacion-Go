package invoice

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"3tcapital/ms_facturacion_core/internal/adapters/invoice/numrot"
	appinvoice "3tcapital/ms_facturacion_core/internal/application/invoice"
	"3tcapital/ms_facturacion_core/internal/core/invoice"
	ctxutil "3tcapital/ms_facturacion_core/internal/infrastructure/context"
	httperrors "3tcapital/ms_facturacion_core/internal/infrastructure/http"
)

// Handler bridges HTTP traffic with the invoice application service.
type Handler struct {
	service      *appinvoice.Service
	numrotClient *numrot.Client // Optional: nil if numrot client not configured
	log          *slog.Logger
}

// NewHandler creates a new invoice HTTP handler.
// numrotClient is optional - if nil, DownloadPDF will return 503
func NewHandler(service *appinvoice.Service, numrotClient *numrot.Client, log *slog.Logger) *Handler {
	return &Handler{
		service:      service,
		numrotClient: numrotClient,
		log:          log,
	}
}

// GetDocumentsRequest represents the request body for getting documents.
type GetDocumentsRequest struct {
	CompanyNit  string `json:"CompanyNit"`
	InitialDate string `json:"InitialDate"`
	FinalDate   string `json:"FinalDate"`
}

// GetDocumentByNumberRequest represents the request body for getting a document by number.
type GetDocumentByNumberRequest struct {
	CompanyNit     string `json:"CompanyNit"`
	DocumentNumber string `json:"DocumentNumber"`
	SupplierNit    string `json:"SupplierNit"`
}

// GetDocumentsResponse represents the response format for documents.
type GetDocumentsResponse struct {
	Status  string             `json:"status"`
	Message string             `json:"message"`
	Total   int                `json:"total"`
	Data    []invoice.Document `json:"data"`
}

// GetDocuments handles POST /api/v1/facturas requests.
func (h *Handler) GetDocuments(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httperrors.WriteError(w, http.StatusMethodNotAllowed, "Método no permitido", []string{"Este endpoint solo acepta POST"}, nil)
		return
	}

	var reqBody GetDocumentsRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		httperrors.WriteError(w, http.StatusBadRequest, "Error de Validación", []string{"El cuerpo de la petición no es válido"}, nil)
		return
	}

	// Validate required fields
	if reqBody.CompanyNit == "" {
		httperrors.WriteError(w, http.StatusBadRequest, "Error de Validación", []string{"CompanyNit es requerido"}, nil)
		return
	}

	if reqBody.InitialDate == "" {
		httperrors.WriteError(w, http.StatusBadRequest, "Error de Validación", []string{"InitialDate es requerido"}, nil)
		return
	}

	if reqBody.FinalDate == "" {
		httperrors.WriteError(w, http.StatusBadRequest, "Error de Validación", []string{"FinalDate es requerido"}, nil)
		return
	}

	query := invoice.DocumentQuery{
		CompanyNit:  reqBody.CompanyNit,
		InitialDate: reqBody.InitialDate,
		FinalDate:   reqBody.FinalDate,
	}

	documents, err := h.service.GetDocuments(r.Context(), query)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	// Transform to response format
	response := GetDocumentsResponse{
		Status:  "200",
		Message: "Exitoso",
		Total:   len(documents),
		Data:    documents,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		// Log error but response already sent
		httperrors.WriteError(w, http.StatusInternalServerError, "Error Interno del Servidor", []string{"Ha ocurrido un error interno"}, nil)
	}
}

// GetDocumentByNumber handles POST /api/v1/facturas/by-number requests.
func (h *Handler) GetDocumentByNumber(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httperrors.WriteError(w, http.StatusMethodNotAllowed, "Método no permitido", []string{"Este endpoint solo acepta POST"}, nil)
		return
	}

	var reqBody GetDocumentByNumberRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		httperrors.WriteError(w, http.StatusBadRequest, "Error de Validación", []string{"El cuerpo de la petición no es válido"}, nil)
		return
	}

	// Validate required fields
	if reqBody.CompanyNit == "" {
		httperrors.WriteError(w, http.StatusBadRequest, "Error de Validación", []string{"CompanyNit es requerido"}, nil)
		return
	}

	if reqBody.DocumentNumber == "" {
		httperrors.WriteError(w, http.StatusBadRequest, "Error de Validación", []string{"DocumentNumber es requerido"}, nil)
		return
	}

	if reqBody.SupplierNit == "" {
		httperrors.WriteError(w, http.StatusBadRequest, "Error de Validación", []string{"SupplierNit es requerido"}, nil)
		return
	}

	query := invoice.DocumentByNumberQuery{
		CompanyNit:     reqBody.CompanyNit,
		DocumentNumber: reqBody.DocumentNumber,
		SupplierNit:    reqBody.SupplierNit,
	}

	documents, err := h.service.GetDocumentByNumber(r.Context(), query)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	// Transform to response format (same as GetDocuments)
	response := GetDocumentsResponse{
		Status:  "200",
		Message: "Exitoso",
		Total:   len(documents),
		Data:    documents,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		// Log error but response already sent
		httperrors.WriteError(w, http.StatusInternalServerError, "Error Interno del Servidor", []string{"Ha ocurrido un error interno"}, nil)
	}
}

// GetReceivedDocuments handles POST /api/v1/facturas/received requests.
func (h *Handler) GetReceivedDocuments(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httperrors.WriteError(w, http.StatusMethodNotAllowed, "Método no permitido", []string{"Este endpoint solo acepta POST"}, nil)
		return
	}

	var reqBody GetDocumentsRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		httperrors.WriteError(w, http.StatusBadRequest, "Error de Validación", []string{"El cuerpo de la petición no es válido"}, nil)
		return
	}

	// Validate required fields
	if reqBody.CompanyNit == "" {
		httperrors.WriteError(w, http.StatusBadRequest, "Error de Validación", []string{"CompanyNit es requerido"}, nil)
		return
	}

	if reqBody.InitialDate == "" {
		httperrors.WriteError(w, http.StatusBadRequest, "Error de Validación", []string{"InitialDate es requerido"}, nil)
		return
	}

	if reqBody.FinalDate == "" {
		httperrors.WriteError(w, http.StatusBadRequest, "Error de Validación", []string{"FinalDate es requerido"}, nil)
		return
	}

	query := invoice.DocumentQuery{
		CompanyNit:  reqBody.CompanyNit,
		InitialDate: reqBody.InitialDate,
		FinalDate:   reqBody.FinalDate,
	}

	documents, err := h.service.GetReceivedDocuments(r.Context(), query)
	if err != nil {
		h.handleError(w, r, err)
		return
	}

	// and call the DownloadPDF handler with JSON body { "id": "<cufe>" }.
	if r.URL.Query().Get("download") == "1" {
		if len(documents) == 0 {
			httperrors.WriteError(w, http.StatusBadRequest, "Error de Validación", []string{"No hay documentos para descargar"}, h.log)
			return
		}
		cufe := documents[0].CUFE
		r2, err := http.NewRequest(http.MethodGet, fmt.Sprintf("/facturas/download?cufe=%s", cufe), nil)
		if err != nil {
			h.log.Error("failed to build internal download request", "error", err)
			httperrors.WriteError(w, http.StatusInternalServerError, "Error Interno del Servidor", []string{"Error al construir petición interna"}, h.log)
			return
		}
		r2 = r2.WithContext(r.Context())
		// No body for GET; ensure headers are minimal

		// Call the local handler which will write the response to the original writer
		h.DownloadPDF(w, r2)
		return
	}

	// Transform to response format (same as GetDocuments)
	response := GetDocumentsResponse{
		Status:  "200",
		Message: "Exitoso",
		Total:   len(documents),
		Data:    documents,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		// Log error but response already sent
		httperrors.WriteError(w, http.StatusInternalServerError, "Error Interno del Servidor", []string{"Ha ocurrido un error interno"}, nil)
	}
}

// DownloadPDF handles requests to download a generated PDF for a document.
func (h *Handler) DownloadPDF(w http.ResponseWriter, r *http.Request) {
	var id string

	switch r.Method {
	case http.MethodPost:
		var body struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			httperrors.WriteError(w, http.StatusBadRequest, "Error de Validación", []string{"El cuerpo de la petición no es válido"}, h.log)
			return
		}
		id = body.ID
	case http.MethodGet:
		id = r.URL.Query().Get("cufe")
	default:
		httperrors.WriteError(w, http.StatusMethodNotAllowed, "Método no permitido", []string{"Este endpoint solo acepta GET o POST"}, h.log)
		return
	}

	if id == "" {
		httperrors.WriteError(w, http.StatusBadRequest, "Error de Validación", []string{"id (cufe) es requerido"}, h.log)
		return
	}

	fecha := r.URL.Query().Get("fecha")
	nit := r.URL.Query().Get("nit")

	if h.service != nil {
		var query invoice.DocumentQuery

		if fecha != "" {
			query = invoice.DocumentQuery{
				CompanyNit:  nit,
				InitialDate: fecha,
				FinalDate:   fecha,
			}
			h.log.Debug("querying received documents for download with fecha", "cufe", id, "fecha", fecha, "nit", nit)
		} else {
			query = invoice.DocumentQuery{
				CompanyNit: nit,
			}
			h.log.Debug("querying received documents for download without fecha", "cufe", id, "nit", nit)
		}

		docs, err := h.service.GetReceivedDocuments(r.Context(), query)
		if err != nil {
			statusCode := http.StatusBadGateway
			if strings.Contains(strings.ToLower(err.Error()), "initial date is required") {
				statusCode = http.StatusNotFound
				h.log.Warn("GetReceivedDocuments failed while attempting download", "error", err)
			} else {
				h.log.Error("GetReceivedDocuments failed while attempting download", "error", err)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(statusCode)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"mensaje": "Error en la peticion",
				"status":  statusCode,
				"urlPDF":  nil,
				"urlXML":  nil,
			})
			return
		}

		h.log.Debug("received documents from provider", "count", len(docs))
		for _, d := range docs {
			if d.CUFE == id {
				respBody := map[string]interface{}{
					"mensaje": "Exitoso",
					"status":  http.StatusOK,
					"urlPDF":  d.UrlPDF,
					"urlXML":  d.UrlXML,
					"cufe":    d.CUFE,
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(respBody)
				return
			}
		}

		h.log.Warn("document not found in received documents",
			"cufe", id,
			"fecha", fecha,
			"nit", nit,
			"docs_count", len(docs))
		if len(docs) > 0 {
			maxLog := 5
			if len(docs) < maxLog {
				maxLog = len(docs)
			}
			cufes := make([]string, 0, maxLog)
			for i := 0; i < maxLog; i++ {
				cufes = append(cufes, docs[i].CUFE)
			}
			h.log.Debug("received CUFEs", "cufes", cufes)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"mensaje": "Error en la peticion",
			"status":  http.StatusNotFound,
			"urlPDF":  nil,
			"urlXML":  nil,
		})
		return
	}

	// If service is nil or did not return, respond with not found
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"mensaje": "Error en la peticion",
		"status":  http.StatusNotFound,
		"urlPDF":  nil,
		"urlXML":  nil,
	})
}

// DownloadPDFFromNumrot handles requests to download a PDF document from Numrot's SearchEstadosDIAN API.
// This endpoint accepts form data with document identification parameters and returns a base64-encoded PDF.
func (h *Handler) DownloadPDFFromNumrot(w http.ResponseWriter, r *http.Request) {
	// Parse form data
	if err := r.ParseForm(); err != nil {
		httperrors.WriteError(w, http.StatusBadRequest, "Error de Validación", []string{"Error al parsear los datos del formulario"}, h.log)
		return
	}

	// Extract required form fields
	ofeIdentificacion := r.FormValue("ofe_identificacion")
	prefijo := r.FormValue("prefijo")
	consecutivo := r.FormValue("consecutivo")
	// tipo_documento and resultado are optional

	if ofeIdentificacion == "" {
		httperrors.WriteError(w, http.StatusBadRequest, "Error de Validación", []string{"ofe_identificacion es requerido"}, h.log)
		return
	}

	if prefijo == "" {
		httperrors.WriteError(w, http.StatusBadRequest, "Error de Validación", []string{"prefijo es requerido"}, h.log)
		return
	}

	if consecutivo == "" {
		httperrors.WriteError(w, http.StatusBadRequest, "Error de Validación", []string{"consecutivo es requerido"}, h.log)
		return
	}

	// Check if numrot client is configured
	if h.numrotClient == nil {
		httperrors.WriteError(w, http.StatusServiceUnavailable, "Servicio No Disponible", []string{"El cliente de Numrot no está configurado"}, h.log)
		return
	}

	// Construct document number: prefijo + consecutivo
	documento := strings.TrimSpace(prefijo) + strings.TrimSpace(consecutivo)

	h.log.Debug("Downloading PDF from Numrot", "ofe", ofeIdentificacion, "documento", documento)

	// Call SearchEstadosDIAN with includePdf=true (already included in the method)
	searchResp, err := h.numrotClient.SearchEstadosDIAN(r.Context(), ofeIdentificacion, documento)
	if err != nil {
		h.log.Error("Failed to get document from Numrot", "error", err, "ofe", ofeIdentificacion, "documento", documento)

		// Check if it's a not found error
		if strings.Contains(err.Error(), "document not found") {
			httperrors.WriteError(w, http.StatusNotFound, "Documento No Encontrado", []string{"No se encontró el documento en Numrot"}, h.log)
			return
		}

		httperrors.WriteError(w, http.StatusBadGateway, "Error del Proveedor", []string{"Error al consultar documento en Numrot"}, h.log)
		return
	}

	// Extract PDF from response
	if searchResp.Document == "" {
		h.log.Warn("PDF not found in document response", "ofe", ofeIdentificacion, "documento", documento)
		httperrors.WriteError(w, http.StatusNotFound, "PDF No Encontrado", []string{"El documento no contiene un PDF"}, h.log)
		return
	}

	// Return PDF in base64 format
	response := map[string]interface{}{
		"data": map[string]string{
			"pdf": searchResp.Document,
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.log.Error("Failed to encode response", "error", err)
	}
}

// handleError maps domain errors to appropriate HTTP status codes.
func (h *Handler) handleError(w http.ResponseWriter, r *http.Request, err error) {
	// Check error type and return appropriate status
	errorMsg := err.Error()

	// Get correlation ID from context for better traceability
	correlationID := ctxutil.GetCorrelationID(r.Context())

	// Determine status code and log level
	var statusCode int
	var logLevel string

	switch {
	// Validation errors from service - basic required fields
	case contains(errorMsg, "company nit is required") || contains(errorMsg, "initial date is required") || contains(errorMsg, "final date is required") || contains(errorMsg, "document number is required") || contains(errorMsg, "supplier nit is required"):
		statusCode = http.StatusBadRequest
		logLevel = "warn"
		httperrors.WriteError(w, statusCode, "Error de Validación", []string{errorMsg}, nil)
	// Validation errors from service - format validation
	case contains(errorMsg, "invalid company nit format") || contains(errorMsg, "invalid initial date format") || contains(errorMsg, "invalid final date format") || contains(errorMsg, "initial date must be before") || contains(errorMsg, "invalid supplier nit format"):
		statusCode = http.StatusBadRequest
		logLevel = "warn"
		httperrors.WriteError(w, statusCode, "Error de Validación", []string{errorMsg}, nil)
	// Document registration validation errors
	case contains(errorMsg, "no documents provided") || contains(errorMsg, "only one document type") || contains(errorMsg, "is required") || contains(errorMsg, "invalid cdo_fecha format") || contains(errorMsg, "invalid cdo_hora format") || contains(errorMsg, "at least one item is required") || contains(errorMsg, "does not match document type") || contains(errorMsg, "FAD09e compliance") || contains(errorMsg, "must be today's date") || contains(errorMsg, "cdo_fecha must be today"):
		statusCode = http.StatusBadRequest
		logLevel = "warn"
		httperrors.WriteError(w, statusCode, "Error de Validación", []string{errorMsg}, nil)
	// Failed documents from provider
	case contains(errorMsg, "documentos fallidos"):
		statusCode = http.StatusBadRequest
		logLevel = "warn"
		httperrors.WriteError(w, statusCode, "Error de Validación", []string{errorMsg}, nil)
	// Provider configuration errors
	case contains(errorMsg, "key and secret are required"):
		statusCode = http.StatusBadGateway
		logLevel = "error"
		httperrors.WriteError(w, statusCode, "Error de Configuraci?n", []string{"Error de configuraci?n del proveedor"}, nil)
	// Authentication errors
	case contains(errorMsg, "authentication failed") || contains(errorMsg, "numrot authentication failed") || contains(errorMsg, "get authentication token"):
		statusCode = http.StatusBadGateway
		logLevel = "error"
		httperrors.WriteError(w, statusCode, "Error de Autenticación", []string{"Error de autenticación con el proveedor"}, nil)
	// Provider communication errors
	case contains(errorMsg, "unexpected status code") || contains(errorMsg, "execute request") || contains(errorMsg, "read response body"):
		statusCode = http.StatusBadGateway
		logLevel = "error"
		httperrors.WriteError(w, statusCode, "Error del Proveedor", []string{"Servicio del proveedor no disponible"}, nil)
	case contains(errorMsg, "numrot API error"):
		// Numrot API returned an error code
		statusCode = http.StatusBadGateway
		logLevel = "error"
		httperrors.WriteError(w, statusCode, "Error del Proveedor", []string{errorMsg}, nil)
	case contains(errorMsg, "unmarshal response") || contains(errorMsg, "marshal request"):
		statusCode = http.StatusBadGateway
		logLevel = "error"
		httperrors.WriteError(w, statusCode, "Error del Proveedor", []string{"Error en el formato de respuesta del proveedor"}, nil)
	case contains(errorMsg, "provider error"):
		// Generic provider error
		statusCode = http.StatusBadGateway
		logLevel = "error"
		httperrors.WriteError(w, statusCode, "Error del Proveedor", []string{"Servicio del proveedor no disponible"}, nil)
	default:
		statusCode = http.StatusInternalServerError
		logLevel = "error"
		httperrors.WriteError(w, statusCode, "Error Interno del Servidor", []string{"Ha ocurrido un error interno"}, nil)
	}

	// Build log attributes with context information
	logAttrs := []any{
		"error", err,
		"error_message", errorMsg,
		"status_code", statusCode,
		"method", r.Method,
		"path", r.URL.Path,
	}

	if correlationID != "" {
		logAttrs = append(logAttrs, "correlation_id", correlationID)
	}

	// Log the error with appropriate level
	if logLevel == "error" {
		h.log.Error("Request failed", logAttrs...)
	} else {
		h.log.Warn("Request failed", logAttrs...)
	}
}

// contains checks if a string contains a substring (case-insensitive).
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// RegisterDocument handles POST /api/v1/registrar-documentos requests.
func (h *Handler) RegisterDocument(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httperrors.WriteError(w, http.StatusMethodNotAllowed, "Método no permitido", []string{"Este endpoint solo acepta POST"}, nil)
		return
	}

	var reqBody invoice.DocumentRegistrationRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		httperrors.WriteError(w, http.StatusBadRequest, "Error de Validación", []string{"El cuerpo de la petición no es válido"}, nil)
		return
	}

	// Count total documents
	totalDocs := len(reqBody.Documentos.FC) + len(reqBody.Documentos.NC) + len(reqBody.Documentos.ND) + len(reqBody.Documentos.DS)

	// Use streaming for all document registrations
	// This enables partial processing (each document validated/processed individually)
	// and prevents timeouts by sending results progressively
	// If ResponseWriter doesn't support flushing, it automatically falls back to regular processing
	h.registerDocumentStreaming(w, r, reqBody, totalDocs)
}

// registerDocumentStreaming handles massive document registration with streaming response.
// This prevents timeouts by sending results progressively as documents are processed.
func (h *Handler) registerDocumentStreaming(w http.ResponseWriter, r *http.Request, reqBody invoice.DocumentRegistrationRequest, totalDocs int) {
	// Check if response writer supports flushing
	flusher, ok := w.(http.Flusher)
	if !ok {
		// Fallback: process documents individually (without streaming)
		// This enables partial processing even when ResponseWriter doesn't support flushing
		h.log.Warn("ResponseWriter does not support flushing, processing documents individually")

		// Determine document type and get documents
		var documents []invoice.OpenETLDocument
		var documentType string

		if len(reqBody.Documentos.FC) > 0 {
			documents = reqBody.Documentos.FC
			documentType = "FC"
		} else if len(reqBody.Documentos.NC) > 0 {
			documents = reqBody.Documentos.NC
			documentType = "NC"
		} else if len(reqBody.Documentos.ND) > 0 {
			documents = reqBody.Documentos.ND
			documentType = "ND"
		} else if len(reqBody.Documentos.DS) > 0 {
			documents = reqBody.Documentos.DS
			documentType = "DS"
		}

		// Process documents concurrently using worker pool pattern
		// Determine number of workers (default 10, max number of documents)
		numWorkers := 10
		if len(documents) < numWorkers {
			numWorkers = len(documents)
		}

		// Create work channel
		type workItem struct {
			index    int
			document invoice.OpenETLDocument
		}
		workChan := make(chan workItem, len(documents))

		// Create result channel
		type docResult struct {
			processed []invoice.ProcessedDocument
			failed    []invoice.FailedDocument
			lote      string
			err       error
		}
		resultChan := make(chan docResult, len(documents))

		// Send all documents to work channel
		for i, doc := range documents {
			workChan <- workItem{index: i, document: doc}
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
						h.log.Error("Worker panic recovered in fallback mode",
							"worker_id", workerID,
							"panic", r)
					}
				}()

				for work := range workChan {
					doc := work.document

					// Create single document request
					singleDocReq := invoice.DocumentRegistrationRequest{
						Documentos: invoice.DocumentsByType{},
					}
					switch documentType {
					case "FC":
						singleDocReq.Documentos.FC = []invoice.OpenETLDocument{doc}
					case "NC":
						singleDocReq.Documentos.NC = []invoice.OpenETLDocument{doc}
					case "ND":
						singleDocReq.Documentos.ND = []invoice.OpenETLDocument{doc}
					case "DS":
						singleDocReq.Documentos.DS = []invoice.OpenETLDocument{doc}
					}

					// Process single document
					response, err := h.service.RegisterDocument(r.Context(), singleDocReq)

					// Build result
					result := docResult{}
					if err != nil {
						// Document failed with error
						now := time.Now()
						result.failed = []invoice.FailedDocument{
							{
								Documento:          documentType,
								Consecutivo:        doc.CdoConsecutivo,
								Prefijo:            doc.RfaPrefijo,
								Errors:             []string{err.Error()},
								FechaProcesamiento: now.Format("2006-01-02"),
								HoraProcesamiento:  now.Format("15:04:05"),
							},
						}
						result.err = err
					} else if response != nil {
						// Collect processed and failed documents from response
						result.processed = response.DocumentosProcesados
						result.failed = response.DocumentosFallidos
						result.lote = response.Lote
					}

					// Send result to channel
					resultChan <- result
				}
			}(w)
		}

		// Wait for all workers to complete and close result channel
		go func() {
			wg.Wait()
			close(resultChan)
		}()

		// Collect all results
		allProcessed := make([]invoice.ProcessedDocument, 0)
		allFailed := make([]invoice.FailedDocument, 0)
		var firstLote string

		for result := range resultChan {
			allProcessed = append(allProcessed, result.processed...)
			allFailed = append(allFailed, result.failed...)

			// Capture first lote
			if firstLote == "" && result.lote != "" {
				firstLote = result.lote
			}
		}

		// Generate lote if not set
		if firstLote == "" {
			firstLote = fmt.Sprintf("lote-%d", time.Now().Unix())
		}

		// Ensure slices are never nil (use empty slice instead)
		if allProcessed == nil {
			allProcessed = make([]invoice.ProcessedDocument, 0)
		}
		if allFailed == nil {
			allFailed = make([]invoice.FailedDocument, 0)
		}

		// Build consolidated response
		consolidatedResponse := &invoice.DocumentRegistrationResponse{
			Lote:                 firstLote,
			DocumentosProcesados: allProcessed,
			DocumentosFallidos:   allFailed,
		}

		// Set message based on results
		if len(allProcessed) > 0 && len(allFailed) == 0 {
			consolidatedResponse.Message = "Documentos procesados exitosamente"
		} else if len(allFailed) > 0 && len(allProcessed) == 0 {
			consolidatedResponse.Message = "Error al procesar documentos"
		} else if len(allProcessed) > 0 && len(allFailed) > 0 {
			consolidatedResponse.Message = "Algunos documentos fueron procesados, otros fallaron"
		} else {
			consolidatedResponse.Message = "Procesamiento completado"
		}

		// Determine HTTP status
		statusCode := http.StatusOK
		if len(allFailed) > 0 {
			statusCode = http.StatusBadRequest
			h.log.Warn("Documents failed during registration",
				"failed_count", len(allFailed),
				"processed_count", len(allProcessed))
		} else if len(allProcessed) > 0 {
			h.log.Info("Documents processed successfully",
				"processed_count", len(allProcessed))
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(consolidatedResponse)
		return
	}

	// Set headers for streaming response
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.WriteHeader(http.StatusOK)

	// Start JSON array
	w.Write([]byte("{\n  \"streaming\": true,\n  \"total_documents\": "))
	json.NewEncoder(w).Encode(totalDocs)
	w.Write([]byte(",\n  \"results\": [\n"))
	flusher.Flush()

	// Determine document type and get documents
	var documents []invoice.OpenETLDocument
	var documentType string

	if len(reqBody.Documentos.FC) > 0 {
		documents = reqBody.Documentos.FC
		documentType = "FC"
	} else if len(reqBody.Documentos.NC) > 0 {
		documents = reqBody.Documentos.NC
		documentType = "NC"
	} else if len(reqBody.Documentos.ND) > 0 {
		documents = reqBody.Documentos.ND
		documentType = "ND"
	} else if len(reqBody.Documentos.DS) > 0 {
		documents = reqBody.Documentos.DS
		documentType = "DS"
	}

	// Process documents in batches and stream results
	// We'll process them through the service but need to handle streaming differently
	// For now, we'll call the service for each document individually to enable streaming
	// This is not optimal but maintains clean architecture separation

	firstResult := true
	processedCount := 0
	failedCount := 0
	var firstLote string

	// Process documents individually to enable streaming
	for i, doc := range documents {
		// Create single document request
		singleDocReq := invoice.DocumentRegistrationRequest{
			Documentos: invoice.DocumentsByType{},
		}
		switch documentType {
		case "FC":
			singleDocReq.Documentos.FC = []invoice.OpenETLDocument{doc}
		case "NC":
			singleDocReq.Documentos.NC = []invoice.OpenETLDocument{doc}
		case "ND":
			singleDocReq.Documentos.ND = []invoice.OpenETLDocument{doc}
		case "DS":
			singleDocReq.Documentos.DS = []invoice.OpenETLDocument{doc}
		}

		// Process single document
		response, err := h.service.RegisterDocument(r.Context(), singleDocReq)

		// Create result object for this document
		result := map[string]interface{}{
			"index":       i,
			"prefijo":     doc.RfaPrefijo,
			"consecutivo": doc.CdoConsecutivo,
		}

		if err != nil {
			// Document failed
			failedCount++
			result["status"] = "failed"
			result["error"] = err.Error()
		} else if response != nil {
			// Check if document was processed or failed
			if len(response.DocumentosProcesados) > 0 {
				processedCount++
				result["status"] = "processed"
				result["fecha_procesamiento"] = response.DocumentosProcesados[0].FechaProcesamiento
				result["hora_procesamiento"] = response.DocumentosProcesados[0].HoraProcesamiento
				if firstLote == "" && response.Lote != "" {
					firstLote = response.Lote
				}
			} else if len(response.DocumentosFallidos) > 0 {
				failedCount++
				result["status"] = "failed"
				if len(response.DocumentosFallidos) > 0 {
					result["errors"] = response.DocumentosFallidos[0].Errors
				}
			}
		}

		// Write comma separator if not first result
		if !firstResult {
			w.Write([]byte(",\n"))
		}
		firstResult = false

		// Write result as JSON
		if err := json.NewEncoder(w).Encode(result); err != nil {
			// Connection may have been closed by client
			h.log.Warn("Failed to encode streaming result - connection may be closed",
				"error", err,
				"index", i,
				"processed_so_far", processedCount,
				"failed_so_far", failedCount)
			// Stop processing if connection is closed
			return
		}

		// Flush to send result immediately
		flusher.Flush()

		// Check if context was cancelled
		select {
		case <-r.Context().Done():
			h.log.Warn("Request context cancelled during streaming", "processed", processedCount, "failed", failedCount)
			return
		default:
			// Continue processing
		}
	}

	// Close JSON array and add summary
	w.Write([]byte("\n  ],\n  \"summary\": {\n"))

	// Generate lote if not set
	if firstLote == "" {
		firstLote = fmt.Sprintf("lote-%d", time.Now().Unix())
	}

	summary := map[string]interface{}{
		"total":     totalDocs,
		"processed": processedCount,
		"failed":    failedCount,
		"lote":      firstLote,
		"message":   h.generateSummaryMessage(processedCount, failedCount),
	}

	summaryJSON, _ := json.MarshalIndent(summary, "    ", "  ")
	if _, err := w.Write(summaryJSON); err != nil {
		h.log.Warn("Failed to write summary - connection may be closed", "error", err)
		return
	}
	if _, err := w.Write([]byte("\n  }\n}")); err != nil {
		h.log.Warn("Failed to write closing JSON - connection may be closed", "error", err)
		return
	}

	// Final flush
	flusher.Flush()

	h.log.Info("Streaming document registration completed",
		"total", totalDocs,
		"processed", processedCount,
		"failed", failedCount)
}

// generateSummaryMessage generates a summary message based on processing results.
func (h *Handler) generateSummaryMessage(processed, failed int) string {
	if processed > 0 && failed == 0 {
		return "Documentos procesados exitosamente"
	} else if failed > 0 && processed == 0 {
		return "Error al procesar documentos"
	} else if processed > 0 && failed > 0 {
		return "Algunos documentos fueron procesados, otros fallaron"
	}
	return "Procesamiento completado"
}
