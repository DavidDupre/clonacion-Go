package reception

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"3tcapital/ms_facturacion_core/internal/adapters/invoice/numrot"
	"3tcapital/ms_facturacion_core/internal/core/event"
	"3tcapital/ms_facturacion_core/internal/core/invoice"
	httperrors "3tcapital/ms_facturacion_core/internal/infrastructure/http"
)

// Handler provides legacy reception endpoints.
type Handler struct {
	numrotClient            *numrot.Client
	emisorNit               string
	generatorNombre         string
	generatorApellido       string
	generatorIdentificacion string
	log                     *slog.Logger
}

// NewHandler creates a new reception handler with dependencies.
// numrotClient can be nil - if nil, consulta-documentos will return 503
func NewHandler(
	numrotClient *numrot.Client,
	emisorNit string,
	generatorNombre string,
	generatorApellido string,
	generatorIdentificacion string,
	log *slog.Logger,
) *Handler {
	return &Handler{
		numrotClient:            numrotClient,
		emisorNit:               emisorNit,
		generatorNombre:         generatorNombre,
		generatorApellido:       generatorApellido,
		generatorIdentificacion: generatorIdentificacion,
		log:                     log,
	}
}

// LegacyRegistrarEventoPayload represents the legacy payload structure.
type LegacyRegistrarEventoPayload struct {
	Evento     string `json:"evento"`
	Documentos []struct {
		CdoCufe        string `json:"cdo_cufe"`
		CdoFecha       string `json:"cdo_fecha"`
		CdoObservacion string `json:"cdo_observacion"`
		CreCodigo      string `json:"cre_codigo"`
	} `json:"documentos"`
}

// RegistrarEvento handles legacy POST /api/recepcion/documentos/registrar-evento
func (h *Handler) RegistrarEvento(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httperrors.WriteError(w, http.StatusMethodNotAllowed, "Método no permitido", []string{"Este endpoint solo acepta POST"}, h.log)
		return
	}

	// 1. Parse and validate request
	var payload LegacyRegistrarEventoPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		httperrors.WriteError(w, http.StatusBadRequest, "Error al procesar el evento: Datos inválidos", []string{"El cuerpo de la petición no es válido"}, h.log)
		return
	}

	// 2. Basic validation
	if payload.Evento == "" || len(payload.Documentos) == 0 {
		httperrors.WriteError(w, http.StatusBadRequest, "Error al procesar el evento: Datos inválidos", []string{"evento y documentos son requeridos"}, h.log)
		return
	}

	// 3. Validate Numrot client is configured
	if h.numrotClient == nil {
		h.log.Error("Numrot client not configured for registrar-evento")
		httperrors.WriteError(w, http.StatusServiceUnavailable, "Servicio Open temporalmente no disponible",
			[]string{"Servicio de registro de eventos no configurado"}, h.log)
		return
	}

	// 4. Validate generator configuration
	if h.generatorNombre == "" || h.generatorApellido == "" || h.generatorIdentificacion == "" {
		h.log.Error("Generator information not configured",
			"nombre", h.generatorNombre,
			"apellido", h.generatorApellido,
			"identificacion", h.generatorIdentificacion)
		httperrors.WriteError(w, http.StatusInternalServerError, "Error interno del servidor al procesar el evento",
			[]string{"Información del generador no configurada"}, h.log)
		return
	}

	// 5. Validate and parse event type
	eventType := event.EventType(strings.ToUpper(payload.Evento))
	if !event.ValidateEventType(eventType) {
		httperrors.WriteError(w, http.StatusBadRequest, "Error al procesar el evento: Datos inválidos",
			[]string{fmt.Sprintf("Tipo de evento inválido: %s. Valores válidos: ACUSE, RECIBOBIEN, ACEPTACION, RECLAMO", payload.Evento)}, h.log)
		return
	}

	h.log.Info("Processing event registration request",
		"event_type", eventType,
		"document_count", len(payload.Documentos))

	// 6. Process each document
	results := make([]documentProcessingResult, 0, len(payload.Documentos))
	for _, doc := range payload.Documentos {
		result := h.processDocumentEvent(r.Context(), doc, eventType)
		results = append(results, result)
	}

	h.log.Info("Event registration completed",
		"total", len(payload.Documentos))

	// 7. Build response messages
	exitosos, fallidos := h.buildResponseMessages(results)

	// 8. Determine HTTP status code
	httpStatus := h.determineHTTPStatus(results)

	// 9. Build response (always initialize arrays to avoid null in JSON)
	response := RegistrarEventoResponse{
		Message:  "Solicitud Procesada",
		Exitosos: exitosos,
		Fallidos: fallidos,
	}

	// Ensure arrays are never null
	if response.Exitosos == nil {
		response.Exitosos = make([]string, 0)
	}
	if response.Fallidos == nil {
		response.Fallidos = make([]string, 0)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.log.Error("Failed to encode response", "error", err)
	}
}

