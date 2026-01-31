package provider

import "context"

// Repository defines the interface for provider persistence operations.
type Repository interface {
	// Create persists a new provider and returns its ID.
	Create(ctx context.Context, provider Provider) (int64, error)

	// Update updates an existing provider identified by ofeIdentificacion and proIdentificacion.
	Update(ctx context.Context, ofeIdentificacion, proIdentificacion string, provider Provider) error

	// FindByID retrieves a provider by its identifiers.
	// Returns nil if not found.
	FindByID(ctx context.Context, ofeIdentificacion, proIdentificacion string) (*Provider, error)

	// Exists checks if a provider exists with the given identifiers.
	Exists(ctx context.Context, ofeIdentificacion, proIdentificacion string) (bool, error)

	// List retrieves providers with pagination, search, and sorting.
	// start: starting index (0-based)
	// length: number of records to return (-1 for all)
	// buscar: search term to filter by (searches in multiple fields)
	// columnaOrden: column name to sort by
	// ordenDireccion: "asc" or "desc"
	// Returns: list of providers, total count, and error
	List(ctx context.Context, start, length int, buscar, columnaOrden, ordenDireccion string) ([]Provider, int, error)

	// Search searches for providers by field, value, OFE, and filter type.
	// campoBuscar: field name to search in (e.g., "pro_identificacion", "pro_razon_social")
	// valorBuscar: value to search for
	// valorOfe: OFE identification
	// filtroColumnas: "exacto" for exact match, "basico" for contains match
	Search(ctx context.Context, campoBuscar, valorBuscar, valorOfe, filtroColumnas string) ([]Provider, error)
}
