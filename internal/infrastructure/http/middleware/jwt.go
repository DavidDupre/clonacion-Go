package middleware

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"

	"3tcapital/ms_facturacion_core/internal/infrastructure/config"
	httperrors "3tcapital/ms_facturacion_core/internal/infrastructure/http"
)

// ContextKeyToken exposes the verified JWT token via request context.
type ContextKeyToken struct{}

// JWTAuthenticator validates Authorization headers against a remote JWKS.
type JWTAuthenticator struct {
	cfg        config.AuthSettings
	log        *slog.Logger
	jwks       keyfunc.Keyfunc
	cancel     context.CancelFunc
	bypassPath map[string]struct{}
}

func NewJWTAuthenticator(cfg config.AuthSettings, log *slog.Logger) (*JWTAuthenticator, error) {
	auth := &JWTAuthenticator{
		cfg:        cfg,
		log:        log,
		bypassPath: make(map[string]struct{}),
	}

	for _, path := range cfg.BypassPaths {
		if path != "" {
			auth.bypassPath[path] = struct{}{}
		}
	}

	if !cfg.Enabled {
		return auth, nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	override := keyfunc.Override{
		RefreshInterval: 6 * time.Hour,
		RefreshErrorHandlerFunc: func(url string) func(context.Context, error) {
			return func(c context.Context, err error) {
				log.Error("failed to refresh JWKS", "url", url, "error", err)
			}
		},
		HTTPTimeout: 10 * time.Second,
	}

	jwks, err := keyfunc.NewDefaultOverrideCtx(ctx, []string{cfg.JWKSetURI}, override)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("unable to load JWKS: %w", err)
	}
	auth.jwks = jwks
	auth.cancel = cancel

	return auth, nil
}

// Middleware enforces JWT validation on inbound requests.
func (a *JWTAuthenticator) Middleware(next http.Handler) http.Handler {
	if !a.cfg.Enabled {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if a.shouldBypass(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		tokenString, err := extractBearerToken(r.Header.Get("Authorization"))
		if err != nil {
			httperrors.WriteError(w, http.StatusUnauthorized, "Error de Autenticación", []string{"Credenciales de acceso no válidas"}, a.log)
			return
		}

		token, err := jwt.Parse(tokenString, a.jwks.Keyfunc,
			jwt.WithIssuer(a.cfg.IssuerURI),
			jwt.WithLeeway(a.cfg.ClockSkew),
			jwt.WithValidMethods([]string{
				jwt.SigningMethodRS256.Alg(),
				jwt.SigningMethodRS384.Alg(),
				jwt.SigningMethodRS512.Alg(),
				jwt.SigningMethodPS256.Alg(),
				jwt.SigningMethodES256.Alg(),
			}),
		)
		if err != nil || !token.Valid {
			a.log.Warn("token validation failed", "error", err)
			httperrors.WriteError(w, http.StatusUnauthorized, "Error de Autenticación", []string{"Token inválido o expirado"}, a.log)
			return
		}

		ctx := context.WithValue(r.Context(), ContextKeyToken{}, token)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Close stops background JWKS refreshers.
func (a *JWTAuthenticator) Close() {
	if a.cancel != nil {
		a.cancel()
	}
}

func (a *JWTAuthenticator) shouldBypass(path string) bool {
	_, ok := a.bypassPath[path]
	return ok
}

func extractBearerToken(header string) (string, error) {
	if header == "" {
		return "", errors.New("missing Authorization header")
	}

	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", errors.New("invalid Authorization header format")
	}
	return parts[1], nil
}
