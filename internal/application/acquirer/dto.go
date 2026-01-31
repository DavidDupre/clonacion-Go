package acquirer

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"3tcapital/ms_facturacion_core/internal/core/acquirer"
)

// ContactoDTO represents a contact in the response DTO format.
type ContactoDTO struct {
	ConNombre        string `json:"con_nombre"`
	ConDireccion     string `json:"con_direccion"`
	ConTelefono      string `json:"con_telefono"`
	ConCorreo        string `json:"con_correo"`
	ConObservaciones string `json:"con_observaciones"`
	ConTipo          string `json:"con_tipo"`
}

// AdquirenteDTO represents the acquirer response DTO matching the expected Java DTO structure.
type AdquirenteDTO struct {
	ID                                          int         `json:"id"`
	OfeIdentificacion                           int         `json:"ofe_identificacion"`
	AdqIdentificacion                           string      `json:"adq_identificacion"`
	AdqIDPersonalizado                          *string     `json:"adq_id_personalizado"`
	AdqInformacionPersonalizada                 *string     `json:"adq_informacion_personalizada"`
	AdqRazonSocial                              string      `json:"adq_razon_social"`
	AdqNombreComercial                          *string     `json:"adq_nombre_comercial"`
	AdqPrimerApellido                           *string     `json:"adq_primer_apellido"`
	AdqSegundoApellido                          *string     `json:"adq_segundo_apellido"`
	AdqPrimerNombre                             *string     `json:"adq_primer_nombre"`
	AdqOtrosNombres                             *string     `json:"adq_otros_nombres"`
	TdoCodigo                                   int         `json:"tdo_codigo"`
	TojCodigo                                   int         `json:"toj_codigo"`
	PaiCodigo                                   int         `json:"pai_codigo"`
	DepCodigo                                   *int        `json:"dep_codigo"`
	MunCodigo                                   *int        `json:"mun_codigo"`
	CpoCodigo                                   *int        `json:"cpo_codigo"`
	AdqDireccion                                *string     `json:"adq_direccion"`
	AdqTelefono                                 *string     `json:"adq_telefono"`
	PaiCodigoDomicilioFiscal                    *int        `json:"pai_codigo_domicilio_fiscal"`
	DepCodigoDomicilioFiscal                    *int        `json:"dep_codigo_domicilio_fiscal"`
	MunCodigoDomicilioFiscal                    *int        `json:"mun_codigo_domicilio_fiscal"`
	CpoCodigoDomicilioFiscal                    *string     `json:"cpo_codigo_domicilio_fiscal"`
	AdqDireccionDomicilioFiscal                 *string     `json:"adq_direccion_domicilio_fiscal"`
	AdqNombreContacto                           *string     `json:"adq_nombre_contacto"`
	AdqNotas                                    *string     `json:"adq_notas"`
	AdqCorreo                                   *string     `json:"adq_correo"`
	AdqCorreosNotificacion                      *string     `json:"adq_correos_notificacion"`
	RfiCodigo                                   *int        `json:"rfi_codigo"`
	RefCodigo                                   *string     `json:"ref_codigo"` // JSON array as string
	AdqMatriculaMercantil                       *string     `json:"adq_matricula_mercantil"`
	Contactos                                   []ContactoDTO `json:"contactos"`
	AdqCamposRepresentacionGrafica              *string     `json:"adq_campos_representacion_grafica"`
	TatID                                       *string     `json:"tat_id"`
	IpvID                                       *string     `json:"ipv_id"`
	AdqReenvioNotificacionContingencia          *string     `json:"adq_reenvio_notificacion_contingencia"`
	UsuarioCreacion                             *int        `json:"usuario_creacion"`
	FechaCreacion                               *string    `json:"fecha_creacion"`
	FechaModificacion                           *string    `json:"fecha_modificacion"`
	Estado                                      *string    `json:"estado"`
	FechaActualizacion                          *string    `json:"fecha_actualizacion"`
	GetUsuarioCreacion                          interface{} `json:"get_usuario_creacion"`
	GetConfiguracionObligadoFacturarElectronicamente interface{} `json:"get_configuracion_obligado_facturar_electronicamente"`
	GetParametroTipoDocumento                    interface{} `json:"get_parametro_tipo_documento"`
	GetParametroPais                            interface{} `json:"get_parametro_pais"`
	GetParametroDepartamento                     interface{} `json:"get_parametro_departamento"`
	GetParametroMunicipio                        interface{} `json:"get_parametro_municipio"`
	GetCodigoPostal                              interface{} `json:"get_codigo_postal"`
	GetParametroDomicilioFiscalPais              interface{} `json:"get_parametro_domicilio_fiscal_pais"`
	GetParametroDomicilioFiscalDepartamento      interface{} `json:"get_parametro_domicilio_fiscal_departamento"`
	GetParametroDomicilioFiscalMunicipio          interface{} `json:"get_parametro_domicilio_fiscal_municipio"`
	GetCodigoPostalDomicilioFiscal                interface{} `json:"get_codigo_postal_domicilio_fiscal"`
	GetParametroTipoOrganizacionJuridica          interface{} `json:"get_parametro_tipo_organizacion_juridica"`
	GetRegimenFiscal                              interface{} `json:"get_regimen_fiscal"`
	GetProcedenciaVendedor                        interface{} `json:"get_procedencia_vendedor"`
	GetTributos                                   interface{} `json:"get_tributos"`
	GetResponsabilidadFiscal                       interface{} `json:"get_responsabilidad_fiscal"`
	GetTiempoAceptacionTacita                     interface{} `json:"get_tiempo_aceptacion_tacita"`
	GetUsuariosPortales                           interface{} `json:"get_usuarios_portales"`
	NombreCompleto                                *string    `json:"nombre_completo"`
	AdqTipoAdquirente                            *string    `json:"adq_tipo_adquirente"`
	AdqTipoAutorizado                             *string    `json:"adq_tipo_autorizado"`
	AdqTipoResponsableEntrega                    *string    `json:"adq_tipo_responsable_entrega"`
	AdqTipoVendedorDs                             *string    `json:"adq_tipo_vendedor_ds"`
}

