package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"3tcapital/ms_facturacion_core/internal/core/dane"
)

const (
	// DANEBaseURL is the base URL for DANE data API on datos.gov.co
	DANEBaseURL = "https://www.datos.gov.co/resource/gdxc-w37w.json"
	// DefaultTimeout is the default timeout for DANE API requests
	DefaultTimeout = 10 * time.Second
)

// Client implements the dane.Service interface using HTTP requests to datos.gov.co
type Client struct {
	baseURL string
	client  *http.Client
	log     *slog.Logger
}

// NewClient creates a new DANE HTTP client.
// If baseURL is empty, uses the default DANE API URL.
func NewClient(baseURL string, httpClient *http.Client, log *slog.Logger) dane.Service {
	if baseURL == "" {
		baseURL = DANEBaseURL
	}
	if httpClient == nil {
		httpClient = &http.Client{
			Timeout: DefaultTimeout,
		}
	}

	return &Client{
		baseURL: baseURL,
		client:  httpClient,
		log:     log,
	}
}

// daneResponse represents the JSON response from DANE API
type daneResponse struct {
	CodDpto       string `json:"cod_dpto"`       // Department code (2 digits)
	Dpto          string `json:"dpto"`           // Department name
	CodMpio       string `json:"cod_mpio"`       // DIVIPOLA code (5 digits)
	NomMpio       string `json:"nom_mpio"`       // Municipality name
	TipoMunicipio string `json:"tipo_municipio"` // Municipality type
	Longitud      string `json:"longitud"`       // Longitude
	Latitud       string `json:"latitud"`        // Latitude
}

// GetMunicipalityByCode retrieves municipality information from DANE API using DIVIPOLA code.
func (c *Client) GetMunicipalityByCode(ctx context.Context, codigoDivipola string) (*dane.Municipality, error) {
	if codigoDivipola == "" {
		return nil, fmt.Errorf("código DIVIPOLA no puede estar vacío")
	}

	// Build URL with query parameter
	apiURL, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	query := apiURL.Query()
	query.Set("cod_mpio", codigoDivipola)
	apiURL.RawQuery = query.Encode()

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Accept", "application/json")

	c.log.Debug("Consulting DANE API", "codigo_divipola", codigoDivipola, "url", apiURL.String())

	// Execute request
	resp, err := c.client.Do(req)
	if err != nil {
		c.log.Warn("Error consulting DANE API", "error", err, "codigo_divipola", codigoDivipola)
		return nil, fmt.Errorf("DANE API request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		c.log.Warn("DANE API returned non-200 status", "status", resp.StatusCode, "body", string(body), "codigo_divipola", codigoDivipola)
		return nil, fmt.Errorf("DANE API returned status %d", resp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}

	// Parse JSON response (array of results)
	var results []daneResponse
	if err := json.Unmarshal(body, &results); err != nil {
		c.log.Warn("Failed to parse DANE API response", "error", err, "body", string(body), "codigo_divipola", codigoDivipola)
		return nil, fmt.Errorf("parse DANE API response: %w", err)
	}

	// Check if any results were returned
	if len(results) == 0 {
		c.log.Warn("Municipality not found in DANE", "codigo_divipola", codigoDivipola)
		return nil, fmt.Errorf("municipio con código DIVIPOLA %s no encontrado en DANE", codigoDivipola)
	}

	// Use first result (should be unique by cod_mpio)
	result := results[0]

	// Validate required fields
	if result.CodMpio == "" || result.NomMpio == "" || result.CodDpto == "" || result.Dpto == "" {
		c.log.Warn("DANE API response missing required fields", "codigo_divipola", codigoDivipola, "result", result)
		return nil, fmt.Errorf("respuesta de DANE API incompleta para código DIVIPOLA %s", codigoDivipola)
	}

	municipality := &dane.Municipality{
		Codigo:    result.CodMpio,
		Nombre:    result.NomMpio,
		DepCodigo: result.CodDpto,
		DepNombre: result.Dpto,
	}

	c.log.Debug("Successfully retrieved municipality from DANE",
		"codigo_divipola", codigoDivipola,
		"municipio", municipality.Nombre,
		"departamento", municipality.DepNombre)

	return municipality, nil
}
