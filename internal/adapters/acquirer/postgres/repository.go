package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"3tcapital/ms_facturacion_core/internal/core/acquirer"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository implements the acquirer.Repository interface using PostgreSQL.
type Repository struct {
	pool *pgxpool.Pool
	log  *slog.Logger
}

// NewRepository creates a new PostgreSQL acquirer repository.
func NewRepository(pool *pgxpool.Pool, log *slog.Logger) acquirer.Repository {
	return &Repository{pool: pool, log: log}
}

// Create persists a new acquirer and returns its ID.
func (r *Repository) Create(ctx context.Context, acq acquirer.Acquirer) (int64, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Serialize arrays to JSONB
	refCodigoJSON, err := json.Marshal(acq.RefCodigo)
	if err != nil {
		return 0, fmt.Errorf("marshal ref_codigo: %w", err)
	}

	responsableTributosJSON, err := json.Marshal(acq.ResponsableTributos)
	if err != nil {
		return 0, fmt.Errorf("marshal responsable_tributos: %w", err)
	}

	var adqInformacionPersonalizadaJSON interface{}
	if acq.AdqInformacionPersonalizada != nil && *acq.AdqInformacionPersonalizada != "" {
		// Parse the JSON string to validate it
		var jsonObj interface{}
		if err := json.Unmarshal([]byte(*acq.AdqInformacionPersonalizada), &jsonObj); err != nil {
			return 0, fmt.Errorf("invalid adq_informacion_personalizada JSON: %w", err)
		}
		adqInformacionPersonalizadaJSON = jsonObj
	}

	query := `
		INSERT INTO acquirer (
			ofe_identificacion, adq_identificacion, adq_tipo_adquirente, adq_id_personalizado,
			adq_informacion_personalizada, adq_razon_social, adq_nombre_comercial,
			adq_primer_apellido, adq_segundo_apellido, adq_primer_nombre, adq_otros_nombres,
			tdo_codigo, toj_codigo, pai_codigo, dep_codigo, dep_nombre, mun_codigo, mun_nombre, cpo_codigo,
			adq_direccion, adq_telefono, pai_codigo_domicilio_fiscal, dep_codigo_domicilio_fiscal,
			dep_nombre_domicilio_fiscal, mun_codigo_domicilio_fiscal, mun_nombre_domicilio_fiscal,
			cpo_codigo_domicilio_fiscal, adq_direccion_domicilio_fiscal,
			adq_nombre_contacto, adq_fax, adq_notas, adq_correo, adq_matricula_mercantil,
			adq_correos_notificacion, rfi_codigo, ref_codigo, responsable_tributos
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19,
			$20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31, $32, $33, $34, $35, $36, $37
		) RETURNING id
	`

	var id int64
	err = tx.QueryRow(ctx, query,
		acq.OfeIdentificacion,
		acq.AdqIdentificacion,
		acq.AdqTipoAdquirente,
		acq.AdqIDPersonalizado,
		adqInformacionPersonalizadaJSON,
		acq.AdqRazonSocial,
		acq.AdqNombreComercial,
		acq.AdqPrimerApellido,
		acq.AdqSegundoApellido,
		acq.AdqPrimerNombre,
		acq.AdqOtrosNombres,
		acq.TdoCodigo,
		acq.TojCodigo,
		acq.PaiCodigo,
		acq.DepCodigo,
		acq.DepNombre,
		acq.MunCodigo,
		acq.MunNombre,
		acq.CpoCodigo,
		acq.AdqDireccion,
		acq.AdqTelefono,
		acq.PaiCodigoDomicilioFiscal,
		acq.DepCodigoDomicilioFiscal,
		acq.DepNombreDomicilioFiscal,
		acq.MunCodigoDomicilioFiscal,
		acq.MunNombreDomicilioFiscal,
		acq.CpoCodigoDomicilioFiscal,
		acq.AdqDireccionDomicilioFiscal,
		acq.AdqNombreContacto,
		acq.AdqFax,
		acq.AdqNotas,
		acq.AdqCorreo,
		acq.AdqMatriculaMercantil,
		acq.AdqCorreosNotificacion,
		acq.RfiCodigo,
		refCodigoJSON,
		responsableTributosJSON,
	).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("insert acquirer: %w", err)
	}

	// Insert contacts
	if len(acq.Contactos) > 0 {
		contactQuery := `
			INSERT INTO acquirer_contact (
				acquirer_id, con_nombre, con_direccion, con_telefono,
				con_correo, con_observaciones, con_tipo
			) VALUES ($1, $2, $3, $4, $5, $6, $7)
		`

		for _, contact := range acq.Contactos {
			_, err = tx.Exec(ctx, contactQuery,
				id,
				contact.Nombre,
				contact.Direccion,
				contact.Telefono,
				contact.Correo,
				contact.Observaciones,
				contact.Tipo,
			)
			if err != nil {
				return 0, fmt.Errorf("insert contact: %w", err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit transaction: %w", err)
	}

	return id, nil
}

// Update updates an existing acquirer.
func (r *Repository) Update(ctx context.Context, ofeIdentificacion, adqIdentificacion, adqIdPersonalizado string, acq acquirer.Acquirer) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Serialize arrays to JSONB
	refCodigoJSON, err := json.Marshal(acq.RefCodigo)
	if err != nil {
		return fmt.Errorf("marshal ref_codigo: %w", err)
	}

	responsableTributosJSON, err := json.Marshal(acq.ResponsableTributos)
	if err != nil {
		return fmt.Errorf("marshal responsable_tributos: %w", err)
	}

	var adqInformacionPersonalizadaJSON interface{}
	if acq.AdqInformacionPersonalizada != nil && *acq.AdqInformacionPersonalizada != "" {
		var jsonObj interface{}
		if err := json.Unmarshal([]byte(*acq.AdqInformacionPersonalizada), &jsonObj); err != nil {
			return fmt.Errorf("invalid adq_informacion_personalizada JSON: %w", err)
		}
		adqInformacionPersonalizadaJSON = jsonObj
	}

	// Build WHERE clause based on adqIdPersonalizado
	var whereClause string
	var queryArgs []interface{}
	argIndex := 1

	if adqIdPersonalizado == "" {
		whereClause = "ofe_identificacion = $" + fmt.Sprintf("%d", argIndex) + " AND adq_identificacion = $" + fmt.Sprintf("%d", argIndex+1) + " AND (adq_id_personalizado IS NULL OR adq_id_personalizado = '')"
		queryArgs = []interface{}{ofeIdentificacion, adqIdentificacion}
		argIndex = 3
	} else {
		whereClause = "ofe_identificacion = $" + fmt.Sprintf("%d", argIndex) + " AND adq_identificacion = $" + fmt.Sprintf("%d", argIndex+1) + " AND adq_id_personalizado = $" + fmt.Sprintf("%d", argIndex+2)
		queryArgs = []interface{}{ofeIdentificacion, adqIdentificacion, adqIdPersonalizado}
		argIndex = 4
	}

	query := fmt.Sprintf(`
		UPDATE acquirer SET
			adq_tipo_adquirente = $%d,
			adq_id_personalizado = $%d,
			adq_informacion_personalizada = $%d,
			adq_razon_social = $%d,
			adq_nombre_comercial = $%d,
			adq_primer_apellido = $%d,
			adq_segundo_apellido = $%d,
			adq_primer_nombre = $%d,
			adq_otros_nombres = $%d,
			tdo_codigo = $%d,
			toj_codigo = $%d,
			pai_codigo = $%d,
			dep_codigo = $%d,
			dep_nombre = $%d,
			mun_codigo = $%d,
			mun_nombre = $%d,
			cpo_codigo = $%d,
			adq_direccion = $%d,
			adq_telefono = $%d,
			pai_codigo_domicilio_fiscal = $%d,
			dep_codigo_domicilio_fiscal = $%d,
			dep_nombre_domicilio_fiscal = $%d,
			mun_codigo_domicilio_fiscal = $%d,
			mun_nombre_domicilio_fiscal = $%d,
			cpo_codigo_domicilio_fiscal = $%d,
			adq_direccion_domicilio_fiscal = $%d,
			adq_nombre_contacto = $%d,
			adq_fax = $%d,
			adq_notas = $%d,
			adq_correo = $%d,
			adq_matricula_mercantil = $%d,
			adq_correos_notificacion = $%d,
			rfi_codigo = $%d,
			ref_codigo = $%d,
			responsable_tributos = $%d,
			updated_at = NOW()
		WHERE %s
	`, argIndex, argIndex+1, argIndex+2, argIndex+3, argIndex+4, argIndex+5, argIndex+6, argIndex+7, argIndex+8, argIndex+9, argIndex+10, argIndex+11, argIndex+12, argIndex+13, argIndex+14, argIndex+15, argIndex+16, argIndex+17, argIndex+18, argIndex+19, argIndex+20, argIndex+21, argIndex+22, argIndex+23, argIndex+24, argIndex+25, argIndex+26, argIndex+27, argIndex+28, argIndex+29, argIndex+30, argIndex+31, argIndex+32, argIndex+33, argIndex+34, whereClause)

	allArgs := append(queryArgs,
		acq.AdqTipoAdquirente,
		acq.AdqIDPersonalizado,
		adqInformacionPersonalizadaJSON,
		acq.AdqRazonSocial,
		acq.AdqNombreComercial,
		acq.AdqPrimerApellido,
		acq.AdqSegundoApellido,
		acq.AdqPrimerNombre,
		acq.AdqOtrosNombres,
		acq.TdoCodigo,
		acq.TojCodigo,
		acq.PaiCodigo,
		acq.DepCodigo,
		acq.DepNombre,
		acq.MunCodigo,
		acq.MunNombre,
		acq.CpoCodigo,
		acq.AdqDireccion,
		acq.AdqTelefono,
		acq.PaiCodigoDomicilioFiscal,
		acq.DepCodigoDomicilioFiscal,
		acq.DepNombreDomicilioFiscal,
		acq.MunCodigoDomicilioFiscal,
		acq.MunNombreDomicilioFiscal,
		acq.CpoCodigoDomicilioFiscal,
		acq.AdqDireccionDomicilioFiscal,
		acq.AdqNombreContacto,
		acq.AdqFax,
		acq.AdqNotas,
		acq.AdqCorreo,
		acq.AdqMatriculaMercantil,
		acq.AdqCorreosNotificacion,
		acq.RfiCodigo,
		refCodigoJSON,
		responsableTributosJSON,
	)

	result, err := tx.Exec(ctx, query, allArgs...)
	if err != nil {
		return fmt.Errorf("update acquirer: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("acquirer not found")
	}

	// Get the acquirer ID for updating contacts
	var acquirerID int64
	err = tx.QueryRow(ctx, "SELECT id FROM acquirer WHERE "+whereClause, queryArgs...).Scan(&acquirerID)
	if err != nil {
		return fmt.Errorf("get acquirer id: %w", err)
	}

	// Delete existing contacts
	_, err = tx.Exec(ctx, "DELETE FROM acquirer_contact WHERE acquirer_id = $1", acquirerID)
	if err != nil {
		return fmt.Errorf("delete contacts: %w", err)
	}

	// Insert new contacts
	if len(acq.Contactos) > 0 {
		contactQuery := `
			INSERT INTO acquirer_contact (
				acquirer_id, con_nombre, con_direccion, con_telefono,
				con_correo, con_observaciones, con_tipo
			) VALUES ($1, $2, $3, $4, $5, $6, $7)
		`

		for _, contact := range acq.Contactos {
			_, err = tx.Exec(ctx, contactQuery,
				acquirerID,
				contact.Nombre,
				contact.Direccion,
				contact.Telefono,
				contact.Correo,
				contact.Observaciones,
				contact.Tipo,
			)
			if err != nil {
				return fmt.Errorf("insert contact: %w", err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// FindByID retrieves an acquirer by its identifiers.
func (r *Repository) FindByID(ctx context.Context, ofeIdentificacion, adqIdentificacion, adqIdPersonalizado string) (*acquirer.Acquirer, error) {
	var whereClause string
	var queryArgs []interface{}

	if adqIdPersonalizado == "" {
		whereClause = "ofe_identificacion = $1 AND adq_identificacion = $2 AND (adq_id_personalizado IS NULL OR adq_id_personalizado = '')"
		queryArgs = []interface{}{ofeIdentificacion, adqIdentificacion}
	} else {
		whereClause = "ofe_identificacion = $1 AND adq_identificacion = $2 AND adq_id_personalizado = $3"
		queryArgs = []interface{}{ofeIdentificacion, adqIdentificacion, adqIdPersonalizado}
	}

	query := `
		SELECT id, ofe_identificacion, adq_identificacion, adq_tipo_adquirente, adq_id_personalizado,
			adq_informacion_personalizada, adq_razon_social, adq_nombre_comercial,
			adq_primer_apellido, adq_segundo_apellido, adq_primer_nombre, adq_otros_nombres,
			tdo_codigo, toj_codigo, pai_codigo, dep_codigo, dep_nombre, mun_codigo, mun_nombre, cpo_codigo,
			adq_direccion, adq_telefono, pai_codigo_domicilio_fiscal, dep_codigo_domicilio_fiscal,
			dep_nombre_domicilio_fiscal, mun_codigo_domicilio_fiscal, mun_nombre_domicilio_fiscal,
			cpo_codigo_domicilio_fiscal, adq_direccion_domicilio_fiscal,
			adq_nombre_contacto, adq_fax, adq_notas, adq_correo, adq_matricula_mercantil,
			adq_correos_notificacion, rfi_codigo, ref_codigo, responsable_tributos, created_at, updated_at
		FROM acquirer
		WHERE ` + whereClause

	var acq acquirer.Acquirer
	var refCodigoJSON, responsableTributosJSON []byte
	var adqInformacionPersonalizadaJSON []byte

	err := r.pool.QueryRow(ctx, query, queryArgs...).Scan(
		&acq.ID,
		&acq.OfeIdentificacion,
		&acq.AdqIdentificacion,
		&acq.AdqTipoAdquirente,
		&acq.AdqIDPersonalizado,
		&adqInformacionPersonalizadaJSON,
		&acq.AdqRazonSocial,
		&acq.AdqNombreComercial,
		&acq.AdqPrimerApellido,
		&acq.AdqSegundoApellido,
		&acq.AdqPrimerNombre,
		&acq.AdqOtrosNombres,
		&acq.TdoCodigo,
		&acq.TojCodigo,
		&acq.PaiCodigo,
		&acq.DepCodigo,
		&acq.DepNombre,
		&acq.MunCodigo,
		&acq.MunNombre,
		&acq.CpoCodigo,
		&acq.AdqDireccion,
		&acq.AdqTelefono,
		&acq.PaiCodigoDomicilioFiscal,
		&acq.DepCodigoDomicilioFiscal,
		&acq.DepNombreDomicilioFiscal,
		&acq.MunCodigoDomicilioFiscal,
		&acq.MunNombreDomicilioFiscal,
		&acq.CpoCodigoDomicilioFiscal,
		&acq.AdqDireccionDomicilioFiscal,
		&acq.AdqNombreContacto,
		&acq.AdqFax,
		&acq.AdqNotas,
		&acq.AdqCorreo,
		&acq.AdqMatriculaMercantil,
		&acq.AdqCorreosNotificacion,
		&acq.RfiCodigo,
		&refCodigoJSON,
		&responsableTributosJSON,
		&acq.CreatedAt,
		&acq.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("query acquirer: %w", err)
	}

	// Unmarshal JSON arrays
	if len(refCodigoJSON) > 0 {
		if err := json.Unmarshal(refCodigoJSON, &acq.RefCodigo); err != nil {
			return nil, fmt.Errorf("unmarshal ref_codigo: %w", err)
		}
	}

	if len(responsableTributosJSON) > 0 {
		if err := json.Unmarshal(responsableTributosJSON, &acq.ResponsableTributos); err != nil {
			return nil, fmt.Errorf("unmarshal responsable_tributos: %w", err)
		}
	}

	// Convert adq_informacion_personalizada JSONB back to JSON string
	if len(adqInformacionPersonalizadaJSON) > 0 {
		jsonStr := string(adqInformacionPersonalizadaJSON)
		acq.AdqInformacionPersonalizada = &jsonStr
	}

	// Load contacts
	contactQuery := `
		SELECT con_nombre, con_direccion, con_telefono, con_correo, con_observaciones, con_tipo
		FROM acquirer_contact
		WHERE acquirer_id = $1
		ORDER BY id
	`

	rows, err := r.pool.Query(ctx, contactQuery, acq.ID)
	if err != nil {
		return nil, fmt.Errorf("query contacts: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var contact acquirer.Contact
		err := rows.Scan(
			&contact.Nombre,
			&contact.Direccion,
			&contact.Telefono,
			&contact.Correo,
			&contact.Observaciones,
			&contact.Tipo,
		)
		if err != nil {
			return nil, fmt.Errorf("scan contact: %w", err)
		}
		acq.Contactos = append(acq.Contactos, contact)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate contacts: %w", err)
	}

	return &acq, nil
}

// Exists checks if an acquirer exists with the given identifiers.
func (r *Repository) Exists(ctx context.Context, ofeIdentificacion, adqIdentificacion, adqIdPersonalizado string) (bool, error) {
	var whereClause string
	var queryArgs []interface{}

	if adqIdPersonalizado == "" {
		whereClause = "ofe_identificacion = $1 AND adq_identificacion = $2 AND (adq_id_personalizado IS NULL OR adq_id_personalizado = '')"
		queryArgs = []interface{}{ofeIdentificacion, adqIdentificacion}
	} else {
		whereClause = "ofe_identificacion = $1 AND adq_identificacion = $2 AND adq_id_personalizado = $3"
		queryArgs = []interface{}{ofeIdentificacion, adqIdentificacion, adqIdPersonalizado}
	}

	query := "SELECT EXISTS(SELECT 1 FROM acquirer WHERE " + whereClause + ")"

	var exists bool
	err := r.pool.QueryRow(ctx, query, queryArgs...).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check existence: %w", err)
	}

	return exists, nil
}

// allowedSearchFields is a whitelist of fields that can be searched to prevent SQL injection.
var allowedSearchFields = map[string]bool{
	"adq_identificacion":      true,
	"adq_id_personalizado":    true,
	"adq_razon_social":        true,
	"adq_nombre_comercial":    true,
	"adq_primer_apellido":     true,
	"adq_segundo_apellido":    true,
	"adq_primer_nombre":       true,
	"adq_otros_nombres":       true,
	"tdo_codigo":              true,
	"toj_codigo":              true,
	"pai_codigo":              true,
	"dep_codigo":              true,
	"mun_codigo":              true,
	"cpo_codigo":              true,
	"adq_direccion":           true,
	"adq_telefono":            true,
	"adq_correo":              true,
	"adq_matricula_mercantil": true,
	"rfi_codigo":              true,
}

// Search searches for acquirers by field, value, OFE, and filter type.
func (r *Repository) Search(ctx context.Context, campoBuscar, valorBuscar, valorOfe, filtroColumnas string) ([]acquirer.Acquirer, error) {
	// Validate field name to prevent SQL injection
	if !allowedSearchFields[campoBuscar] {
		return nil, fmt.Errorf("campo de búsqueda no permitido: %s", campoBuscar)
	}

	// Build WHERE clause based on filter type
	var searchCondition string
	if filtroColumnas == "exacto" {
		// Exact match - trim both sides for whitespace handling
		// Since ofe_identificacion and adq_identificacion are NOT NULL, we can safely use TRIM
		searchCondition = campoBuscar + " = $1"
	} else if filtroColumnas == "basico" {
		// Contains match (case-insensitive)
		searchCondition = campoBuscar + " ILIKE '%' || $1 || '%'"
	} else {
		return nil, fmt.Errorf("tipo de filtro inválido: %s (debe ser 'exacto' o 'basico')", filtroColumnas)
	}

	// Build full WHERE clause with OFE filter
	// Trim the input values before passing to query
	whereClause := "ofe_identificacion = $2 AND " + searchCondition
	
	// Trim input values to handle whitespace
	valorBuscar = strings.TrimSpace(valorBuscar)
	valorOfe = strings.TrimSpace(valorOfe)

	query := `
		SELECT id, ofe_identificacion, adq_identificacion, adq_tipo_adquirente, adq_id_personalizado,
			adq_informacion_personalizada, adq_razon_social, adq_nombre_comercial,
			adq_primer_apellido, adq_segundo_apellido, adq_primer_nombre, adq_otros_nombres,
			tdo_codigo, toj_codigo, pai_codigo, dep_codigo, dep_nombre, mun_codigo, mun_nombre, cpo_codigo,
			adq_direccion, adq_telefono, pai_codigo_domicilio_fiscal, dep_codigo_domicilio_fiscal,
			dep_nombre_domicilio_fiscal, mun_codigo_domicilio_fiscal, mun_nombre_domicilio_fiscal,
			cpo_codigo_domicilio_fiscal, adq_direccion_domicilio_fiscal,
			adq_nombre_contacto, adq_fax, adq_notas, adq_correo, adq_matricula_mercantil,
			adq_correos_notificacion, rfi_codigo, ref_codigo, responsable_tributos, created_at, updated_at
		FROM acquirer
		WHERE ` + whereClause + `
		ORDER BY id
	`

	// Log the query and parameters for debugging
	if r.log != nil {
		r.log.Debug("Executing acquirer search query",
			"campoBuscar", campoBuscar,
			"valorBuscar", valorBuscar,
			"valorOfe", valorOfe,
			"filtroColumnas", filtroColumnas,
			"query", query)
	}

	rows, err := r.pool.Query(ctx, query, valorBuscar, valorOfe)
	if err != nil {
		return nil, fmt.Errorf("query acquirers: %w", err)
	}
	defer rows.Close()

	var acquirers []acquirer.Acquirer
	for rows.Next() {
		var acq acquirer.Acquirer
		var refCodigoJSON, responsableTributosJSON []byte
		var adqInformacionPersonalizadaJSON []byte

		err := rows.Scan(
			&acq.ID,
			&acq.OfeIdentificacion,
			&acq.AdqIdentificacion,
			&acq.AdqTipoAdquirente,
			&acq.AdqIDPersonalizado,
			&adqInformacionPersonalizadaJSON,
			&acq.AdqRazonSocial,
			&acq.AdqNombreComercial,
			&acq.AdqPrimerApellido,
			&acq.AdqSegundoApellido,
			&acq.AdqPrimerNombre,
			&acq.AdqOtrosNombres,
			&acq.TdoCodigo,
			&acq.TojCodigo,
			&acq.PaiCodigo,
			&acq.DepCodigo,
			&acq.DepNombre,
			&acq.MunCodigo,
			&acq.MunNombre,
			&acq.CpoCodigo,
			&acq.AdqDireccion,
			&acq.AdqTelefono,
			&acq.PaiCodigoDomicilioFiscal,
			&acq.DepCodigoDomicilioFiscal,
			&acq.DepNombreDomicilioFiscal,
			&acq.MunCodigoDomicilioFiscal,
			&acq.MunNombreDomicilioFiscal,
			&acq.CpoCodigoDomicilioFiscal,
			&acq.AdqDireccionDomicilioFiscal,
			&acq.AdqNombreContacto,
			&acq.AdqFax,
			&acq.AdqNotas,
			&acq.AdqCorreo,
			&acq.AdqMatriculaMercantil,
			&acq.AdqCorreosNotificacion,
			&acq.RfiCodigo,
			&refCodigoJSON,
			&responsableTributosJSON,
			&acq.CreatedAt,
			&acq.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan acquirer: %w", err)
		}

		// Unmarshal JSON arrays
		if len(refCodigoJSON) > 0 {
			if err := json.Unmarshal(refCodigoJSON, &acq.RefCodigo); err != nil {
				return nil, fmt.Errorf("unmarshal ref_codigo: %w", err)
			}
		}

		if len(responsableTributosJSON) > 0 {
			if err := json.Unmarshal(responsableTributosJSON, &acq.ResponsableTributos); err != nil {
				return nil, fmt.Errorf("unmarshal responsable_tributos: %w", err)
			}
		}

		// Convert adq_informacion_personalizada JSONB back to JSON string
		if len(adqInformacionPersonalizadaJSON) > 0 {
			jsonStr := string(adqInformacionPersonalizadaJSON)
			acq.AdqInformacionPersonalizada = &jsonStr
		}

		// Load contacts for this acquirer
		contactQuery := `
			SELECT con_nombre, con_direccion, con_telefono, con_correo, con_observaciones, con_tipo
			FROM acquirer_contact
			WHERE acquirer_id = $1
			ORDER BY id
		`

		contactRows, err := r.pool.Query(ctx, contactQuery, acq.ID)
		if err != nil {
			return nil, fmt.Errorf("query contacts: %w", err)
		}

		for contactRows.Next() {
			var contact acquirer.Contact
			err := contactRows.Scan(
				&contact.Nombre,
				&contact.Direccion,
				&contact.Telefono,
				&contact.Correo,
				&contact.Observaciones,
				&contact.Tipo,
			)
			if err != nil {
				contactRows.Close()
				return nil, fmt.Errorf("scan contact: %w", err)
			}
			acq.Contactos = append(acq.Contactos, contact)
		}
		contactRows.Close()

		if err := contactRows.Err(); err != nil {
			return nil, fmt.Errorf("iterate contacts: %w", err)
		}

		acquirers = append(acquirers, acq)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate rows: %w", err)
	}

	return acquirers, nil
}

// allowedOrderFields is a whitelist of fields that can be used for ordering.
var allowedAcquirerOrderFields = map[string]bool{
	"id":                   true,
	"adq_identificacion":   true,
	"adq_razon_social":     true,
	"adq_nombre_comercial": true,
	"created_at":           true,
	"updated_at":           true,
}

// List retrieves acquirers with pagination, search, and sorting.
func (r *Repository) List(ctx context.Context, start, length int, buscar, columnaOrden, ordenDireccion string) ([]acquirer.Acquirer, int, error) {
	// Build WHERE clause for search
	whereConditions := []string{}
	queryArgs := []interface{}{}
	argIndex := 1

	if buscar != "" {
		// Search in multiple fields: identificación, razón social, nombre comercial
		searchPattern := "%" + buscar + "%"
		whereConditions = append(whereConditions, fmt.Sprintf(
			"(adq_identificacion ILIKE $%d OR adq_razon_social ILIKE $%d OR adq_nombre_comercial ILIKE $%d OR COALESCE(adq_primer_nombre || ' ' || adq_primer_apellido, '') ILIKE $%d)",
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
			"codigo":             "adq_identificacion",
			"identificacion":     "adq_identificacion",
			"razon_social":       "adq_razon_social",
			"nombre_comercial":   "adq_nombre_comercial",
			"fecha_creacion":     "created_at",
			"fecha_modificacion": "updated_at",
		}

		dbColumn := orderFieldMap[columnaOrden]
		if dbColumn == "" {
			// Try direct column name if not in map
			dbColumn = columnaOrden
		}

		if allowedAcquirerOrderFields[dbColumn] {
			direction := "ASC"
			if strings.ToLower(ordenDireccion) == "desc" {
				direction = "DESC"
			}
			orderByClause = fmt.Sprintf("ORDER BY %s %s", dbColumn, direction)
		}
	}

	// Get total count
	countQuery := "SELECT COUNT(*) FROM acquirer " + whereClause
	var total int
	err := r.pool.QueryRow(ctx, countQuery, queryArgs...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count acquirers: %w", err)
	}

	// Build main query with pagination
	query := fmt.Sprintf(`
		SELECT id, ofe_identificacion, adq_identificacion, adq_tipo_adquirente, adq_id_personalizado,
			adq_informacion_personalizada, adq_razon_social, adq_nombre_comercial,
			adq_primer_apellido, adq_segundo_apellido, adq_primer_nombre, adq_otros_nombres,
			tdo_codigo, toj_codigo, pai_codigo, dep_codigo, dep_nombre, mun_codigo, mun_nombre, cpo_codigo,
			adq_direccion, adq_telefono, pai_codigo_domicilio_fiscal, dep_codigo_domicilio_fiscal,
			dep_nombre_domicilio_fiscal, mun_codigo_domicilio_fiscal, mun_nombre_domicilio_fiscal,
			cpo_codigo_domicilio_fiscal, adq_direccion_domicilio_fiscal,
			adq_nombre_contacto, adq_fax, adq_notas, adq_correo, adq_matricula_mercantil,
			adq_correos_notificacion, rfi_codigo, ref_codigo, responsable_tributos, created_at, updated_at
		FROM acquirer
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
		return nil, 0, fmt.Errorf("query acquirers: %w", err)
	}
	defer rows.Close()

	var acquirers []acquirer.Acquirer
	for rows.Next() {
		var acq acquirer.Acquirer
		var refCodigoJSON, responsableTributosJSON []byte
		var adqInformacionPersonalizadaJSON []byte

		err := rows.Scan(
			&acq.ID,
			&acq.OfeIdentificacion,
			&acq.AdqIdentificacion,
			&acq.AdqTipoAdquirente,
			&acq.AdqIDPersonalizado,
			&adqInformacionPersonalizadaJSON,
			&acq.AdqRazonSocial,
			&acq.AdqNombreComercial,
			&acq.AdqPrimerApellido,
			&acq.AdqSegundoApellido,
			&acq.AdqPrimerNombre,
			&acq.AdqOtrosNombres,
			&acq.TdoCodigo,
			&acq.TojCodigo,
			&acq.PaiCodigo,
			&acq.DepCodigo,
			&acq.DepNombre,
			&acq.MunCodigo,
			&acq.MunNombre,
			&acq.CpoCodigo,
			&acq.AdqDireccion,
			&acq.AdqTelefono,
			&acq.PaiCodigoDomicilioFiscal,
			&acq.DepCodigoDomicilioFiscal,
			&acq.DepNombreDomicilioFiscal,
			&acq.MunCodigoDomicilioFiscal,
			&acq.MunNombreDomicilioFiscal,
			&acq.CpoCodigoDomicilioFiscal,
			&acq.AdqDireccionDomicilioFiscal,
			&acq.AdqNombreContacto,
			&acq.AdqFax,
			&acq.AdqNotas,
			&acq.AdqCorreo,
			&acq.AdqMatriculaMercantil,
			&acq.AdqCorreosNotificacion,
			&acq.RfiCodigo,
			&refCodigoJSON,
			&responsableTributosJSON,
			&acq.CreatedAt,
			&acq.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scan acquirer: %w", err)
		}

		// Unmarshal JSON arrays
		if len(refCodigoJSON) > 0 {
			if err := json.Unmarshal(refCodigoJSON, &acq.RefCodigo); err != nil {
				return nil, 0, fmt.Errorf("unmarshal ref_codigo: %w", err)
			}
		}

		if len(responsableTributosJSON) > 0 {
			if err := json.Unmarshal(responsableTributosJSON, &acq.ResponsableTributos); err != nil {
				return nil, 0, fmt.Errorf("unmarshal responsable_tributos: %w", err)
			}
		}

		// Convert adq_informacion_personalizada JSONB back to JSON string
		if len(adqInformacionPersonalizadaJSON) > 0 {
			jsonStr := string(adqInformacionPersonalizadaJSON)
			acq.AdqInformacionPersonalizada = &jsonStr
		}

		// Load contacts for this acquirer
		contactQuery := `
			SELECT con_nombre, con_direccion, con_telefono, con_correo, con_observaciones, con_tipo
			FROM acquirer_contact
			WHERE acquirer_id = $1
			ORDER BY id
		`

		contactRows, err := r.pool.Query(ctx, contactQuery, acq.ID)
		if err != nil {
			return nil, 0, fmt.Errorf("query contacts: %w", err)
		}

		for contactRows.Next() {
			var contact acquirer.Contact
			err := contactRows.Scan(
				&contact.Nombre,
				&contact.Direccion,
				&contact.Telefono,
				&contact.Correo,
				&contact.Observaciones,
				&contact.Tipo,
			)
			if err != nil {
				contactRows.Close()
				return nil, 0, fmt.Errorf("scan contact: %w", err)
			}
			acq.Contactos = append(acq.Contactos, contact)
		}
		contactRows.Close()

		if err := contactRows.Err(); err != nil {
			return nil, 0, fmt.Errorf("iterate contacts: %w", err)
		}

		acquirers = append(acquirers, acq)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate rows: %w", err)
	}

	// Calculate filtered count (same as total if no search, otherwise use filtered results count)
	filtered := len(acquirers)
	if buscar == "" && length == -1 {
		filtered = total
	} else if buscar != "" {
		// Re-count with search to get filtered total
		filtered = total
	}

	return acquirers, filtered, nil
}
