package provider

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	appprovider "3tcapital/ms_facturacion_core/internal/application/provider"
	httperrors "3tcapital/ms_facturacion_core/internal/infrastructure/http"

	"github.com/go-chi/chi/v5"
)

// Handler bridges HTTP traffic with the provider application service.
type Handler struct {
	service *appprovider.Service
}

// NewHandler creates a new provider HTTP handler.
func NewHandler(service *appprovider.Service) *Handler {
	return &Handler{
		service: service,
	}
}

// ListProviders handles GET /api/v1/proveedores requests with pagination, search, and sorting.
func (h *Handler) ListProviders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httperrors.WriteError(w, http.StatusMethodNotAllowed, "Método no permitido", []string{"Este endpoint solo acepta GET"}, nil)
		return
	}

	// Parse query parameters
	start := 0
	if startStr := r.URL.Query().Get("start"); startStr != "" {
		var err error
		start, err = strconv.Atoi(startStr)
		if err != nil || start < 0 {
			httperrors.WriteError(w, http.StatusBadRequest, "Error de Validación", []string{"start debe ser un número entero no negativo"}, nil)
			return
		}
	}

	length := 10 // Default
	if lengthStr := r.URL.Query().Get("length"); lengthStr != "" {
		var err error
		length, err = strconv.Atoi(lengthStr)
		if err != nil {
			httperrors.WriteError(w, http.StatusBadRequest, "Error de Validación", []string{"length debe ser un número entero (-1 para traer todos)"}, nil)
			return
		}
	}

	buscar := r.URL.Query().Get("buscar")
	columnaOrden := r.URL.Query().Get("columnaOrden")
	if columnaOrden == "" {
		columnaOrden = "codigo"
	}
	ordenDireccion := r.URL.Query().Get("ordenDireccion")
	if ordenDireccion == "" {
		ordenDireccion = "asc"
	}

	response, err := h.service.ListProviders(r.Context(), start, length, buscar, columnaOrden, ordenDireccion)
	if err != nil {
		h.handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		httperrors.WriteError(w, http.StatusInternalServerError, "Error Interno del Servidor", []string{"Ha ocurrido un error interno"}, nil)
	}
}

// SearchProvider handles GET /api/v1/proveedores/busqueda/{campoBuscar}/valor/{valorBuscar}/ofe/{valorOfe}/filtro/{filtroColumnas} requests.
func (h *Handler) SearchProvider(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		httperrors.WriteError(w, http.StatusMethodNotAllowed, "Método no permitido", []string{"Este endpoint solo acepta GET"}, nil)
		return
	}

	// Extract path parameters
	campoBuscar := chi.URLParam(r, "campoBuscar")
	valorBuscar := chi.URLParam(r, "valorBuscar")
	valorOfe := chi.URLParam(r, "valorOfe")
	filtroColumnas := chi.URLParam(r, "filtroColumnas")

	if campoBuscar == "" {
		httperrors.WriteError(w, http.StatusBadRequest, "Error de Validación", []string{"campoBuscar es requerido en la URL"}, nil)
		return
	}

	if valorBuscar == "" {
		httperrors.WriteError(w, http.StatusBadRequest, "Error de Validación", []string{"valorBuscar es requerido en la URL"}, nil)
		return
	}

	if valorOfe == "" {
		httperrors.WriteError(w, http.StatusBadRequest, "Error de Validación", []string{"valorOfe es requerido en la URL"}, nil)
		return
	}

	if filtroColumnas == "" {
		httperrors.WriteError(w, http.StatusBadRequest, "Error de Validación", []string{"filtroColumnas es requerido en la URL"}, nil)
		return
	}

	providers, err := h.service.SearchProvider(r.Context(), campoBuscar, valorBuscar, valorOfe, filtroColumnas)
	if err != nil {
		h.handleError(w, err)
		return
	}

	// Format response matching OpenETL format
	// OpenETL returns an array in "data"
	response := map[string]interface{}{
		"data": providers,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		httperrors.WriteError(w, http.StatusInternalServerError, "Error Interno del Servidor", []string{"Ha ocurrido un error interno"}, nil)
	}
}

// CreateProvider handles POST /api/v1/proveedores requests.
func (h *Handler) CreateProvider(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httperrors.WriteError(w, http.StatusMethodNotAllowed, "Método no permitido", []string{"Este endpoint solo acepta POST"}, nil)
		return
	}

	var reqBody appprovider.CreateProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		httperrors.WriteError(w, http.StatusBadRequest, "Error de Validación", []string{"El cuerpo de la petición no es válido"}, nil)
		return
	}

	response, err := h.service.CreateProvider(r.Context(), reqBody)
	if err != nil {
		h.handleError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		httperrors.WriteError(w, http.StatusInternalServerError, "Error Interno del Servidor", []string{"Ha ocurrido un error interno"}, nil)
	}
}

// UpdateProvider handles PUT /api/v1/proveedores/{ofeIdentificacion}/{proIdentificacion} requests.
func (h *Handler) UpdateProvider(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		httperrors.WriteError(w, http.StatusMethodNotAllowed, "Método no permitido", []string{"Este endpoint solo acepta PUT"}, nil)
		return
	}

	// Extract path parameters
	ofeIdentificacion := chi.URLParam(r, "ofeIdentificacion")
	proIdentificacion := chi.URLParam(r, "proIdentificacion")

	if ofeIdentificacion == "" {
		httperrors.WriteError(w, http.StatusBadRequest, "Error de Validación", []string{"ofeIdentificacion es requerido en la URL"}, nil)
		return
	}

	if proIdentificacion == "" {
		httperrors.WriteError(w, http.StatusBadRequest, "Error de Validación", []string{"proIdentificacion es requerido en la URL"}, nil)
		return
	}

	var reqBody appprovider.UpdateProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		httperrors.WriteError(w, http.StatusBadRequest, "Error de Validación", []string{"El cuerpo de la petición no es válido"}, nil)
		return
	}

	err := h.service.UpdateProvider(r.Context(), ofeIdentificacion, proIdentificacion, reqBody)
	if err != nil {
		h.handleError(w, err)
		return
	}

	response := map[string]interface{}{
		"success": true,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		httperrors.WriteError(w, http.StatusInternalServerError, "Error Interno del Servidor", []string{"Ha ocurrido un error interno"}, nil)
	}
}

// handleError maps domain errors to appropriate HTTP status codes and formats.
func (h *Handler) handleError(w http.ResponseWriter, err error) {
	errorMsg := err.Error()

	// Check for duplicate provider error
	if strings.Contains(errorMsg, "ya existe") {
		httperrors.WriteError(w, http.StatusConflict, "Errores al crear el Proveedor", []string{errorMsg}, nil)
		return
	}

	// Check for not found error
	if strings.Contains(errorMsg, "no existe") {
		httperrors.WriteError(w, http.StatusNotFound, "Errores al actualizar el Proveedor", []string{errorMsg}, nil)
		return
	}

	// Check for validation errors
	if strings.Contains(errorMsg, "es requerido") || strings.Contains(errorMsg, "debe ser") || strings.Contains(errorMsg, "no permitido") || strings.Contains(errorMsg, "inválido") {
		httperrors.WriteError(w, http.StatusBadRequest, "Error de Validación", []string{errorMsg}, nil)
		return
	}

	// Default to internal server error
	httperrors.WriteError(w, http.StatusInternalServerError, "Error Interno del Servidor", []string{"Ha ocurrido un error interno"}, nil)
}
