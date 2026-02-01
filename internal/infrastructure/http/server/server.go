package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
)

// Server expone únicamente los endpoints solicitados de clonación.
type Server struct {
	log        *slog.Logger
	httpServer *http.Server
	db         *sql.DB
}

// Options de construcción mínimos.
type Options struct {
	Addr   string
	Logger *slog.Logger
	DB     *sql.DB
}

// New crea el servidor con los endpoints requeridos.
func New(opts Options) (*Server, error) {
	if opts.Logger == nil {
		return nil, errors.New("logger is required")
	}
	if opts.Addr == "" {
		opts.Addr = ":8080"
	}

	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Recoverer)

	// Health
	r.Method(http.MethodGet, "/health", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	// Ejecutar job de alertas manual
	r.Method(http.MethodPost, "/admin/clonaciones/alertas/run", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"resultado": true})
	}))

	// Crear clonación (puede ser para múltiples usuarios)
	r.Method(http.MethodPost, "/clonaciones", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		contentType := strings.ToLower(req.Header.Get("Content-Type"))
		if strings.Contains(contentType, "application/json") {
			type tiempoPayload struct {
				Valor int `json:"valor"`
			}
			type usuarioClonadoPayload struct {
				UsuarioID string         `json:"usuarioId"`
				Tiempo    *tiempoPayload `json:"tiempo"`
			}
			type createClonacionesRequest struct {
				TramiteID string                  `json:"tramiteId"`
				Usuarios  []usuarioClonadoPayload `json:"usuarios"`
				Motivo    string                  `json:"motivo"`
				Adjuntos  []string                `json:"adjuntos"`
			}

			var body createClonacionesRequest
			if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
				http.Error(w, "invalid json", http.StatusBadRequest)
				return
			}
			if body.TramiteID == "" || body.Motivo == "" || len(body.Usuarios) == 0 {
				http.Error(w, "tramiteId, motivo y usuarios son requeridos", http.StatusBadRequest)
				return
			}
			usuarioAsignadorID := "00000000-0000-0000-0000-000000000000"

			now := time.Now()
			tx, err := opts.DB.Begin()
			if err != nil {
				opts.Logger.Error("failed to begin tx", "error", err)
				http.Error(w, "db error", http.StatusInternalServerError)
				return
			}
			defer tx.Rollback()

			var clonacionesCreadas []string
			for _, usuario := range body.Usuarios {
				if usuario.UsuarioID == "" {
					http.Error(w, "usuarioId es requerido en usuarios", http.StatusBadRequest)
					return
				}

				clonacionID := uuid.New()
				_, err = tx.Exec(`
					INSERT INTO clonaciones (id, tramite_id, usuario_clonado_id, usuario_asignador_id, motivo, estado, 
						contador_rechazos, created_at, updated_at)
					VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
				`,
					clonacionID, body.TramiteID, usuario.UsuarioID, usuarioAsignadorID, body.Motivo,
					"CLONACION_CREADA", 0, now, now)
				if err != nil {
					opts.Logger.Error("failed to insert clonacion", "error", err, "usuario", usuario.UsuarioID)
					http.Error(w, "db insert error", http.StatusInternalServerError)
					return
				}

				for _, adjunto := range body.Adjuntos {
					if strings.TrimSpace(adjunto) == "" {
						continue
					}
					normalized := strings.ReplaceAll(adjunto, "\\", "/")
					nombre := path.Base(normalized)
					_, err = tx.Exec(`
						INSERT INTO clonacion_adjuntos (id, clonacion_id, nombre, ruta_url, tipo, tamaño, created_at)
						VALUES ($1, $2, $3, $4, $5, $6, $7)
					`, uuid.New(), clonacionID, nombre, adjunto, "REFERENCE", 0, now)
					if err != nil {
						opts.Logger.Error("failed to insert adjunto", "error", err)
						http.Error(w, "db insert adjunto error", http.StatusInternalServerError)
						return
					}
				}

				clonacionesCreadas = append(clonacionesCreadas, clonacionID.String())
			}

			if err := tx.Commit(); err != nil {
				opts.Logger.Error("failed to commit tx", "error", err)
				http.Error(w, "db commit error", http.StatusInternalServerError)
				return
			}

			writeJSON(w, http.StatusOK, map[string]any{
				"mensaje":            "Asignaciones realizadas exitosamente",
				"estado":             "CLONACION_CREADA",
				"clonacionesCreadas": len(clonacionesCreadas),
				"ids":                clonacionesCreadas,
			})
			return
		}

		// Parsear multipart/form-data (máx 10MB)
		if err := req.ParseMultipartForm(10 << 20); err != nil {
			http.Error(w, "invalid multipart form", http.StatusBadRequest)
			return
		}

		// Obtener campos del formulario
		tramiteID := req.FormValue("tramiteId")
		usuarioAsignadorID := req.FormValue("usuarioAsignadorId")
		motivo := req.FormValue("motivo")
		// Recibir array de usuarios (separados por coma: "uuid1,uuid2,uuid3")
		usuariosClonadosStr := req.FormValue("usuariosClonadosIds")

		if tramiteID == "" || usuarioAsignadorID == "" || motivo == "" || usuariosClonadosStr == "" {
			http.Error(w, "tramiteId, usuarioAsignadorId, motivo y usuariosClonadosIds son requeridos", http.StatusBadRequest)
			return
		}

		// Parsear array de usuarios
		var usuariosClonadosIDs []string
		if err := json.Unmarshal([]byte(usuariosClonadosStr), &usuariosClonadosIDs); err != nil {
			http.Error(w, "usuariosClonadosIds debe ser un array JSON válido", http.StatusBadRequest)
			return
		}

		if len(usuariosClonadosIDs) == 0 {
			http.Error(w, "debe especificar al menos un usuario para clonar", http.StatusBadRequest)
			return
		}

		// Validar que el adjunto esté presente (OBLIGATORIO)
		file, header, err := req.FormFile("adjunto")
		if err != nil {
			http.Error(w, "adjunto es obligatorio", http.StatusBadRequest)
			return
		}
		defer file.Close()

		// Iniciar transacción
		now := time.Now()
		tx, err := opts.DB.Begin()
		if err != nil {
			opts.Logger.Error("failed to begin tx", "error", err)
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		adjuntoID := uuid.New()
		var clonacionesCreadas []string

		// Crear una clonación por cada usuario
		for _, usuarioClonadoID := range usuariosClonadosIDs {
			clonacionID := uuid.New()

			_, err = tx.Exec(`
				INSERT INTO clonaciones (id, tramite_id, usuario_clonado_id, usuario_asignador_id, motivo, estado, 
					contador_rechazos, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			`,
				clonacionID, tramiteID, usuarioClonadoID, usuarioAsignadorID, motivo,
				"CLONACION_CREADA", 0, now, now)
			if err != nil {
				opts.Logger.Error("failed to insert clonacion", "error", err, "usuario", usuarioClonadoID)
				http.Error(w, "db insert error", http.StatusInternalServerError)
				return
			}

			// Guardar referencia del adjunto para cada clonación
			_, err = tx.Exec(`
				INSERT INTO clonacion_adjuntos (id, clonacion_id, nombre, ruta_url, tipo, tamaño, created_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
			`, uuid.New(), clonacionID, header.Filename, "/uploads/"+adjuntoID.String(), header.Header.Get("Content-Type"), header.Size, now)
			if err != nil {
				opts.Logger.Error("failed to insert adjunto", "error", err)
				http.Error(w, "db insert adjunto error", http.StatusInternalServerError)
				return
			}

			clonacionesCreadas = append(clonacionesCreadas, clonacionID.String())
		}

		// TODO: Guardar archivo físico UNA SOLA VEZ (todas las clonaciones comparten el mismo archivo)
		// os.WriteFile("/uploads/"+adjuntoID.String(), fileBytes, 0644)

		if err := tx.Commit(); err != nil {
			opts.Logger.Error("failed to commit tx", "error", err)
			http.Error(w, "db commit error", http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"mensaje":            "Asignaciones realizadas exitosamente",
			"estado":             "CLONACION_CREADA",
			"clonacionesCreadas": len(clonacionesCreadas),
			"ids":                clonacionesCreadas,
		})
	}))

	// Consultar tiempo disponible por trámite
	r.Method(http.MethodGet, "/tramites/{tramiteId}/tiempo-disponible", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		tramiteID := chi.URLParam(req, "tramiteId")
		if tramiteID == "" {
			http.Error(w, "tramiteId requerido", http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"tiempoTotalTramite":    map[string]any{"valor": 0},
			"tiempoRestanteTramite": map[string]any{"valor": 0},
			"tiempoMaximoClonacion": map[string]any{"valor": 0},
			"permiteHorasYMinutos":  true,
		})
	}))

	// Listar clonaciones por trámite (servicio FE)
	r.Method(http.MethodGet, "/tramites/{tramiteId}/clonaciones", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		tramiteID := chi.URLParam(req, "tramiteId")
		if tramiteID == "" {
			http.Error(w, "tramiteId requerido", http.StatusBadRequest)
			return
		}
		rows, err := opts.DB.Query(`
			SELECT id, tramite_id, usuario_clonado_id, motivo, estado, contador_rechazos, created_at, updated_at
			FROM clonaciones
			WHERE tramite_id=$1 AND deleted_at IS NULL
			ORDER BY created_at DESC
		`, tramiteID)
		if err != nil {
			opts.Logger.Error("failed to list clonaciones", "error", err)
			http.Error(w, "db read error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		type clonacionListItem struct {
			ClonacionID        string `json:"clonacionId"`
			TramiteID          string `json:"tramiteId"`
			Motivo             string `json:"motivo"`
			Estado             string `json:"estado"`
			FechaCreacion      string `json:"fechaCreacion"`
			DestinatarioID     string `json:"destinatarioId"`
			DestinatarioNombre string `json:"destinatarioNombre"`
			RechazosRealizados int    `json:"rechazosRealizados"`
			MaximoRechazos     int    `json:"maximoRechazos"`
		}

		var result []clonacionListItem
		for rows.Next() {
			var (
				createdAt time.Time
				updatedAt time.Time
			)
			var item clonacionListItem
			if err := rows.Scan(&item.ClonacionID, &item.TramiteID, &item.DestinatarioID, &item.Motivo, &item.Estado, &item.RechazosRealizados, &createdAt, &updatedAt); err != nil {
				opts.Logger.Error("failed to scan clonacion", "error", err)
				http.Error(w, "db scan error", http.StatusInternalServerError)
				return
			}
			item.FechaCreacion = createdAt.Format(time.RFC3339)
			item.MaximoRechazos = 2
			result = append(result, item)
		}
		if err := rows.Err(); err != nil {
			opts.Logger.Error("failed to iterate clonaciones", "error", err)
			http.Error(w, "db rows error", http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, result)
	}))

	// Detalle de clonación
	r.Method(http.MethodGet, "/clonaciones/{clonacionId}", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		clonacionID := chi.URLParam(req, "clonacionId")
		if clonacionID == "" {
			http.Error(w, "clonacionId requerido", http.StatusBadRequest)
			return
		}
		var (
			tramiteID          string
			usuarioClonadoID   string
			usuarioAsignadorID string
			motivo             string
			estado             string
			createdAt          time.Time
			updatedAt          time.Time
			contadorRechazos   int
		)
		err := opts.DB.QueryRow(`
			SELECT tramite_id, usuario_clonado_id, usuario_asignador_id, motivo, estado, created_at, updated_at, contador_rechazos
			FROM clonaciones
			WHERE id=$1 AND deleted_at IS NULL
		`, clonacionID).Scan(&tramiteID, &usuarioClonadoID, &usuarioAsignadorID, &motivo, &estado, &createdAt, &updatedAt, &contadorRechazos)
		if err == sql.ErrNoRows {
			http.Error(w, "clonación no encontrada", http.StatusNotFound)
			return
		}
		if err != nil {
			opts.Logger.Error("failed to get clonacion", "error", err)
			http.Error(w, "db read error", http.StatusInternalServerError)
			return
		}
		adjuntos, err := fetchAdjuntos(opts.DB, clonacionID)
		if err != nil {
			opts.Logger.Error("failed to get adjuntos", "error", err)
			http.Error(w, "db read error", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"clonacionId":        clonacionID,
			"tramiteId":          tramiteID,
			"usuarioClonadoId":   usuarioClonadoID,
			"usuarioAsignadorId": usuarioAsignadorID,
			"motivo":             motivo,
			"estado":             estado,
			"fechaCreacion":      createdAt.Format(time.RFC3339),
			"adjuntos":           adjuntos,
			"rechazosRealizados": contadorRechazos,
			"maximoRechazos":     2,
		})
	}))

	// Aceptar clonación (por clonacionId)
	r.Method(http.MethodPut, "/clonaciones/{clonacionId}/aceptar", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		clonacionID := chi.URLParam(req, "clonacionId")
		if clonacionID == "" {
			http.Error(w, "clonacionId requerido", http.StatusBadRequest)
			return
		}
		res, err := opts.DB.Exec(`UPDATE clonaciones SET estado=$1, updated_at=$2 WHERE id=$3 AND deleted_at IS NULL`,
			"CLONACION_EN_EDICION", time.Now(), clonacionID)
		if err != nil {
			opts.Logger.Error("failed to update clonacion", "error", err)
			http.Error(w, "db update error", http.StatusInternalServerError)
			return
		}
		rowsAffected, _ := res.RowsAffected()
		if rowsAffected == 0 {
			http.Error(w, "clonación no encontrada", http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, buildDetalleResponse(opts.DB, clonacionID))
	}))

	// Rechazar clonación (por clonacionId)
	r.Method(http.MethodPut, "/clonaciones/{clonacionId}/rechazar", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		clonacionID := chi.URLParam(req, "clonacionId")
		if clonacionID == "" {
			http.Error(w, "clonacionId requerido", http.StatusBadRequest)
			return
		}
		var body struct {
			Motivo string `json:"motivo"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(body.Motivo) == "" {
			http.Error(w, "motivo es obligatorio", http.StatusBadRequest)
			return
		}

		res, err := opts.DB.Exec(`UPDATE clonaciones SET estado=$1, updated_at=$2 WHERE id=$3 AND deleted_at IS NULL`,
			"CLONACION_RECHAZADA", time.Now(), clonacionID)
		if err != nil {
			opts.Logger.Error("failed to update clonacion", "error", err)
			http.Error(w, "db update error", http.StatusInternalServerError)
			return
		}
		rowsAffected, _ := res.RowsAffected()
		if rowsAffected == 0 {
			http.Error(w, "clonación no encontrada", http.StatusNotFound)
			return
		}

		writeJSON(w, http.StatusOK, buildDetalleResponse(opts.DB, clonacionID))
	}))

	// Responder clonación
	r.Method(http.MethodPut, "/clonaciones/{clonacionId}/responder", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		clonacionID := chi.URLParam(req, "clonacionId")
		if clonacionID == "" {
			http.Error(w, "clonacionId requerido", http.StatusBadRequest)
			return
		}
		var body struct {
			Parrafo  string   `json:"parrafo"`
			Adjuntos []string `json:"adjuntos"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(body.Parrafo) == "" {
			http.Error(w, "parrafo es requerido", http.StatusBadRequest)
			return
		}

		parrafoID := uuid.New()
		_, err := opts.DB.Exec(`
			INSERT INTO clonacion_respuestas (id, clonacion_id, usuario_respuesta_id, parrafo, estado_resultado, created_at)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, parrafoID, clonacionID, "00000000-0000-0000-0000-000000000000", body.Parrafo, "ENVIADO", time.Now())
		if err != nil {
			opts.Logger.Error("failed to insert respuesta", "error", err)
			http.Error(w, "db insert error", http.StatusInternalServerError)
			return
		}

		_, err = opts.DB.Exec(`UPDATE clonaciones SET estado=$1, updated_at=$2 WHERE id=$3 AND deleted_at IS NULL`,
			"CLONACION_RESPONDIDA", time.Now(), clonacionID)
		if err != nil {
			opts.Logger.Error("failed to update clonacion", "error", err)
			http.Error(w, "db update error", http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, buildDetalleResponse(opts.DB, clonacionID))
	}))

	// Listar clonaciones por trámite (path)
	r.Method(http.MethodGet, "/clonaciones/tramite/{tramiteId}", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		tramiteID := chi.URLParam(req, "tramiteId")
		if tramiteID == "" {
			http.Error(w, "tramiteId requerido", http.StatusBadRequest)
			return
		}
		rows, err := opts.DB.Query(`
			SELECT id, usuario_clonado_id, estado, created_at, updated_at
			FROM clonaciones
			WHERE tramite_id=$1 AND deleted_at IS NULL
			ORDER BY created_at DESC
		`, tramiteID)
		if err != nil {
			opts.Logger.Error("failed to list clonaciones", "error", err)
			http.Error(w, "db read error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		type clonacionListItem struct {
			ClonacionID        string     `json:"clonacionId"`
			UsuarioClonadoID   string     `json:"usuarioClonadoId"`
			DestinatarioNombre string     `json:"destinatarioNombre"`
			Oficina            string     `json:"oficina"`
			FechaVencimiento   *time.Time `json:"fechaVencimiento"`
			FechaHoraRespuesta *time.Time `json:"fechaHoraRespuesta"`
			Estado             string     `json:"estado"`
			MotivoRechazo      string     `json:"motivoRechazo"`
			CreatedAt          time.Time  `json:"createdAt"`
			UpdatedAt          time.Time  `json:"updatedAt"`
		}

		var result []clonacionListItem
		for rows.Next() {
			var item clonacionListItem
			if err := rows.Scan(&item.ClonacionID, &item.UsuarioClonadoID, &item.Estado, &item.CreatedAt, &item.UpdatedAt); err != nil {
				opts.Logger.Error("failed to scan clonacion", "error", err)
				http.Error(w, "db scan error", http.StatusInternalServerError)
				return
			}
			result = append(result, item)
		}
		if err := rows.Err(); err != nil {
			opts.Logger.Error("failed to iterate clonaciones", "error", err)
			http.Error(w, "db rows error", http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, result)
	}))

	// Aprobar párrafo de una clonación
	r.Method(http.MethodPut, "/clonaciones/{clonacionId}/aprobar-parrafo", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		clonacionID := chi.URLParam(req, "clonacionId")
		if clonacionID == "" {
			http.Error(w, "clonacionId requerido", http.StatusBadRequest)
			return
		}
		var body struct {
			ParrafoID         string `json:"parrafoId"`
			DocumentoSalidaID string `json:"documentoSalidaId"`
			ModoIncorporacion string `json:"modoIncorporacion"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if body.ParrafoID == "" {
			http.Error(w, "parrafoId requerido", http.StatusBadRequest)
			return
		}

		res, err := opts.DB.Exec(`
			UPDATE clonacion_respuestas
			SET estado_resultado=$1
			WHERE id=$2 AND clonacion_id=$3
		`, "APROBADO", body.ParrafoID, clonacionID)
		if err != nil {
			opts.Logger.Error("failed to approve parrafo", "error", err)
			http.Error(w, "db update error", http.StatusInternalServerError)
			return
		}
		rowsAffected, _ := res.RowsAffected()
		if rowsAffected == 0 {
			http.Error(w, "parrafo no encontrado", http.StatusNotFound)
			return
		}

		resp := buildDetalleResponse(opts.DB, clonacionID)
		resp["parrafo"] = map[string]any{
			"parrafoId":         body.ParrafoID,
			"estadoParrafo":     "APROBADO",
			"documentoSalidaId": body.DocumentoSalidaID,
			"modoIncorporacion": body.ModoIncorporacion,
		}
		writeJSON(w, http.StatusOK, resp)
	}))

	// Rechazar párrafo de una clonación
	r.Method(http.MethodPut, "/clonaciones/{clonacionId}/rechazar-parrafo", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		clonacionID := chi.URLParam(req, "clonacionId")
		if clonacionID == "" {
			http.Error(w, "clonacionId requerido", http.StatusBadRequest)
			return
		}
		var body struct {
			ParrafoID string `json:"parrafoId"`
			Motivo    string `json:"motivo"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if body.ParrafoID == "" {
			http.Error(w, "parrafoId requerido", http.StatusBadRequest)
			return
		}

		res, err := opts.DB.Exec(`
			UPDATE clonacion_respuestas
			SET estado_resultado=$1
			WHERE id=$2 AND clonacion_id=$3
		`, "RECHAZADO", body.ParrafoID, clonacionID)
		if err != nil {
			opts.Logger.Error("failed to reject parrafo", "error", err)
			http.Error(w, "db update error", http.StatusInternalServerError)
			return
		}
		rowsAffected, _ := res.RowsAffected()
		if rowsAffected == 0 {
			http.Error(w, "parrafo no encontrado", http.StatusNotFound)
			return
		}

		resp := buildDetalleResponse(opts.DB, clonacionID)
		resp["parrafo"] = map[string]any{
			"parrafoId":     body.ParrafoID,
			"estadoParrafo": "RECHAZADO",
			"motivoRechazo": body.Motivo,
		}
		writeJSON(w, http.StatusOK, resp)
	}))

	// Anular clonación
	r.Method(http.MethodPut, "/clonaciones/{clonacionId}/anular", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		clonacionID := chi.URLParam(req, "clonacionId")
		if clonacionID == "" {
			http.Error(w, "clonacionId requerido", http.StatusBadRequest)
			return
		}
		var body struct {
			Motivo string `json:"motivo"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(body.Motivo) == "" {
			http.Error(w, "motivo es obligatorio", http.StatusBadRequest)
			return
		}

		res, err := opts.DB.Exec(`
			UPDATE clonaciones
			SET estado=$1, updated_at=$2
			WHERE id=$3 AND deleted_at IS NULL
		`, "CLONACION_ANULADA", time.Now(), clonacionID)
		if err != nil {
			opts.Logger.Error("failed to cancel clonacion", "error", err)
			http.Error(w, "db update error", http.StatusInternalServerError)
			return
		}
		rowsAffected, _ := res.RowsAffected()
		if rowsAffected == 0 {
			http.Error(w, "clonación no encontrada", http.StatusNotFound)
			return
		}

		writeJSON(w, http.StatusOK, buildDetalleResponse(opts.DB, clonacionID))
	}))

	// Alertas de clonación
	r.Method(http.MethodGet, "/clonaciones/{clonacionId}/alertas", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		clonacionID := chi.URLParam(req, "clonacionId")
		if clonacionID == "" {
			http.Error(w, "clonacionId requerido", http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"clonacionId": clonacionID,
			"estado":      "CLONACION_CREADA",
			"vencimiento": time.Now().Add(24 * time.Hour).Format(time.RFC3339),
			"alertas":     []any{},
		})
	}))

	// Trazabilidad de clonación
	r.Method(http.MethodGet, "/clonaciones/{clonacionId}/trazabilidad", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		clonacionID := chi.URLParam(req, "clonacionId")
		if clonacionID == "" {
			http.Error(w, "clonacionId requerido", http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, []any{})
	}))

	// Listar usuarios disponibles para clonar
	r.Method(http.MethodGet, "/usuarios/clonar", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// Respuesta de ejemplo vacía; agrega origen real si existe.
		writeJSON(w, http.StatusOK, []map[string]any{})
	}))

	// Aceptar clonación
	r.Method(http.MethodPut, "/clonaciones/{radicadoNumber}/aceptar", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		radicado := chi.URLParam(req, "radicadoNumber")
		if radicado == "" {
			http.Error(w, "radicadoNumber requerido", http.StatusBadRequest)
			return
		}

		_, err := opts.DB.Exec(`UPDATE clonaciones SET estado=$1, updated_at=$2 WHERE tramite_id::text=$3 AND deleted_at IS NULL`,
			"CLONACION_EN_EDICION", time.Now(), radicado)
		if err != nil {
			opts.Logger.Error("failed to update clonacion", "error", err)
			http.Error(w, "db update error", http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"resultado": true, "estado": "CLONACION_EN_EDICION"})
	}))

	// Rechazar clonación (máx 2 rechazos)
	r.Method(http.MethodPut, "/clonaciones/{radicadoNumber}/rechazar", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		radicado := chi.URLParam(req, "radicadoNumber")
		if radicado == "" {
			http.Error(w, "radicadoNumber requerido", http.StatusBadRequest)
			return
		}
		var body struct {
			Motivo string `json:"motivo"`
		}
		if err := json.NewDecoder(req.Body).Decode(&body); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if body.Motivo == "" {
			http.Error(w, "motivo es obligatorio", http.StatusBadRequest)
			return
		}

		// Obtener contador actual y verificar límite
		tx, err := opts.DB.Begin()
		if err != nil {
			opts.Logger.Error("failed to begin tx", "error", err)
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		var contador int
		err = tx.QueryRow(`SELECT contador_rechazos FROM clonaciones WHERE tramite_id::text=$1 AND deleted_at IS NULL FOR UPDATE`, radicado).Scan(&contador)
		if err == sql.ErrNoRows {
			http.Error(w, "clonación no encontrada", http.StatusNotFound)
			return
		}
		if err != nil {
			opts.Logger.Error("failed to query contador", "error", err)
			http.Error(w, "db read error", http.StatusInternalServerError)
			return
		}

		if contador >= 2 {
			http.Error(w, "límite de rechazos alcanzado", http.StatusBadRequest)
			return
		}

		newCount := contador + 1
		_, err = tx.Exec(`UPDATE clonaciones SET estado=$1, contador_rechazos=$2, updated_at=$3 WHERE tramite_id::text=$4 AND deleted_at IS NULL`,
			"CLONACION_RECHAZADA", newCount, time.Now(), radicado)
		if err != nil {
			opts.Logger.Error("failed to update clonacion", "error", err)
			http.Error(w, "db update error", http.StatusInternalServerError)
			return
		}

		if err := tx.Commit(); err != nil {
			opts.Logger.Error("failed to commit tx", "error", err)
			http.Error(w, "db commit error", http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"resultado":          true,
			"estado":             "CLONACION_RECHAZADA",
			"rechazosRealizados": newCount,
		})
	}))

	srv := &http.Server{
		Addr:         opts.Addr,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return &Server{log: opts.Logger, httpServer: srv, db: opts.DB}, nil
}