// documentProcessingResult holds detailed information about document processing
type documentProcessingResult struct {
	Cufe            string
	DocumentNumber  string
	Status          string // "success", "error", "partial", "duplicate"
	EventResult     *invoice.EventRegistrationResult
	ErrorDetails    *EventErrorDetails
	IsDuplicate     bool
	IsDocumentNotFound bool
	IsInvalidEvent  bool
}

// processDocumentEvent processes event registration for a single document
func (h *Handler) processDocumentEvent(
	ctx context.Context,
	doc struct {
		CdoCufe        string `json:"cdo_cufe"`
		CdoFecha       string `json:"cdo_fecha"`
		CdoObservacion string `json:"cdo_observacion"`
		CreCodigo      string `json:"cre_codigo"`
	},
	eventType event.EventType,
) documentProcessingResult {
	cufe := doc.CdoCufe

	h.log.Debug("Processing document event", "cufe", cufe, "event_type", eventType)

	// 1. Validate CUFE is present
	if cufe == "" {
		return documentProcessingResult{
			Cufe:   cufe,
			Status: "error",
			ErrorDetails: &EventErrorDetails{
				Stage:       "validation",
				Description: "El campo cdo_cufe está vacío",
			},
		}
	}

	// 2. Query DocumentInfo to get Emisor data
	docInfo, err := h.numrotClient.GetDocumentInfo(ctx, h.emisorNit, cufe)
	if err != nil {
		h.log.Error("Failed to get document info", "error", err, "cufe", cufe)

		isDocumentNotFound := strings.Contains(err.Error(), "document not found")
		errorCode := ""
		if isDocumentNotFound {
			errorCode = "DOCUMENT_NOT_FOUND"
		} else if strings.Contains(err.Error(), "authentication failed") {
			errorCode = "AUTH_FAILED"
		}

		return documentProcessingResult{
			Cufe:                cufe,
			Status:              "error",
			IsDocumentNotFound:  isDocumentNotFound,
			ErrorDetails: &EventErrorDetails{
				Stage:       "document_info",
				ErrorCode:   errorCode,
				Description: err.Error(),
			},
		}
	}

	// 3. Validate we have document info
	if len(docInfo.DocumentInfo) == 0 {
		return documentProcessingResult{
			Cufe:   cufe,
			Status: "error",
			ErrorDetails: &EventErrorDetails{
				Stage:       "document_info",
				ErrorCode:   "EMPTY_RESPONSE",
				Description: "La respuesta del proveedor no contiene información del documento",
			},
		}
	}

	// 4. Extract Emisor data
	docData := docInfo.DocumentInfo[0]
	emisorNit := docData.Emisor.NumeroDoc
	razonSocial := docData.Emisor.Nombre

	if emisorNit == "" || razonSocial == "" {
		return documentProcessingResult{
			Cufe:   cufe,
			Status: "error",
			ErrorDetails: &EventErrorDetails{
				Stage:       "document_info",
				ErrorCode:   "INCOMPLETE_EMISOR_DATA",
				Description: fmt.Sprintf("EmisorNit: '%s', RazonSocial: '%s'", emisorNit, razonSocial),
			},
		}
	}

	h.log.Debug("Extracted emisor data",
		"cufe", cufe,
		"emisor_nit", emisorNit,
		"razon_social", razonSocial)

	// 5. Build document number from prefix + consecutive
	documentNumber := docData.NumeroDocumento.Serie + docData.NumeroDocumento.Folio
	if documentNumber == "" {
		documentNumber = cufe // Fallback to CUFE if document number can't be constructed
	}

	// 6. Build event domain object
	evt := event.Event{
		EventType:               eventType,
		DocumentNumber:          documentNumber,
		NombreGenerador:         h.generatorNombre,
		ApellidoGenerador:       h.generatorApellido,
		IdentificacionGenerador: h.generatorIdentificacion,
		EventGenerationDate:     time.Now(),
	}

	// 7. Handle rejection code for RECLAMO events
	if eventType == event.EventTypeReclamo {
		if doc.CreCodigo != "" {
			rejectionCode := event.RejectionCode(doc.CreCodigo)
			if !event.ValidateRejectionCode(rejectionCode) {
				return documentProcessingResult{
					Cufe:   cufe,
					Status: "error",
					ErrorDetails: &EventErrorDetails{
						Stage:       "validation",
						ErrorCode:   "INVALID_REJECTION_CODE",
						Description: fmt.Sprintf("El código de rechazo '%s' no es válido. Valores válidos: 01, 02, 03, 04", doc.CreCodigo),
					},
				}
			}
			evt.RejectionCode = &rejectionCode
		} else {
			return documentProcessingResult{
				Cufe:   cufe,
				Status: "error",
				ErrorDetails: &EventErrorDetails{
					Stage:       "validation",
					ErrorCode:   "MISSING_REJECTION_CODE",
					Description: "El campo cre_codigo es obligatorio para eventos tipo RECLAMO",
				},
			}
		}
	}

	// 8. Validate event
	if err := evt.Validate(); err != nil {
		return documentProcessingResult{
			Cufe:   cufe,
			Status: "error",
			ErrorDetails: &EventErrorDetails{
				Stage:       "validation",
				ErrorCode:   "EVENT_VALIDATION_FAILED",
				Description: err.Error(),
			},
		}
	}

	// 9. Register event with Numrot
	eventResult, err := h.numrotClient.RegisterEvent(ctx, evt, emisorNit, razonSocial)
	if err != nil {
		h.log.Error("Failed to register event", "error", err, "cufe", cufe)

		isDocumentNotFound := strings.Contains(err.Error(), "document not found")
		errorCode := "REGISTRATION_FAILED"
		if isDocumentNotFound {
			errorCode = "DOCUMENT_NOT_FOUND_RADIAN"
		}

		return documentProcessingResult{
			Cufe:                cufe,
			DocumentNumber:      documentNumber,
			Status:              "error",
			IsDocumentNotFound:  isDocumentNotFound,
			ErrorDetails: &EventErrorDetails{
				Stage:       "event_registration",
				ErrorCode:   errorCode,
				Description: err.Error(),
			},
		}
	}

	// 10. Analyze event result to determine status
	result := documentProcessingResult{
		Cufe:           cufe,
		DocumentNumber: eventResult.NumeroDocumento,
		EventResult:    eventResult,
	}

	// Analyze the result to determine status
	result.Status, result.IsDuplicate, result.IsInvalidEvent = h.analyzeEventResult(eventResult)

	h.log.Info("Event registration completed",
		"cufe", cufe,
		"event_type", eventType,
		"document_number", documentNumber,
		"emisor_nit", emisorNit,
		"status", result.Status)

	return result
}

