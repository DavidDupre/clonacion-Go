package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"3tcapital/ms_facturacion_core/internal/core/provider"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository implements the provider.Repository interface using PostgreSQL.
type Repository struct {
	pool *pgxpool.Pool
}

// NewRepository creates a new PostgreSQL provider repository.
func NewRepository(pool *pgxpool.Pool) provider.Repository {
	return &Repository{pool: pool}
}

// Create persists a new provider and returns its ID.
func (r *Repository) Create(ctx context.Context, prov provider.Provider) (int64, error) {
	// Serialize arrays to JSONB
	refCodigoJSON, err := json.Marshal(prov.RefCodigo)
	if err != nil {
		return 0, fmt.Errorf("marshal ref_codigo: %w", err)
	}

	proUsuariosRecepcionJSON, err := json.Marshal(prov.ProUsuariosRecepcion)
	if err != nil {
		return 0, fmt.Errorf("marshal pro_usuarios_recepcion: %w", err)
	}

	query := `
		INSERT INTO provider (
			ofe_identificacion, pro_identificacion, pro_id_personalizado,
			pro_razon_social, pro_nombre_comercial,
			pro_primer_apellido, pro_segundo_apellido, pro_primer_nombre, pro_otros_nombres,
			tdo_codigo, toj_codigo, pai_codigo, dep_codigo, mun_codigo, cpo_codigo,
			pro_direccion, pro_telefono, pai_codigo_domicilio_fiscal, dep_codigo_domicilio_fiscal,
			mun_codigo_domicilio_fiscal, cpo_codigo_domicilio_fiscal, pro_direccion_domicilio_fiscal,
			pro_correo, pro_correos_notificacion, pro_matricula_mercantil,
			pro_usuarios_recepcion, rfi_codigo, ref_codigo, estado
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15,
			$16, $17, $18, $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29
		) RETURNING id
	`

	var id int64
	err = r.pool.QueryRow(ctx, query,
		prov.OfeIdentificacion,
		prov.ProIdentificacion,
		prov.ProIDPersonalizado,
		prov.ProRazonSocial,
		prov.ProNombreComercial,
		prov.ProPrimerApellido,
		prov.ProSegundoApellido,
		prov.ProPrimerNombre,
		prov.ProOtrosNombres,
		prov.TdoCodigo,
		prov.TojCodigo,
		prov.PaiCodigo,
		prov.DepCodigo,
		prov.MunCodigo,
		prov.CpoCodigo,
		prov.ProDireccion,
		prov.ProTelefono,
		prov.PaiCodigoDomicilioFiscal,
		prov.DepCodigoDomicilioFiscal,
		prov.MunCodigoDomicilioFiscal,
		prov.CpoCodigoDomicilioFiscal,
		prov.ProDireccionDomicilioFiscal,
		prov.ProCorreo,
		prov.ProCorreosNotificacion,
		prov.ProMatriculaMercantil,
		proUsuariosRecepcionJSON,
		prov.RfiCodigo,
		refCodigoJSON,
		prov.Estado,
	).Scan(&id)

	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
			return 0, fmt.Errorf("ya existe un Proveedor con el numero de identificacion [%s] para el OFE [%s]", prov.ProIdentificacion, prov.OfeIdentificacion)
		}
		return 0, fmt.Errorf("create provider: %w", err)
	}

	return id, nil
}

