package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
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

	// Crear clonación (puede ser para múltiples usuarios)
	r.Method(http.MethodPost, "/clonaciones", http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
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
