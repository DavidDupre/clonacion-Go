package provider

import "time"

// Provider represents a provider entity in the domain.
type Provider struct {
	ID                          int64     `json:"pro_id"`
	OfeIdentificacion           string    `json:"ofe_identificacion"`
	ProIdentificacion           string    `json:"pro_identificacion"`
	ProIDPersonalizado          *string   `json:"pro_id_personalizado"`
	ProRazonSocial              *string   `json:"pro_razon_social"`
	ProNombreComercial          *string   `json:"pro_nombre_comercial"`
	ProPrimerApellido           *string   `json:"pro_primer_apellido"`
	ProSegundoApellido          *string   `json:"pro_segundo_apellido"`
	ProPrimerNombre             *string   `json:"pro_primer_nombre"`
	ProOtrosNombres             *string   `json:"pro_otros_nombres"`
	TdoCodigo                   string    `json:"tdo_codigo"`
	TojCodigo                   string    `json:"toj_codigo"`
	PaiCodigo                   *string   `json:"pai_codigo"`
	DepCodigo                   *string   `json:"dep_codigo"`
	MunCodigo                   *string   `json:"mun_codigo"`
	CpoCodigo                   *string   `json:"cpo_codigo"`
	ProDireccion                *string   `json:"pro_direccion"`
	ProTelefono                 *string   `json:"pro_telefono"`
	PaiCodigoDomicilioFiscal    *string   `json:"pai_codigo_domicilio_fiscal"`
	DepCodigoDomicilioFiscal    *string   `json:"dep_codigo_domicilio_fiscal"`
	MunCodigoDomicilioFiscal    *string   `json:"mun_codigo_domicilio_fiscal"`
	CpoCodigoDomicilioFiscal    *string   `json:"cpo_codigo_domicilio_fiscal"`
	ProDireccionDomicilioFiscal *string   `json:"pro_direccion_domicilio_fiscal"`
	ProCorreo                   string    `json:"pro_correo"`
	ProCorreosNotificacion      *string   `json:"pro_correos_notificacion"`
	ProMatriculaMercantil       *string   `json:"pro_matricula_mercantil"`
	ProUsuariosRecepcion        []string  `json:"pro_usuarios_recepcion"` // Array of user identifiers
	RfiCodigo                   *string   `json:"rfi_codigo"`
	RefCodigo                   []string  `json:"ref_codigo"` // Array of codes
	Estado                      string    `json:"estado"`     // ACTIVO/INACTIVO
	CreatedAt                   time.Time `json:"fecha_creacion"`
	UpdatedAt                   time.Time `json:"fecha_modificacion"`
}

// NombreCompleto returns the full name of the provider.
// For legal entities (JURÍDICA), it returns the commercial name or business name.
// For natural persons (NATURAL), it concatenates names and surnames.
func (p *Provider) NombreCompleto() string {
	if p.ProNombreComercial != nil && *p.ProNombreComercial != "" {
		return *p.ProNombreComercial
	}
	if p.ProRazonSocial != nil && *p.ProRazonSocial != "" {
		return *p.ProRazonSocial
	}
	// For natural persons, concatenate names
	var parts []string
	if p.ProPrimerNombre != nil {
		parts = append(parts, *p.ProPrimerNombre)
	}
	if p.ProOtrosNombres != nil {
		parts = append(parts, *p.ProOtrosNombres)
	}
	if p.ProPrimerApellido != nil {
		parts = append(parts, *p.ProPrimerApellido)
	}
	if p.ProSegundoApellido != nil {
		parts = append(parts, *p.ProSegundoApellido)
	}
	if len(parts) > 0 {
		result := ""
		for i, part := range parts {
			if i > 0 {
				result += " "
			}
			result += part
		}
		return result
	}
	return ""
}