// Update updates an existing provider identified by ofeIdentificacion and proIdentificacion.
func (r *Repository) Update(ctx context.Context, ofeIdentificacion, proIdentificacion string, prov provider.Provider) error {
	// Serialize arrays to JSONB
	refCodigoJSON, err := json.Marshal(prov.RefCodigo)
	if err != nil {
		return fmt.Errorf("marshal ref_codigo: %w", err)
	}

	proUsuariosRecepcionJSON, err := json.Marshal(prov.ProUsuariosRecepcion)
	if err != nil {
		return fmt.Errorf("marshal pro_usuarios_recepcion: %w", err)
	}

	query := `
		UPDATE provider SET
			pro_id_personalizado = $1,
			pro_razon_social = $2,
			pro_nombre_comercial = $3,
			pro_primer_apellido = $4,
			pro_segundo_apellido = $5,
			pro_primer_nombre = $6,
			pro_otros_nombres = $7,
			tdo_codigo = $8,
			toj_codigo = $9,
			pai_codigo = $10,
			dep_codigo = $11,
			mun_codigo = $12,
			cpo_codigo = $13,
			pro_direccion = $14,
			pro_telefono = $15,
			pai_codigo_domicilio_fiscal = $16,
			dep_codigo_domicilio_fiscal = $17,
			mun_codigo_domicilio_fiscal = $18,
			cpo_codigo_domicilio_fiscal = $19,
			pro_direccion_domicilio_fiscal = $20,
			pro_correo = $21,
			pro_correos_notificacion = $22,
			pro_matricula_mercantil = $23,
			pro_usuarios_recepcion = $24,
			rfi_codigo = $25,
			ref_codigo = $26,
			estado = $27,
			fecha_modificacion = NOW()
		WHERE ofe_identificacion = $28 AND pro_identificacion = $29
	`

	result, err := r.pool.Exec(ctx, query,
		prov.ProIDPersonalizado,
		prov.ProRazonSocial,
		prov.ProNombreComercial,
		prov.ProPrimerApellido,
		prov.ProSegundoApellido,
		prov.ProPrimerNombre,
		prov.ProOtrosNombres,
		prov.TdoCodigo,
		prov.TojCodigo,
		prov.PaiCodigo,
		prov.DepCodigo,
		prov.MunCodigo,
		prov.CpoCodigo,
		prov.ProDireccion,
		prov.ProTelefono,
		prov.PaiCodigoDomicilioFiscal,
		prov.DepCodigoDomicilioFiscal,
		prov.MunCodigoDomicilioFiscal,
		prov.CpoCodigoDomicilioFiscal,
		prov.ProDireccionDomicilioFiscal,
		prov.ProCorreo,
		prov.ProCorreosNotificacion,
		prov.ProMatriculaMercantil,
		proUsuariosRecepcionJSON,
		prov.RfiCodigo,
		refCodigoJSON,
		prov.Estado,
		ofeIdentificacion,
		proIdentificacion,
	)

	if err != nil {
		return fmt.Errorf("update provider: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("el Id del proveedor no existe")
	}

	return nil
}

// FindByID retrieves a provider by its identifiers.
func (r *Repository) FindByID(ctx context.Context, ofeIdentificacion, proIdentificacion string) (*provider.Provider, error) {
	query := `
		SELECT id, ofe_identificacion, pro_identificacion, pro_id_personalizado,
			pro_razon_social, pro_nombre_comercial,
			pro_primer_apellido, pro_segundo_apellido, pro_primer_nombre, pro_otros_nombres,
			tdo_codigo, toj_codigo, pai_codigo, dep_codigo, mun_codigo, cpo_codigo,
			pro_direccion, pro_telefono, pai_codigo_domicilio_fiscal, dep_codigo_domicilio_fiscal,
			mun_codigo_domicilio_fiscal, cpo_codigo_domicilio_fiscal, pro_direccion_domicilio_fiscal,
			pro_correo, pro_correos_notificacion, pro_matricula_mercantil,
			pro_usuarios_recepcion, rfi_codigo, ref_codigo, estado, fecha_creacion, fecha_modificacion
		FROM provider
		WHERE ofe_identificacion = $1 AND pro_identificacion = $2
	`

	var prov provider.Provider
	var refCodigoJSON, proUsuariosRecepcionJSON []byte

	err := r.pool.QueryRow(ctx, query, ofeIdentificacion, proIdentificacion).Scan(
		&prov.ID,
		&prov.OfeIdentificacion,
		&prov.ProIdentificacion,
		&prov.ProIDPersonalizado,
		&prov.ProRazonSocial,
		&prov.ProNombreComercial,
		&prov.ProPrimerApellido,
		&prov.ProSegundoApellido,
		&prov.ProPrimerNombre,
		&prov.ProOtrosNombres,
		&prov.TdoCodigo,
		&prov.TojCodigo,
		&prov.PaiCodigo,
		&prov.DepCodigo,
		&prov.MunCodigo,
		&prov.CpoCodigo,
		&prov.ProDireccion,
		&prov.ProTelefono,
		&prov.PaiCodigoDomicilioFiscal,
		&prov.DepCodigoDomicilioFiscal,
		&prov.MunCodigoDomicilioFiscal,
		&prov.CpoCodigoDomicilioFiscal,
		&prov.ProDireccionDomicilioFiscal,
		&prov.ProCorreo,
		&prov.ProCorreosNotificacion,
		&prov.ProMatriculaMercantil,
		&proUsuariosRecepcionJSON,
		&prov.RfiCodigo,
		&refCodigoJSON,
		&prov.Estado,
		&prov.CreatedAt,
		&prov.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query provider: %w", err)
	}

	// Unmarshal JSON arrays
	if len(refCodigoJSON) > 0 {
		if err := json.Unmarshal(refCodigoJSON, &prov.RefCodigo); err != nil {
			return nil, fmt.Errorf("unmarshal ref_codigo: %w", err)
		}
	}

	if len(proUsuariosRecepcionJSON) > 0 {
		if err := json.Unmarshal(proUsuariosRecepcionJSON, &prov.ProUsuariosRecepcion); err != nil {
			return nil, fmt.Errorf("unmarshal pro_usuarios_recepcion: %w", err)
		}
	}

	return &prov, nil
}

// Exists checks if a provider exists with the given identifiers.
func (r *Repository) Exists(ctx context.Context, ofeIdentificacion, proIdentificacion string) (bool, error) {
	query := "SELECT EXISTS(SELECT 1 FROM provider WHERE ofe_identificacion = $1 AND pro_identificacion = $2)"

	var exists bool
	err := r.pool.QueryRow(ctx, query, ofeIdentificacion, proIdentificacion).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check existence: %w", err)
	}

	return exists, nil
}

