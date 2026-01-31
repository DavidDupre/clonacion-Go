package event

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	appevent "3tcapital/ms_facturacion_core/internal/application/event"
	"3tcapital/ms_facturacion_core/internal/core/event"
	"3tcapital/ms_facturacion_core/internal/core/invoice"
	httperrors "3tcapital/ms_facturacion_core/internal/infrastructure/http"
)

// Handler bridges HTTP traffic with the event application service.
type Handler struct {
	service *appevent.Service
}

// NewHandler creates a new event HTTP handler.
func NewHandler(service *appevent.Service) *Handler {
	return &Handler{
		service: service,
	}
}

// RegisterEventRequest represents the request body for registering an event.
type RegisterEventRequest struct {
	EventType              string `json:"EventType"`
	DocumentoNumeroCompleto string `json:"DocumentoNumeroCompleto"`
	NombreGenerador        string `json:"NombreGenerador"`
	ApellidoGenerador      string `json:"ApellidoGenerador"`
	IdentificacionGenerador string `json:"IdentificacionGenerador"`
	CodigoRechazo          string `json:"CodigoRechazo,omitempty"`
	FechaGeneracionEvento  string `json:"FechaGeneracionEvento"`
}

// RegisterEventResponse represents the response format for event registration.
type RegisterEventResponse struct {
	Status  string                        `json:"status"`
	Message string                        `json:"message"`
	Data    *invoice.EventRegistrationResult `json:"data,omitempty"`
}

// RegisterEvent handles POST /api/v1/eventos requests.
func (h *Handler) RegisterEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httperrors.WriteError(w, http.StatusMethodNotAllowed, "Método no permitido", []string{"Este endpoint solo acepta POST"}, nil)
		return
	}

	var reqBody RegisterEventRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		httperrors.WriteError(w, http.StatusBadRequest, "Error de Validación", []string{"El cuerpo de la petición no es válido"}, nil)
		return
	}

	// Validate required fields
	if reqBody.EventType == "" {
		httperrors.WriteError(w, http.StatusBadRequest, "Error de Validación", []string{"EventType es requerido"}, nil)
		return
	}

	if reqBody.DocumentoNumeroCompleto == "" {
		httperrors.WriteError(w, http.StatusBadRequest, "Error de Validación", []string{"DocumentoNumeroCompleto es requerido"}, nil)
		return
	}

	if reqBody.NombreGenerador == "" {
		httperrors.WriteError(w, http.StatusBadRequest, "Error de Validación", []string{"NombreGenerador es requerido"}, nil)
		return
	}

	if reqBody.ApellidoGenerador == "" {
		httperrors.WriteError(w, http.StatusBadRequest, "Error de Validación", []string{"ApellidoGenerador es requerido"}, nil)
		return
	}

	if reqBody.IdentificacionGenerador == "" {
		httperrors.WriteError(w, http.StatusBadRequest, "Error de Validación", []string{"IdentificacionGenerador es requerido"}, nil)
		return
	}

	if reqBody.FechaGeneracionEvento == "" {
		httperrors.WriteError(w, http.StatusBadRequest, "Error de Validación", []string{"FechaGeneracionEvento es requerido"}, nil)
		return
	}

	// Parse and validate event type
	eventType := event.EventType(reqBody.EventType)
	if !event.ValidateEventType(eventType) {
		httperrors.WriteError(w, http.StatusBadRequest, "Error de Validación", []string{"EventType inválido. Debe ser uno de: ACUSE, RECIBOBIEN, ACEPTACION, RECLAMO"}, nil)
		return
	}

	// Parse event generation date
	dateFormat := "2006-01-02 15:04:05"
	fechaGeneracionEvento, err := time.Parse(dateFormat, reqBody.FechaGeneracionEvento)
	if err != nil {
		httperrors.WriteError(w, http.StatusBadRequest, "Error de Validación", []string{"FechaGeneracionEvento debe tener el formato YYYY-MM-DD HH:MM:SS"}, nil)
		return
	}

	// Build domain event
	evt := event.Event{
		EventType:              eventType,
		DocumentNumber:         reqBody.DocumentoNumeroCompleto,
		NombreGenerador:        reqBody.NombreGenerador,
		ApellidoGenerador:      reqBody.ApellidoGenerador,
		IdentificacionGenerador: reqBody.IdentificacionGenerador,
		EventGenerationDate:    fechaGeneracionEvento,
	}

	// Add rejection code if provided
	if reqBody.CodigoRechazo != "" {
		rejectionCode := event.RejectionCode(reqBody.CodigoRechazo)
		if !event.ValidateRejectionCode(rejectionCode) {
			httperrors.WriteError(w, http.StatusBadRequest, "Error de Validación", []string{"CodigoRechazo inválido. Debe ser uno de: 01, 02, 03, 04"}, nil)
			return
		}
		evt.RejectionCode = &rejectionCode
	}

	// Register event
	result, err := h.service.RegisterEvent(r.Context(), evt)
	if err != nil {
		h.handleError(w, err)
		return
	}

	// Transform to response format
	response := RegisterEventResponse{
		Status:  "200",
		Message: "Exitoso",
		Data:    result,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		// Log error but response already sent
		httperrors.WriteError(w, http.StatusInternalServerError, "Error Interno del Servidor", []string{"Ha ocurrido un error interno"}, nil)
	}
}

// handleError maps domain errors to appropriate HTTP status codes.
func (h *Handler) handleError(w http.ResponseWriter, err error) {
	// Check error type and return appropriate status
	errorMsg := err.Error()

	switch {
	case contains(errorMsg, "validation error") || contains(errorMsg, "invalid event type") || contains(errorMsg, "document number is required") || contains(errorMsg, "nombre generador is required") || contains(errorMsg, "apellido generador is required") || contains(errorMsg, "identificacion generador is required") || contains(errorMsg, "rejection code is required") || contains(errorMsg, "invalid rejection code"):
		httperrors.WriteError(w, http.StatusBadRequest, "Error de Validación", []string{errorMsg}, nil)
	case contains(errorMsg, "emisor nit is not configured") || contains(errorMsg, "razon social is not configured"):
		httperrors.WriteError(w, http.StatusInternalServerError, "Error de Configuración", []string{"Error de configuración del servicio"}, nil)
	case contains(errorMsg, "document not found"):
		httperrors.WriteError(w, http.StatusNotFound, "Documento No Encontrado", []string{errorMsg}, nil)
	case contains(errorMsg, "provider error") || contains(errorMsg, "execute request") || contains(errorMsg, "read response body") || contains(errorMsg, "unexpected status code") || contains(errorMsg, "unmarshal response") || contains(errorMsg, "marshal request"):
		httperrors.WriteError(w, http.StatusBadGateway, "Error del Proveedor", []string{"Servicio del proveedor no disponible"}, nil)
	default:
		httperrors.WriteError(w, http.StatusInternalServerError, "Error Interno del Servidor", []string{"Ha ocurrido un error interno"}, nil)
	}
}

// contains checks if a string contains a substring (case-insensitive).
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