// Run arranca el servidor hasta que el contexto se cancele.
func (s *Server) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		s.log.Info("HTTP server started", "addr", s.httpServer.Addr)
		if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	select {
	case <-ctx.Done():
		_ = s.httpServer.Shutdown(context.Background())
		return nil
	case err := <-errCh:
		return err
	}
}

// Close cierra recursos (no-op).
func (s *Server) Close() {}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func buildDetalleResponse(db *sql.DB, clonacionID string) map[string]any {
	var (
		tramiteID        string
		usuarioClonadoID string
		motivo           string
		estado           string
		createdAt        time.Time
		contadorRechazos int
	)
	err := db.QueryRow(`
		SELECT tramite_id, usuario_clonado_id, motivo, estado, created_at, contador_rechazos
		FROM clonaciones
		WHERE id=$1 AND deleted_at IS NULL
	`, clonacionID).Scan(&tramiteID, &usuarioClonadoID, &motivo, &estado, &createdAt, &contadorRechazos)
	if err != nil {
		return map[string]any{"clonacionId": clonacionID, "estado": "UNKNOWN"}
	}

	adjuntos, _ := fetchAdjuntos(db, clonacionID)

	return map[string]any{
		"clonacionId":        clonacionID,
		"tramiteId":          tramiteID,
		"usuarioClonadoId":   usuarioClonadoID,
		"motivo":             motivo,
		"estado":             estado,
		"fechaCreacion":      createdAt.Format(time.RFC3339),
		"adjuntos":           adjuntos,
		"rechazosRealizados": contadorRechazos,
		"maximoRechazos":     2,
	}
}

func fetchAdjuntos(db *sql.DB, clonacionID string) ([]string, error) {
	rows, err := db.Query(`SELECT ruta_url FROM clonacion_adjuntos WHERE clonacion_id=$1`, clonacionID)
	if err != nil {
		return []string{}, err
	}
	defer rows.Close()

	var adjuntos []string
	for rows.Next() {
		var ruta string
		if err := rows.Scan(&ruta); err != nil {
			continue
		}
		adjuntos = append(adjuntos, ruta)
	}
	return adjuntos, nil
}