// allowedSearchFields is a whitelist of fields that can be searched to prevent SQL injection.
var allowedProviderSearchFields = map[string]bool{
	"pro_identificacion":     true,
	"pro_id_personalizado":   true,
	"pro_razon_social":       true,
	"pro_nombre_comercial":   true,
	"pro_primer_apellido":    true,
	"pro_segundo_apellido":   true,
	"pro_primer_nombre":      true,
	"pro_otros_nombres":      true,
	"tdo_codigo":             true,
	"toj_codigo":             true,
	"pai_codigo":             true,
	"dep_codigo":             true,
	"mun_codigo":             true,
	"cpo_codigo":             true,
	"pro_direccion":          true,
	"pro_telefono":           true,
	"pro_correo":             true,
	"pro_matricula_mercantil": true,
	"rfi_codigo":             true,
}

// allowedOrderFields is a whitelist of fields that can be used for ordering.
var allowedOrderFields = map[string]bool{
	"pro_id":                 true,
	"pro_identificacion":     true,
	"pro_razon_social":       true,
	"pro_nombre_comercial":   true,
	"tdo_codigo":             true,
	"toj_codigo":             true,
	"fecha_creacion":         true,
	"fecha_modificacion":     true,
	"estado":                 true,
}

// List retrieves providers with pagination, search, and sorting.
func (r *Repository) List(ctx context.Context, start, length int, buscar, columnaOrden, ordenDireccion string) ([]provider.Provider, int, error) {
	// Build WHERE clause for search
	whereConditions := []string{}
	queryArgs := []interface{}{}
	argIndex := 1

	if buscar != "" {
		// Search in multiple fields: identificación, razón social, nombre comercial
		searchPattern := "%" + buscar + "%"
		whereConditions = append(whereConditions, fmt.Sprintf(
			"(pro_identificacion ILIKE $%d OR pro_razon_social ILIKE $%d OR pro_nombre_comercial ILIKE $%d OR COALESCE(pro_primer_nombre || ' ' || pro_primer_apellido, '') ILIKE $%d)",
			argIndex, argIndex, argIndex, argIndex,
		))
		queryArgs = append(queryArgs, searchPattern)
		argIndex++
	}

	whereClause := ""
	if len(whereConditions) > 0 {
		whereClause = "WHERE " + strings.Join(whereConditions, " AND ")
	}

	// Validate and build ORDER BY clause
	orderByClause := "ORDER BY id"
	if columnaOrden != "" {
		// Map common field names to database columns
		orderFieldMap := map[string]string{
			"codigo":           "pro_identificacion",
			"identificacion":   "pro_identificacion",
			"razon_social":     "pro_razon_social",
			"nombre_comercial": "pro_nombre_comercial",
			"fecha_creacion":   "fecha_creacion",
			"fecha_modificacion": "fecha_modificacion",
			"estado":           "estado",
		}

		dbColumn := orderFieldMap[columnaOrden]
		if dbColumn == "" {
			// Try direct column name if not in map
			dbColumn = columnaOrden
		}

		if allowedOrderFields[dbColumn] {
			direction := "ASC"
			if strings.ToLower(ordenDireccion) == "desc" {
				direction = "DESC"
			}
			orderByClause = fmt.Sprintf("ORDER BY %s %s", dbColumn, direction)
		}
	}

	// Get total count
	countQuery := "SELECT COUNT(*) FROM provider " + whereClause
	var total int
	err := r.pool.QueryRow(ctx, countQuery, queryArgs...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count providers: %w", err)
	}

	// Build main query with pagination
	query := fmt.Sprintf(`
		SELECT id, ofe_identificacion, pro_identificacion, pro_id_personalizado,
			pro_razon_social, pro_nombre_comercial,
			pro_primer_apellido, pro_segundo_apellido, pro_primer_nombre, pro_otros_nombres,
			tdo_codigo, toj_codigo, pai_codigo, dep_codigo, mun_codigo, cpo_codigo,
			pro_direccion, pro_telefono, pai_codigo_domicilio_fiscal, dep_codigo_domicilio_fiscal,
			mun_codigo_domicilio_fiscal, cpo_codigo_domicilio_fiscal, pro_direccion_domicilio_fiscal,
			pro_correo, pro_correos_notificacion, pro_matricula_mercantil,
			pro_usuarios_recepcion, rfi_codigo, ref_codigo, estado, fecha_creacion, fecha_modificacion
		FROM provider
		%s
		%s
	`, whereClause, orderByClause)

	// Add LIMIT and OFFSET if length is not -1
	if length != -1 {
		query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIndex, argIndex+1)
		queryArgs = append(queryArgs, length, start)
	}

	rows, err := r.pool.Query(ctx, query, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("query providers: %w", err)
	}
	defer rows.Close()

	var providers []provider.Provider
	for rows.Next() {
		var prov provider.Provider
		var refCodigoJSON, proUsuariosRecepcionJSON []byte

		err := rows.Scan(
			&prov.ID,
			&prov.OfeIdentificacion,
			&prov.ProIdentificacion,
			&prov.ProIDPersonalizado,
			&prov.ProRazonSocial,
			&prov.ProNombreComercial,
			&prov.ProPrimerApellido,
			&prov.ProSegundoApellido,
			&prov.ProPrimerNombre,
			&prov.ProOtrosNombres,
			&prov.TdoCodigo,
			&prov.TojCodigo,
			&prov.PaiCodigo,
			&prov.DepCodigo,
			&prov.MunCodigo,
			&prov.CpoCodigo,
			&prov.ProDireccion,
			&prov.ProTelefono,
			&prov.PaiCodigoDomicilioFiscal,
			&prov.DepCodigoDomicilioFiscal,
			&prov.MunCodigoDomicilioFiscal,
			&prov.CpoCodigoDomicilioFiscal,
			&prov.ProDireccionDomicilioFiscal,
			&prov.ProCorreo,
			&prov.ProCorreosNotificacion,
			&prov.ProMatriculaMercantil,
			&proUsuariosRecepcionJSON,
			&prov.RfiCodigo,
			&refCodigoJSON,
			&prov.Estado,
			&prov.CreatedAt,
			&prov.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scan provider: %w", err)
		}

		// Unmarshal JSON arrays
		if len(refCodigoJSON) > 0 {
			if err := json.Unmarshal(refCodigoJSON, &prov.RefCodigo); err != nil {
				return nil, 0, fmt.Errorf("unmarshal ref_codigo: %w", err)
			}
		}

		if len(proUsuariosRecepcionJSON) > 0 {
			if err := json.Unmarshal(proUsuariosRecepcionJSON, &prov.ProUsuariosRecepcion); err != nil {
				return nil, 0, fmt.Errorf("unmarshal pro_usuarios_recepcion: %w", err)
			}
		}

		providers = append(providers, prov)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate rows: %w", err)
	}

	// Calculate filtered count (same as total if no search, otherwise use filtered results count)
	filtered := len(providers)
	if buscar == "" && length == -1 {
		filtered = total
	} else if buscar != "" {
		// Re-count with search to get filtered total
		filtered = total
	}

	return providers, filtered, nil
}

