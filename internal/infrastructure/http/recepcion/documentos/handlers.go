package documentos

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// RegisterEventPayload represents the expected JSON payload for registrar-evento
type RegisterEventPayload struct {
	Evento     string      `json:"evento"`
	Documentos []Documento `json:"documentos"`
}

// Documento represents a document entry inside the payload
type Documento struct {
	CdoCufe        string `json:"cdo_cufe"`
	CdoFecha       string `json:"cdo_fecha"`
	CdoObservacion string `json:"cdo_observacion"`
	CreCodigo      string `json:"cre_codigo"`
}

// NewRegistrarEventoHandler returns an http.Handler that parses JSON payload and echoes it back.
func NewRegistrarEventoHandler(log *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var payload RegisterEventPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "invalid JSON payload"})
			if log != nil {
				log.Error("failed to decode registrar-evento payload", "error", err)
			}
			return
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "received": payload})
	})
}

// NewListarDocumentosHandler returns an http.Handler that parses form values and returns an empty list.
func NewListarDocumentosHandler(log *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Expect application/x-www-form-urlencoded
		if err := r.ParseForm(); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "invalid form payload"})
			if log != nil {
				log.Error("failed to parse form", "error", err)
			}
			return
		}

		fechaDesde := r.FormValue("fecha_desde")
		fechaHasta := r.FormValue("fecha_hasta")

		// Placeholder response: empty documents list
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "fecha_desde": fechaDesde, "fecha_hasta": fechaHasta, "documents": []any{}})
	})
}

// NewConsultaDocumentosHandler returns an http.Handler that reads query param `cufe` and returns a placeholder.
func NewConsultaDocumentosHandler(log *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		cufe := r.URL.Query().Get("cufe")
		if cufe == "" {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": "missing cufe query parameter"})
			return
		}

		// Placeholder response: no document found
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "cufe": cufe, "document": nil})
	})
}
