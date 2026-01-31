package resolution

import (
	"encoding/json"
	"net/http"
	"strings"

	appresolution "3tcapital/ms_facturacion_core/internal/application/resolution"
	httperrors "3tcapital/ms_facturacion_core/internal/infrastructure/http"
)

// Handler bridges HTTP traffic with the resolution application service.
type Handler struct {
	service  *appresolution.Service
	emisorNit string
}

// NewHandler creates a new resolution HTTP handler.
func NewHandler(service *appresolution.Service, emisorNit string) *Handler {
	return &Handler{
		service:  service,
		emisorNit: emisorNit,
	}
}

// GetResolutions handles GET /api/v1/configuracion/lista-resoluciones-facturacion requests.
func (h *Handler) GetResolutions(w http.ResponseWriter, r *http.Request) {
	nit := h.emisorNit
	if nit == "" {
		httperrors.WriteError(w, http.StatusBadRequest, "Error de Validación", []string{"El NIT del emisor no está configurado. Configure NUMROT_EMISOR_NIT"}, nil)
		return
	}

	resolutions, err := h.service.GetResolutions(r.Context(), nit)
	if err != nil {
		h.handleError(w, err)
		return
	}

	// If no resolutions found, return empty array (not 404, as this is a valid response)
	response := map[string]interface{}{
		"resolutions": resolutions,
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
	case contains(errorMsg, "nit is required"):
		httperrors.WriteError(w, http.StatusBadRequest, "Error de Validación", []string{"El parámetro NIT es requerido"}, nil)
	case contains(errorMsg, "invalid nit format"):
		httperrors.WriteError(w, http.StatusBadRequest, "Error de Validación", []string{"Formato de NIT inválido"}, nil)
	case contains(errorMsg, "authentication failed") || contains(errorMsg, "numrot authentication failed") || contains(errorMsg, "get authentication token"):
		httperrors.WriteError(w, http.StatusBadGateway, "Error de Autenticación", []string{"Error de autenticación con el proveedor"}, nil)
	case contains(errorMsg, "unexpected status code") || contains(errorMsg, "execute request") || contains(errorMsg, "read response body"):
		httperrors.WriteError(w, http.StatusBadGateway, "Error del Proveedor", []string{"Servicio del proveedor no disponible"}, nil)
	case contains(errorMsg, "numrot API error"):
		// Numrot API returned an error code (not 100)
		httperrors.WriteError(w, http.StatusBadGateway, "Error del Proveedor", []string{errorMsg}, nil)
	case contains(errorMsg, "unmarshal response"):
		httperrors.WriteError(w, http.StatusBadGateway, "Error del Proveedor", []string{"Error en el formato de respuesta del proveedor"}, nil)
	case contains(errorMsg, "provider error"):
		// Generic provider error - check the wrapped error
		httperrors.WriteError(w, http.StatusBadGateway, "Error del Proveedor", []string{"Servicio del proveedor no disponible"}, nil)
	default:
		httperrors.WriteError(w, http.StatusInternalServerError, "Error Interno del Servidor", []string{"Ha ocurrido un error interno"}, nil)
	}
}

// contains checks if a string contains a substring (case-insensitive).
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