// analyzeEventResult analyzes the EventRegistrationResult to determine status
func (h *Handler) analyzeEventResult(eventResult *invoice.EventRegistrationResult) (status string, isDuplicate bool, isInvalidEvent bool) {
	// Check for invalid event code (1004)
	for _, res := range eventResult.Resultado {
		if res.CodigoRespuesta == "1004" {
			return "error", false, true
		}
	}

	// Check main code
	if eventResult.Code == "1000" {
		// Check if all results are successful
		allSuccess := true
		for _, res := range eventResult.Resultado {
			if res.CodigoRespuesta != "1000" {
				allSuccess = false
				break
			}
		}
		if allSuccess {
			return "success", false, false
		}
		// Some events succeeded, some failed
		return "partial", false, false
	}

	// Code is "1001" or other - check for duplicates
	// Detect duplicates by checking error messages
	for _, res := range eventResult.Resultado {
		errorMsg := strings.ToLower(res.MensajeError)
		if strings.Contains(errorMsg, "ya") || strings.Contains(errorMsg, "already") ||
			strings.Contains(errorMsg, "duplicado") || strings.Contains(errorMsg, "duplicate") ||
			strings.Contains(errorMsg, "procesado anteriormente") {
			return "duplicate", true, false
		}
	}

	// If code is 1001, it's a rejection
	if eventResult.Code == "1001" {
		return "partial", false, false
	}

	// Default to error
	return "error", false, false
}

