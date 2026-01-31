package dane

import "context"

// Service defines the interface for querying DANE (Departamento Administrativo Nacional de Estadística) data.
// DANE provides official municipality and department information based on DIVIPOLA codes.
type Service interface {
	// GetMunicipalityByCode retrieves municipality information from DANE API
	// based on the DIVIPOLA code (concatenation of dep_codigo + mun_codigo, e.g., "05001").
	// The DIVIPOLA code is formed by: dep_codigo (2 digits) + mun_codigo (3 digits, zero-padded).
	// Returns the municipality name, department code, and department name.
	GetMunicipalityByCode(ctx context.Context, codigoDivipola string) (*Municipality, error)
}

// Municipality represents municipality information from DANE.
type Municipality struct {
	Codigo    string // Código DIVIPOLA (5 dígitos, e.g., "05001")
	Nombre    string // Nombre del municipio (e.g., "MEDELLÍN")
	DepCodigo string // Código del departamento (2 dígitos, e.g., "05")
	DepNombre string // Nombre del departamento (e.g., "ANTIOQUIA")
}
