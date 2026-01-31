package health

import (
	"encoding/json"
	"net/http"

	apphealth "3tcapital/ms_facturacion_core/internal/application/health"
)

// Handler bridges HTTP traffic with the health application service.
type Handler struct {
	service *apphealth.Service
}

func NewHandler(service *apphealth.Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	response := h.service.Status(r.Context())

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}