// buildResponseMessages builds the exitosos and fallidos messages based on processing results
func (h *Handler) buildResponseMessages(results []documentProcessingResult) (exitosos []string, fallidos []string) {
	exitosos = make([]string, 0)
	fallidos = make([]string, 0)

	successDocs := make([]string, 0)
	partialDocs := make([]string, 0)
	failedDocs := make([]string, 0)
	duplicateDocs := make([]string, 0)

	for _, result := range results {
		docNum := result.DocumentNumber
		if docNum == "" {
			docNum = result.Cufe
		}

		switch result.Status {
		case "success":
			successDocs = append(successDocs, docNum)
		case "partial":
			partialDocs = append(partialDocs, docNum)
		case "duplicate":
			duplicateDocs = append(duplicateDocs, docNum)
		default:
			failedDocs = append(failedDocs, docNum)
		}
	}

	// Build success message - all successful
	if len(successDocs) > 0 && len(partialDocs) == 0 && len(failedDocs) == 0 && len(duplicateDocs) == 0 {
		exitosos = append(exitosos, "Documentos agendados con exito.")
		return exitosos, fallidos
	}

	// Build partial success message - some successful or partial
	if len(successDocs) > 0 || len(partialDocs) > 0 {
		onlineDocs := append(successDocs, partialDocs...)
		motivos := make([]string, 0)
		
		// Collect reasons from partial and failed results
		for _, result := range results {
			if result.Status == "partial" && result.EventResult != nil {
				for _, res := range result.EventResult.Resultado {
					if res.CodigoRespuesta != "1000" && res.MensajeError != "" {
						motivos = append(motivos, res.MensajeError)
					}
				}
			} else if result.Status == "error" && result.EventResult != nil {
				for _, res := range result.EventResult.Resultado {
					if res.MensajeError != "" {
						motivos = append(motivos, res.MensajeError)
					}
				}
			}
		}

		msg := fmt.Sprintf("Algunos documento no fueron agendados. .  - Documentos eventos DIAN en línea: [%s].", strings.Join(onlineDocs, ", "))
		if len(motivos) > 0 {
			msg += fmt.Sprintf(" Motivos: [%s]", strings.Join(motivos, ", "))
		}
		exitosos = append(exitosos, msg)
		return exitosos, fallidos
	}

	// Build failure message - all failed
	if len(failedDocs) > 0 || len(duplicateDocs) > 0 {
		allFailedDocs := append(failedDocs, duplicateDocs...)
		motivos := make([]string, 0)
		
		for _, result := range results {
			if result.Status == "error" {
				if result.IsDocumentNotFound {
					motivos = append(motivos, "Documento no encontrado")
				} else if result.ErrorDetails != nil && result.ErrorDetails.Description != "" {
					motivos = append(motivos, result.ErrorDetails.Description)
				} else if result.EventResult != nil {
					for _, res := range result.EventResult.Resultado {
						if res.MensajeError != "" {
							motivos = append(motivos, res.MensajeError)
						} else if res.CodigoRespuesta != "1000" && res.Mensaje != "" {
							// Use Mensaje if there's no MensajeError
							motivos = append(motivos, res.Mensaje)
						}
					}
				}
			} else if result.Status == "duplicate" {
				// For duplicates, check if there's a specific error message from Numrot
				if result.EventResult != nil {
					for _, res := range result.EventResult.Resultado {
						if res.MensajeError != "" {
							motivos = append(motivos, res.MensajeError)
						}
					}
				}
				// If no specific message, use default
				if len(motivos) == 0 || !strings.Contains(strings.ToLower(strings.Join(motivos, " ")), "procesado") {
					motivos = append(motivos, "El documento ya fue procesado anteriormente")
				}
			}
		}

		msg := fmt.Sprintf("No se agendó ningún documento. Documentos no agendados: [%s].", strings.Join(allFailedDocs, ", "))
		if len(motivos) > 0 {
			msg += fmt.Sprintf(" Motivos: [%s]", strings.Join(motivos, ", "))
		}
		fallidos = append(fallidos, msg)
	}

	return exitosos, fallidos
}

// determineHTTPStatus determines the appropriate HTTP status code based on results
func (h *Handler) determineHTTPStatus(results []documentProcessingResult) int {
	hasSuccess := false
	hasPartial := false
	hasDuplicate := false
	hasDocumentNotFound := false
	hasInvalidEvent := false
	hasValidationError := false

	for _, result := range results {
		switch result.Status {
		case "success":
			hasSuccess = true
		case "partial":
			hasPartial = true
		case "duplicate":
			hasDuplicate = true
		case "error":
			if result.IsDocumentNotFound {
				hasDocumentNotFound = true
			}
			if result.IsInvalidEvent {
				hasInvalidEvent = true
			}
			// Check if it's a validation error
			if result.ErrorDetails != nil && result.ErrorDetails.Stage == "validation" {
				hasValidationError = true
			}
		}
	}

	// 409: Duplicate documents (only if all are duplicates and no success/partial)
	if hasDuplicate && !hasSuccess && !hasPartial {
		return http.StatusConflict
	}

	// 400: Invalid data (document not found, invalid event, validation errors)
	// But only if there's no success or partial success
	if !hasSuccess && !hasPartial {
		if hasDocumentNotFound || hasInvalidEvent || hasValidationError {
			return http.StatusBadRequest
		}
	}

	// 200: Success (all, partial, or some successful)
	// This includes cases where some documents succeeded even if others failed
	if hasSuccess || hasPartial {
		return http.StatusOK
	}

	// 500: Internal server error (unexpected errors)
	return http.StatusInternalServerError
}