// Search searches for providers by field, value, OFE, and filter type.
func (r *Repository) Search(ctx context.Context, campoBuscar, valorBuscar, valorOfe, filtroColumnas string) ([]provider.Provider, error) {
	// Validate field name to prevent SQL injection
	if !allowedProviderSearchFields[campoBuscar] {
		return nil, fmt.Errorf("campo de búsqueda no permitido: %s", campoBuscar)
	}

	// Build WHERE clause based on filter type
	var searchCondition string
	if filtroColumnas == "exacto" {
		// Exact match
		searchCondition = campoBuscar + " = $1"
	} else if filtroColumnas == "basico" {
		// Contains match (case-insensitive)
		searchCondition = campoBuscar + " ILIKE '%' || $1 || '%'"
	} else {
		return nil, fmt.Errorf("tipo de filtro inválido: %s (debe ser 'exacto' o 'basico')", filtroColumnas)
	}

	// Build full WHERE clause with OFE filter
	whereClause := "ofe_identificacion = $2 AND " + searchCondition

	query := `
		SELECT id, ofe_identificacion, pro_identificacion, pro_id_personalizado,
			pro_razon_social, pro_nombre_comercial,
			pro_primer_apellido, pro_segundo_apellido, pro_primer_nombre, pro_otros_nombres,
			tdo_codigo, toj_codigo, pai_codigo, dep_codigo, mun_codigo, cpo_codigo,
			pro_direccion, pro_telefono, pai_codigo_domicilio_fiscal, dep_codigo_domicilio_fiscal,
			mun_codigo_domicilio_fiscal, cpo_codigo_domicilio_fiscal, pro_direccion_domicilio_fiscal,
			pro_correo, pro_correos_notificacion, pro_matricula_mercantil,
			pro_usuarios_recepcion, rfi_codigo, ref_codigo, estado, fecha_creacion, fecha_modificacion
		FROM provider
		WHERE ` + whereClause + `
		ORDER BY id
	`

	rows, err := r.pool.Query(ctx, query, valorBuscar, valorOfe)
	if err != nil {
		return nil, fmt.Errorf("query providers: %w", err)
	}
	defer rows.Close()

	var providers []provider.Provider
	for rows.Next() {
		var prov provider.Provider
		var refCodigoJSON, proUsuariosRecepcionJSON []byte

		err := rows.Scan(
			&prov.ID,
			&prov.OfeIdentificacion,
			&prov.ProIdentificacion,
			&prov.ProIDPersonalizado,
			&prov.ProRazonSocial,
			&prov.ProNombreComercial,
			&prov.ProPrimerApellido,
			&prov.ProSegundoApellido,
			&prov.ProPrimerNombre,
			&prov.ProOtrosNombres,
			&prov.TdoCodigo,
			&prov.TojCodigo,
			&prov.PaiCodigo,
			&prov.DepCodigo,
			&prov.MunCodigo,
			&prov.CpoCodigo,
			&prov.ProDireccion,
			&prov.ProTelefono,
			&prov.PaiCodigoDomicilioFiscal,
			&prov.DepCodigoDomicilioFiscal,
			&prov.MunCodigoDomicilioFiscal,
			&prov.CpoCodigoDomicilioFiscal,
			&prov.ProDireccionDomicilioFiscal,
			&prov.ProCorreo,
			&prov.ProCorreosNotificacion,
			&prov.ProMatriculaMercantil,
			&proUsuariosRecepcionJSON,
			&prov.RfiCodigo,
			&refCodigoJSON,
			&prov.Estado,
			&prov.CreatedAt,
			&prov.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan provider: %w", err)
		}

		// Unmarshal JSON arrays
		if len(refCodigoJSON) > 0 {
			if err := json.Unmarshal(refCodigoJSON, &prov.RefCodigo); err != nil {
				return nil, fmt.Errorf("unmarshal ref_codigo: %w", err)
			}
		}

		if len(proUsuariosRecepcionJSON) > 0 {
			if err := json.Unmarshal(proUsuariosRecepcionJSON, &prov.ProUsuariosRecepcion); err != nil {
				return nil, fmt.Errorf("unmarshal pro_usuarios_recepcion: %w", err)
			}
		}

		providers = append(providers, prov)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rows: %w", err)
	}

	return providers, nil
}