// stringToInt converts a string to int, returning nil if empty or invalid.
func stringToInt(s *string) *int {
	if s == nil || *s == "" {
		return nil
	}
	val, err := strconv.Atoi(strings.TrimSpace(*s))
	if err != nil {
		return nil
	}
	return &val
}

// normalizeNIT extracts the base NIT without verification digit (DV).
// Examples:
//   - "860011153-6" -> "860011153"
//   - "860011153" -> "860011153"
func normalizeNIT(nit string) string {
	if nit == "" {
		return ""
	}
	// Split by "-" to separate NIT from DV
	parts := strings.Split(nit, "-")
	if len(parts) > 1 {
		return strings.TrimSpace(parts[0])
	}
	return strings.TrimSpace(nit)
}

// stringToIntRequired converts a string to int, returning 0 if empty or invalid.
func stringToIntRequired(s string) int {
	val, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return val
}

// formatDateTime formats a time.Time to string in format "YYYY-MM-DD HH:MM:SS".
func formatDateTime(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

// buildNombreCompleto builds the full name from acquirer fields.
func buildNombreCompleto(acq *acquirer.Acquirer) *string {
	var parts []string
	
	if acq.AdqPrimerNombre != nil && *acq.AdqPrimerNombre != "" {
		parts = append(parts, *acq.AdqPrimerNombre)
	}
	if acq.AdqOtrosNombres != nil && *acq.AdqOtrosNombres != "" {
		parts = append(parts, *acq.AdqOtrosNombres)
	}
	if acq.AdqPrimerApellido != nil && *acq.AdqPrimerApellido != "" {
		parts = append(parts, *acq.AdqPrimerApellido)
	}
	if acq.AdqSegundoApellido != nil && *acq.AdqSegundoApellido != "" {
		parts = append(parts, *acq.AdqSegundoApellido)
	}
	
	if len(parts) == 0 {
		return nil
	}
	
	fullName := strings.Join(parts, " ")
	return &fullName
}

// arrayToString converts a string array to a JSON string.
func arrayToString(arr []string) *string {
	if len(arr) == 0 {
		return nil
	}
	// Use JSON encoding to properly escape strings
	jsonBytes, err := json.Marshal(arr)
	if err != nil {
		return nil
	}
	jsonStr := string(jsonBytes)
	return &jsonStr
}

// ToAdquirenteDTO converts a domain Acquirer entity to AdquirenteDTO.
func ToAdquirenteDTO(acq *acquirer.Acquirer) *AdquirenteDTO {
	if acq == nil {
		return nil
	}

	dto := &AdquirenteDTO{
		ID:                          int(acq.ID),
		OfeIdentificacion:           stringToIntRequired(normalizeNIT(acq.OfeIdentificacion)),
		AdqIdentificacion:           acq.AdqIdentificacion,
		AdqIDPersonalizado:          acq.AdqIDPersonalizado,
		AdqInformacionPersonalizada: acq.AdqInformacionPersonalizada,
		AdqRazonSocial:              acq.AdqRazonSocial,
		AdqNombreComercial:          acq.AdqNombreComercial,
		AdqPrimerApellido:           acq.AdqPrimerApellido,
		AdqSegundoApellido:          acq.AdqSegundoApellido,
		AdqPrimerNombre:             acq.AdqPrimerNombre,
		AdqOtrosNombres:             acq.AdqOtrosNombres,
		TdoCodigo:                   stringToIntRequired(acq.TdoCodigo),
		TojCodigo:                   stringToIntRequired(acq.TojCodigo),
		PaiCodigo:                   stringToIntRequired(acq.PaiCodigo),
		DepCodigo:                   stringToInt(acq.DepCodigo),
		MunCodigo:                   stringToInt(acq.MunCodigo),
		CpoCodigo:                   stringToInt(acq.CpoCodigo),
		AdqDireccion:                acq.AdqDireccion,
		AdqTelefono:                 acq.AdqTelefono,
		PaiCodigoDomicilioFiscal:    stringToInt(acq.PaiCodigoDomicilioFiscal),
		DepCodigoDomicilioFiscal:    stringToInt(acq.DepCodigoDomicilioFiscal),
		MunCodigoDomicilioFiscal:    stringToInt(acq.MunCodigoDomicilioFiscal),
		CpoCodigoDomicilioFiscal:     acq.CpoCodigoDomicilioFiscal,
		AdqDireccionDomicilioFiscal: acq.AdqDireccionDomicilioFiscal,
		AdqNombreContacto:           acq.AdqNombreContacto,
		AdqNotas:                     acq.AdqNotas,
		AdqCorreo:                    acq.AdqCorreo,
		AdqCorreosNotificacion:       acq.AdqCorreosNotificacion,
		RfiCodigo:                    stringToInt(acq.RfiCodigo),
		RefCodigo:                    arrayToString(acq.RefCodigo),
		AdqMatriculaMercantil:        acq.AdqMatriculaMercantil,
		AdqTipoAdquirente:            acq.AdqTipoAdquirente,
		NombreCompleto:               buildNombreCompleto(acq),
	}

	// Convert contacts
	if len(acq.Contactos) > 0 {
		dto.Contactos = make([]ContactoDTO, len(acq.Contactos))
		for i, contact := range acq.Contactos {
			dto.Contactos[i] = ContactoDTO{
				ConNombre:        contact.Nombre,
				ConDireccion:     contact.Direccion,
				ConTelefono:      contact.Telefono,
				ConCorreo:        contact.Correo,
				ConObservaciones: contact.Observaciones,
				ConTipo:          contact.Tipo,
			}
		}
	}

	// Format dates
	if !acq.CreatedAt.IsZero() {
		fechaCreacion := formatDateTime(acq.CreatedAt)
		dto.FechaCreacion = &fechaCreacion
	}
	if !acq.UpdatedAt.IsZero() {
		fechaModificacion := formatDateTime(acq.UpdatedAt)
		dto.FechaModificacion = &fechaModificacion
		fechaActualizacion := formatDateTime(acq.UpdatedAt)
		dto.FechaActualizacion = &fechaActualizacion
	}

	// Set default estado to "ACTIVO" if not set
	estado := "ACTIVO"
	dto.Estado = &estado

	// All "get_*" fields are set to null (nil) as they are not implemented
	// These would typically be populated from related tables or external services

	return dto
}

// ToAdquirenteDTOList converts a slice of domain Acquirer entities to AdquirenteDTO slice.
func ToAdquirenteDTOList(acquirers []acquirer.Acquirer) []AdquirenteDTO {
	if len(acquirers) == 0 {
		return []AdquirenteDTO{}
	}

	dtos := make([]AdquirenteDTO, 0, len(acquirers))
	for i := range acquirers {
		dto := ToAdquirenteDTO(&acquirers[i])
		if dto != nil {
			dtos = append(dtos, *dto)
		}
	}
	return dtos
}