// ListarDocumentos handles legacy POST /api/recepcion/documentos/listar-documentos (form data)
func (h *Handler) ListarDocumentos(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httperrors.WriteError(w, http.StatusMethodNotAllowed, "Método no permitido", []string{"Este endpoint solo acepta POST"}, h.log)
		return
	}

	if err := r.ParseForm(); err != nil {
		httperrors.WriteError(w, http.StatusBadRequest, "Error de Validación", []string{"No se pudieron parsear parámetros"}, h.log)
		return
	}

	fechaDesde := r.PostFormValue("fecha_desde")
	fechaHasta := r.PostFormValue("fecha_hasta")
	if fechaDesde == "" || fechaHasta == "" {
		httperrors.WriteError(w, http.StatusBadRequest, "Error de Validación", []string{"fecha_desde y fecha_hasta son requeridos"}, h.log)
		return
	}

	// Return empty list placeholder
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "200", "message": "Exitoso", "documents": []interface{}{}})
}

// ConsultaDocumentos handles POST /api/v1/recepcion/documentos/consulta-documentos
// POST with query parameter ?cufe=... (consulta por CUFE)
// POST with query parameters ?proveedor=...&consecutivo=...&prefijo=...&ofe=...&tipo=... (consulta por parámetros de proveedor)
// POST with query parameter ?fecha=... or form data fecha=... (consulta por fecha)
func (h *Handler) ConsultaDocumentos(w http.ResponseWriter, r *http.Request) {
	// Validar que el cliente Numrot esté configurado
	if h.numrotClient == nil {
		h.log.Error("Numrot client not configured for consulta-documentos")
		httperrors.WriteError(w, http.StatusServiceUnavailable, "Servicio No Disponible",
			[]string{"Servicio de consulta no configurado"}, h.log)
		return
	}

	// Solo permitir POST
	if r.Method != http.MethodPost {
		httperrors.WriteError(w, http.StatusMethodNotAllowed, "Método no permitido",
			[]string{"Este endpoint solo acepta POST"}, h.log)
		return
	}

	// Parsear form data si está presente (para Content-Type: application/x-www-form-urlencoded)
	contentType := r.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/x-www-form-urlencoded") {
		if err := r.ParseForm(); err != nil {
			h.log.Warn("Failed to parse form data", "error", err)
			// Continuar, los query parameters aún pueden estar disponibles
		}
	}

	// Determinar si es consulta por CUFE (query parameter)
	cufe := r.URL.Query().Get("cufe")
	if cufe == "" {
		// Intentar obtener desde form data
		cufe = r.PostFormValue("cufe")
	}
	if cufe != "" {
		// Consulta por CUFE (método existente)
		if h.emisorNit == "" {
			h.log.Error("Emisor NIT not configured for consulta-documentos")
			httperrors.WriteError(w, http.StatusInternalServerError, "Error de Configuración",
				[]string{"Configuración del servicio inválida"}, h.log)
			return
		}

		h.log.Info("Consulta documento por CUFE", "cufe", cufe, "nit", h.emisorNit)

		// Llamar a Numrot DocumentInfo API
		docInfoResp, err := h.numrotClient.GetDocumentInfo(r.Context(), h.emisorNit, cufe)
		if err != nil {
			h.log.Error("Numrot error", "error", err, "cufe", cufe)

			// Manejar diferentes tipos de errores
			if strings.Contains(err.Error(), "document not found") {
				httperrors.WriteError(w, http.StatusNotFound, "Documento No Encontrado",
					[]string{"No existe documento con ese CUFE"}, h.log)
			} else if strings.Contains(err.Error(), "authentication failed") {
				httperrors.WriteError(w, http.StatusBadGateway, "Error de Autenticación",
					[]string{"Error de autenticación con el proveedor"}, h.log)
			} else {
				httperrors.WriteError(w, http.StatusBadGateway, "Error del Proveedor",
					[]string{"Error al consultar documento"}, h.log)
			}
			return
		}

		// Validar que la respuesta tenga al menos un documento
		if len(docInfoResp.DocumentInfo) == 0 {
			h.log.Warn("Numrot returned empty document list", "cufe", cufe)
			httperrors.WriteError(w, http.StatusNotFound, "Documento No Encontrado",
				[]string{"Respuesta vacía del proveedor"}, h.log)
			return
		}

		// Transformar a formato legacy (usar solo el primer documento)
		// No tenemos UrlPDF/UrlXML aquí porque no usamos GetDocumentByNumber
		legacyDoc := transformToLegacyFormat(r.Context(), &docInfoResp.DocumentInfo[0], nil, nil)
		response := ConsultaDocumentosResponse{Data: []LegacyDocumentData{legacyDoc}}

		h.log.Info("Documento encontrado",
			"prefijo", legacyDoc.Prefijo,
			"consecutivo", legacyDoc.Consecutivo,
			"clasificacion", legacyDoc.CdoClasificacion)

		// Escribir respuesta
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.log.Error("Failed to encode response", "error", err)
		}
		return
	}

	// Consulta por fecha (query parameter o form data)
	fecha := r.URL.Query().Get("fecha")
	if fecha == "" {
		fecha = r.PostFormValue("fecha")
	}
	fechaDesde := r.URL.Query().Get("fecha_desde")
	if fechaDesde == "" {
		fechaDesde = r.PostFormValue("fecha_desde")
	}
	fechaHasta := r.URL.Query().Get("fecha_hasta")
	if fechaHasta == "" {
		fechaHasta = r.PostFormValue("fecha_hasta")
	}

	// Si hay fecha o rango de fechas, consultar por fecha
	if fecha != "" || (fechaDesde != "" && fechaHasta != "") {
		// Validar que el emisor NIT esté configurado
		if h.emisorNit == "" {
			h.log.Error("Emisor NIT not configured for consulta-documentos by date")
			httperrors.WriteError(w, http.StatusInternalServerError, "Error de Configuración",
				[]string{"Configuración del servicio inválida"}, h.log)
			return
		}

		// Determinar fechas inicial y final
		var initialDate, finalDate string
		if fecha != "" {
			// Si solo hay una fecha, agregar 1 día a la fecha final para crear un rango válido
			// La API de Numrot no maneja bien cuando InitialDate == FinalDate
			fechaParsed, err := time.Parse(time.DateOnly, fecha)
			if err == nil {
				initialDate = fecha
				finalDate = fechaParsed.AddDate(0, 0, 1).Format(time.DateOnly) // Agregar 1 día
			} else {
				// Si hay error al parsear, usar la fecha tal cual (se validará después)
				initialDate = fecha
				finalDate = fecha
			}
		} else {
			initialDate = fechaDesde
			finalDate = fechaHasta
		}

		// Validar formato de fecha (YYYY-MM-DD)
		if _, err := time.Parse(time.DateOnly, initialDate); err != nil {
			httperrors.WriteError(w, http.StatusBadRequest, "Error de Validación",
				[]string{"Formato de fecha inválido. Debe ser YYYY-MM-DD"}, h.log)
			return
		}
		if _, err := time.Parse(time.DateOnly, finalDate); err != nil {
			httperrors.WriteError(w, http.StatusBadRequest, "Error de Validación",
				[]string{"Formato de fecha inválido. Debe ser YYYY-MM-DD"}, h.log)
			return
		}

		h.log.Info("Consulta documentos por fecha",
			"initialDate", initialDate,
			"finalDate", finalDate,
			"emisorNit", h.emisorNit)

		// 1. Llamar a GetReceivedDocuments para obtener la lista de documentos recibidos por fecha
		query := invoice.DocumentQuery{
			CompanyNit:  h.emisorNit,
			InitialDate: initialDate,
			FinalDate:   finalDate,
		}

		documents, err := h.numrotClient.GetReceivedDocuments(r.Context(), query)
		if err != nil {
			h.log.Error("Numrot GetReceivedDocuments error", "error", err,
				"initialDate", initialDate,
				"finalDate", finalDate,
				"emisorNit", h.emisorNit)

			// Manejar diferentes tipos de errores
			if strings.Contains(err.Error(), "document not found") || strings.Contains(err.Error(), "No se encontraron documentos") {
				// Retornar lista vacía en lugar de error 404
				response := ConsultaDocumentosResponse{Data: []LegacyDocumentData{}}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				if err := json.NewEncoder(w).Encode(response); err != nil {
					h.log.Error("Failed to encode response", "error", err)
				}
				return
			} else if strings.Contains(err.Error(), "authentication failed") {
				httperrors.WriteError(w, http.StatusBadGateway, "Error de Autenticación",
					[]string{"Error de autenticación con el proveedor"}, h.log)
			} else {
				httperrors.WriteError(w, http.StatusBadGateway, "Error del Proveedor",
					[]string{"Error al consultar documentos por fecha"}, h.log)
			}
			return
		}

		// Si no hay documentos, retornar lista vacía
		if len(documents) == 0 {
			h.log.Info("No se encontraron documentos para el rango de fechas",
				"initialDate", initialDate,
				"finalDate", finalDate)
			response := ConsultaDocumentosResponse{Data: []LegacyDocumentData{}}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			if err := json.NewEncoder(w).Encode(response); err != nil {
				h.log.Error("Failed to encode response", "error", err)
			}
			return
		}

		// 2. Para cada documento, obtener información completa usando GetDocumentInfo
		legacyDocs := make([]LegacyDocumentData, 0, len(documents))
		for _, doc := range documents {
			if doc.CUFE == "" {
				h.log.Warn("Document without CUFE, skipping", "ofe", doc.OFE, "proveedor", doc.Proveedor)
				continue
			}

			// Para documentos recibidos, usar el NIT del proveedor (emisor) en lugar del emisorNit (receptor)
			// doc.Proveedor es el NIT del emisor del documento recibido
			proveedorNit := doc.Proveedor
			if proveedorNit == "" {
				h.log.Warn("Document without proveedor NIT, skipping", "cufe", doc.CUFE, "ofe", doc.OFE)
				continue
			}

			// Obtener información completa del documento usando el NIT del proveedor (emisor)
			docInfoResp, err := h.numrotClient.GetDocumentInfo(r.Context(), proveedorNit, doc.CUFE)
			if err != nil {
				h.log.Warn("Failed to get document info", "error", err, "cufe", doc.CUFE, "proveedor_nit", proveedorNit)
				// Continuar con el siguiente documento en lugar de fallar completamente
				continue
			}

			// Transformar a formato legacy
			// No tenemos UrlPDF/UrlXML aquí porque no usamos GetDocumentByNumber
			if len(docInfoResp.DocumentInfo) > 0 {
				legacyDoc := transformToLegacyFormat(r.Context(), &docInfoResp.DocumentInfo[0], nil, nil)
				legacyDocs = append(legacyDocs, legacyDoc)
			}
		}

		h.log.Info("Documentos encontrados por fecha",
			"total", len(legacyDocs),
			"initialDate", initialDate,
			"finalDate", finalDate)

		// 3. Retornar todos los documentos encontrados
		response := ConsultaDocumentosResponse{Data: legacyDocs}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.log.Error("Failed to encode response", "error", err)
		}
		return
	}

	// Consulta por parámetros de proveedor (query parameters)
	proveedor := r.URL.Query().Get("proveedor")
	if proveedor == "" {
		proveedor = r.PostFormValue("proveedor")
	}
	consecutivo := r.URL.Query().Get("consecutivo")
	if consecutivo == "" {
		consecutivo = r.PostFormValue("consecutivo")
	}
	prefijo := r.URL.Query().Get("prefijo")
	if prefijo == "" {
		prefijo = r.PostFormValue("prefijo")
	}
	ofe := r.URL.Query().Get("ofe")
	if ofe == "" {
		ofe = r.PostFormValue("ofe")
	}
	tipo := r.URL.Query().Get("tipo")
	if tipo == "" {
		tipo = r.PostFormValue("tipo")
	}

	// Validar que todos los parámetros requeridos estén presentes
	if proveedor == "" || consecutivo == "" || prefijo == "" || ofe == "" {
		httperrors.WriteError(w, http.StatusBadRequest, "Error de Validación",
			[]string{"proveedor, consecutivo, prefijo y ofe son requeridos, o se debe proporcionar fecha para consultar por fecha"}, h.log)
		return
	}

	// Construir número de documento: prefijo + consecutivo
	documentNumber := prefijo + consecutivo

	h.log.Info("Consulta documento por parámetros de proveedor",
		"proveedor", proveedor,
		"consecutivo", consecutivo,
		"prefijo", prefijo,
		"ofe", ofe,
		"tipo", tipo,
		"documentNumber", documentNumber)

	// 1. Llamar a GetDocumentByNumber para obtener el documento y su CUFE
	query := invoice.DocumentByNumberQuery{
		CompanyNit:     ofe,
		SupplierNit:    proveedor,
		DocumentNumber: documentNumber,
	}

	documents, err := h.numrotClient.GetDocumentByNumber(r.Context(), query)
	if err != nil {
		h.log.Error("Numrot GetDocumentByNumber error", "error", err,
			"proveedor", proveedor,
			"consecutivo", consecutivo,
			"prefijo", prefijo,
			"ofe", ofe,
			"documentNumber", documentNumber)

		// Manejar diferentes tipos de errores
		if strings.Contains(err.Error(), "document not found") || strings.Contains(err.Error(), "No se encontraron documentos") {
			httperrors.WriteError(w, http.StatusNotFound, "Documento No Encontrado",
				[]string{"No existe documento con los parámetros proporcionados"}, h.log)
		} else if strings.Contains(err.Error(), "authentication failed") {
			httperrors.WriteError(w, http.StatusBadGateway, "Error de Autenticación",
				[]string{"Error de autenticación con el proveedor"}, h.log)
		} else {
			httperrors.WriteError(w, http.StatusBadGateway, "Error del Proveedor",
				[]string{"Error al consultar documento"}, h.log)
		}
		return
	}

	// Validar que se haya encontrado al menos un documento
	if len(documents) == 0 {
		h.log.Warn("GetDocumentByNumber returned empty document list",
			"proveedor", proveedor,
			"consecutivo", consecutivo,
			"prefijo", prefijo,
			"ofe", ofe,
			"documentNumber", documentNumber)
		httperrors.WriteError(w, http.StatusNotFound, "Documento No Encontrado",
			[]string{"No se encontraron documentos con los parámetros proporcionados"}, h.log)
		return
	}

	// 2. Extraer CUFE, UrlPDF y UrlXML del primer documento
	cufeFromDoc := documents[0].CUFE
	if cufeFromDoc == "" {
		h.log.Error("Document found but CUFE is empty",
			"proveedor", proveedor,
			"consecutivo", consecutivo,
			"prefijo", prefijo,
			"ofe", ofe)
		httperrors.WriteError(w, http.StatusBadGateway, "Error del Proveedor",
			[]string{"El documento encontrado no tiene CUFE válido"}, h.log)
		return
	}

	// Extraer UrlPDF y UrlXML del documento obtenido de GetDocumentByNumber
	var urlPDFPtr *string
	if documents[0].UrlPDF != "" {
		urlPDFPtr = &documents[0].UrlPDF
	}
	var urlXMLPtr *string
	if documents[0].UrlXML != "" {
		urlXMLPtr = &documents[0].UrlXML
	}

	// 3. Llamar a GetDocumentInfo con el CUFE para obtener información completa
	if h.emisorNit == "" {
		h.log.Error("Emisor NIT not configured for consulta-documentos")
		httperrors.WriteError(w, http.StatusInternalServerError, "Error de Configuración",
			[]string{"Configuración del servicio inválida"}, h.log)
		return
	}

	h.log.Info("Obteniendo información completa del documento", "cufe", cufeFromDoc, "nit", h.emisorNit)

	docInfoResp, err := h.numrotClient.GetDocumentInfo(r.Context(), h.emisorNit, cufeFromDoc)
	if err != nil {
		h.log.Error("Numrot GetDocumentInfo error", "error", err, "cufe", cufeFromDoc)

		// Manejar diferentes tipos de errores
		if strings.Contains(err.Error(), "document not found") {
			httperrors.WriteError(w, http.StatusNotFound, "Documento No Encontrado",
				[]string{"No se pudo obtener información completa del documento"}, h.log)
		} else if strings.Contains(err.Error(), "authentication failed") {
			httperrors.WriteError(w, http.StatusBadGateway, "Error de Autenticación",
				[]string{"Error de autenticación con el proveedor"}, h.log)
		} else {
			httperrors.WriteError(w, http.StatusBadGateway, "Error del Proveedor",
				[]string{"Error al obtener información completa del documento"}, h.log)
		}
		return
	}

	// Validar que la respuesta tenga al menos un documento
	if len(docInfoResp.DocumentInfo) == 0 {
		h.log.Warn("GetDocumentInfo returned empty document list", "cufe", cufeFromDoc)
		httperrors.WriteError(w, http.StatusNotFound, "Documento No Encontrado",
			[]string{"Respuesta vacía del proveedor"}, h.log)
		return
	}

	// 4. Transformar a formato legacy (usar solo el primer documento)
	// Pasar UrlPDF y UrlXML obtenidos de GetDocumentByNumber
	legacyDoc := transformToLegacyFormat(r.Context(), &docInfoResp.DocumentInfo[0], urlPDFPtr, urlXMLPtr)
	response := ConsultaDocumentosResponse{Data: []LegacyDocumentData{legacyDoc}}

	h.log.Info("Documento encontrado",
		"prefijo", legacyDoc.Prefijo,
		"consecutivo", legacyDoc.Consecutivo,
		"clasificacion", legacyDoc.CdoClasificacion,
		"cufe", cufeFromDoc)

	// Escribir respuesta
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.log.Error("Failed to encode response", "error", err)
	}
}
