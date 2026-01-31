package acquirer

import "time"

// Acquirer represents an acquirer entity in the domain.
type Acquirer struct {
	ID                          int64     `json:"id"`
	OfeIdentificacion           string    `json:"ofe_identificacion"`
	AdqIdentificacion           string    `json:"adq_identificacion"`
	AdqTipoAdquirente           *string   `json:"adq_tipo_adquirente"`
	AdqIDPersonalizado          *string   `json:"adq_id_personalizado"`
	AdqInformacionPersonalizada *string   `json:"adq_informacion_personalizada"` // JSON string
	AdqRazonSocial              string    `json:"adq_razon_social"`
	AdqNombreComercial          *string   `json:"adq_nombre_comercial"`
	AdqPrimerApellido           *string   `json:"adq_primer_apellido"`
	AdqSegundoApellido          *string   `json:"adq_segundo_apellido"`
	AdqPrimerNombre             *string   `json:"adq_primer_nombre"`
	AdqOtrosNombres             *string   `json:"adq_otros_nombres"`
	TdoCodigo                   string    `json:"tdo_codigo"`
	TojCodigo                   string    `json:"toj_codigo"`
	PaiCodigo                   string    `json:"pai_codigo"`
	DepCodigo                   *string   `json:"dep_codigo"`
	DepNombre                   *string   `json:"dep_nombre"`
	MunCodigo                   *string   `json:"mun_codigo"`
	MunNombre                   *string   `json:"mun_nombre"`
	CpoCodigo                   *string   `json:"cpo_codigo"`
	AdqDireccion                *string   `json:"adq_direccion"`
	AdqTelefono                 *string   `json:"adq_telefono"`
	PaiCodigoDomicilioFiscal    *string   `json:"pai_codigo_domicilio_fiscal"`
	DepCodigoDomicilioFiscal    *string   `json:"dep_codigo_domicilio_fiscal"`
	DepNombreDomicilioFiscal    *string   `json:"dep_nombre_domicilio_fiscal"`
	MunCodigoDomicilioFiscal    *string   `json:"mun_codigo_domicilio_fiscal"`
	MunNombreDomicilioFiscal    *string   `json:"mun_nombre_domicilio_fiscal"`
	CpoCodigoDomicilioFiscal    *string   `json:"cpo_codigo_domicilio_fiscal"`
	AdqDireccionDomicilioFiscal *string   `json:"adq_direccion_domicilio_fiscal"`
	AdqNombreContacto           *string   `json:"adq_nombre_contacto"`
	AdqFax                      *string   `json:"adq_fax"`
	AdqNotas                    *string   `json:"adq_notas"`
	AdqCorreo                   *string   `json:"adq_correo"`
	AdqMatriculaMercantil       *string   `json:"adq_matricula_mercantil"`
	AdqCorreosNotificacion      *string   `json:"adq_correos_notificacion"`
	RfiCodigo                   *string   `json:"rfi_codigo"`
	RefCodigo                   []string `json:"ref_codigo"` // Array of codes
	ResponsableTributos         []string `json:"responsable_tributos"` // Array of codes
	Contactos                   []Contact `json:"contactos"`
	CreatedAt                   time.Time `json:"created_at"`
	UpdatedAt                   time.Time `json:"updated_at"`
}
