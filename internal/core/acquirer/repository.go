package acquirer

import "context"

// Repository defines the interface for acquirer persistence operations.
type Repository interface {
	// Create persists a new acquirer and returns its ID.
	Create(ctx context.Context, acquirer Acquirer) (int64, error)

	// Update updates an existing acquirer identified by ofeIdentificacion, adqIdentificacion, and optionally adqIdPersonalizado.
	// If adqIdPersonalizado is empty, it will be treated as NULL in the database.
	Update(ctx context.Context, ofeIdentificacion, adqIdentificacion, adqIdPersonalizado string, acquirer Acquirer) error

	// FindByID retrieves an acquirer by its identifiers.
	// Returns nil if not found.
	FindByID(ctx context.Context, ofeIdentificacion, adqIdentificacion, adqIdPersonalizado string) (*Acquirer, error)

	// Exists checks if an acquirer exists with the given identifiers.
	Exists(ctx context.Context, ofeIdentificacion, adqIdentificacion, adqIdPersonalizado string) (bool, error)

	// Search searches for acquirers by field, value, OFE, and filter type.
	// campoBuscar: field name to search in (e.g., "adq_identificacion", "adq_razon_social")
	// valorBuscar: value to search for
	// valorOfe: OFE identification
	// filtroColumnas: "exacto" for exact match, "basico" for contains match
	Search(ctx context.Context, campoBuscar, valorBuscar, valorOfe, filtroColumnas string) ([]Acquirer, error)

	// List retrieves acquirers with pagination, search, and sorting.
	// start: starting index (0-based)
	// length: number of records to return (-1 for all)
	// buscar: search term to filter by (searches in multiple fields)
	// columnaOrden: column name to sort by
	// ordenDireccion: "asc" or "desc"
	// Returns: list of acquirers, total count, and error
	List(ctx context.Context, start, length int, buscar, columnaOrden, ordenDireccion string) ([]Acquirer, int, error)
}
