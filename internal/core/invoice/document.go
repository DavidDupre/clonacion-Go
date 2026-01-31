package invoice

import "time"

// Document represents an invoice/document in the domain.
type Document struct {
	OFE         string    `json:"ofe"`
	Proveedor   string    `json:"proveedor"`
	Tipo        string    `json:"tipo"`
	Prefijo     string    `json:"prefijo"`
	Consecutivo string    `json:"consecutivo"`
	CUFE        string    `json:"cufe"`
	Fecha       time.Time `json:"fecha"`
	Hora        string    `json:"hora"`
	Valor       float64   `json:"valor"`
	Marca       bool      `json:"marca"`
	UrlPDF      string    `json:"urlPDF,omitempty"`
	UrlXML      string    `json:"urlXML,omitempty"`
}

// DocumentQuery represents the query parameters for retrieving documents.
type DocumentQuery struct {
	CompanyNit  string
	InitialDate string
	FinalDate   string
}

// DocumentByNumberQuery represents the query parameters for retrieving a document by number.
type DocumentByNumberQuery struct {
	CompanyNit     string
	DocumentNumber string
	SupplierNit    string
}

// DocumentRegistrationRequest represents a request to register documents.
type DocumentRegistrationRequest struct {
	Documentos DocumentsByType `json:"documentos"`
}

// DocumentsByType groups documents by their type (FC, NC, ND, DS).
type DocumentsByType struct {
	FC []OpenETLDocument `json:"FC"`
	NC []OpenETLDocument `json:"NC"`
	ND []OpenETLDocument `json:"ND"`
	DS []OpenETLDocument `json:"DS"`
}

// OpenETLDocument represents a single document in OpenETL format.
type OpenETLDocument struct {
	TdeCodigo                         string                 `json:"tde_codigo"`
	TopCodigo                         string                 `json:"top_codigo"`
	OfeIdentificacion                 string                 `json:"ofe_identificacion"`
	AdqIdentificacion                 string                 `json:"adq_identificacion"`
	AdqIdentificacionAutorizado       *string                `json:"adq_identificacion_autorizado"`
	RfaPrefijo                        string                 `json:"rfa_prefijo"`
	RfaResolucion                     string                 `json:"rfa_resolucion"`
	RfaFechaInicio                    *string                `json:"rfa_fecha_inicio"`
	RfaFechaFin                       *string                `json:"rfa_fecha_fin"`
	RfaNumeroInicio                   *string                `json:"rfa_numero_inicio"`
	RfaNumeroFin                      *string                `json:"rfa_numero_fin"`
	CdoAmbiente                       *string                `json:"cdo_ambiente"`
	CdoConsecutivo                    string                 `json:"cdo_consecutivo"`
	CdoFecha                          string                 `json:"cdo_fecha"`
	CdoHora                           string                 `json:"cdo_hora"`
	CdoVencimiento                    *string                `json:"cdo_vencimiento"`
	CdoRepresentacionGraficaDocumento *string                `json:"cdo_representacion_grafica_documento"`
	CdoRepresentacionGraficaAcuse     *string                `json:"cdo_representacion_grafica_acuse"`
	CdoMediosPago                     []OpenETLMedioPago     `json:"cdo_medios_pago"`
	CdoInformacionAdicional           map[string]interface{} `json:"cdo_informacion_adicional"`
	MonCodigo                         string                 `json:"mon_codigo"`
	CdoValorSinImpuestos              string                 `json:"cdo_valor_sin_impuestos"`
	CdoImpuestos                      string                 `json:"cdo_impuestos"`
	CdoTotal                          string                 `json:"cdo_total"`
	CdoRetencionesSugeridas           string                 `json:"cdo_retenciones_sugeridas"`
	CdoRetenciones                    string                 `json:"cdo_retenciones"`
	CdoCargos                         string                 `json:"cdo_cargos"`
	CdoDescuentos                     string                 `json:"cdo_descuentos"`
	CdoAnticipo                       string                 `json:"cdo_anticipo"`
	CdoRedondeo                       string                 `json:"cdo_redondeo"`
	CdoDetalleAnticipos               []interface{}          `json:"cdo_detalle_anticipos"`
	CdoDetalleRetencionesSugeridas    []OpenETLRetencion     `json:"cdo_detalle_retenciones_sugeridas"`
	Items                             []OpenETLItem          `json:"items"`
	Tributos                          []OpenETLTributo       `json:"tributos"`

	// Customer (Adquirente) location and identification fields - Required for DIAN FAJ25, FAK48, FAJ43b
	AdqRazonSocial        *string `json:"adq_razon_social"`
	AdqDireccion          *string `json:"adq_direccion"`
	AdqMunicipioCodigo    *string `json:"adq_municipio_codigo"`
	AdqMunicipioNombre    *string `json:"adq_municipio_nombre"`
	AdqDepartamentoCodigo *string `json:"adq_departamento_codigo"`
	AdqDepartamentoNombre *string `json:"adq_departamento_nombre"`
	AdqPaisCodigo         *string `json:"adq_pais_codigo"`
	AdqPaisNombre         *string `json:"adq_pais_nombre"`
	AdqCpoCodigo          *string `json:"adq_cpo_codigo"`

	// Supplier (Oferente) location and identification fields
	OfeRazonSocial        *string `json:"ofe_razon_social"`
	OfeDireccion          *string `json:"ofe_direccion"`
	OfeMunicipioCodigo    *string `json:"ofe_municipio_codigo"`
	OfeMunicipioNombre    *string `json:"ofe_municipio_nombre"`
	OfeDepartamentoCodigo *string `json:"ofe_departamento_codigo"`
	OfeDepartamentoNombre *string `json:"ofe_departamento_nombre"`

	// Notes and Order Reference
	Note           []string               `json:"note"`
	OrderReference *OpenETLOrderReference `json:"order_reference"`

	// Invoice reference and correction concepts for NC/ND
	FacturaReferencia      *OpenETLFacturaReferencia  `json:"factura_referencia,omitempty"`
	CdoConceptosCorreccion *OpenETLConceptoCorreccion `json:"cdo_conceptos_correccion,omitempty"`
}

// OpenETLOrderReference represents an order reference in OpenETL format.
type OpenETLOrderReference struct {
	ID string `json:"id"`
}

// OpenETLConceptoCorreccion represents a correction concept in OpenETL format.
type OpenETLConceptoCorreccion struct {
	CcoCodigo                string `json:"cco_codigo"`
	CdoObservacionCorreccion string `json:"cdo_observacion_correccion"`
}

// OpenETLFacturaReferencia represents a reference to the original invoice in OpenETL format.
type OpenETLFacturaReferencia struct {
	PrefijoFC       string `json:"prefijo_fc"`
	NumeroFacturaFC string `json:"numero_factura_fc"`
}

// OpenETLMedioPago represents a payment means in OpenETL format.
type OpenETLMedioPago struct {
	FpaCodigo           string  `json:"fpa_codigo"`
	MpaCodigo           string  `json:"mpa_codigo"`
	MenFechaVencimiento *string `json:"men_fecha_vencimiento"`
}

// OpenETLRetencion represents a suggested retention in OpenETL format.
type OpenETLRetencion struct {
	Tipo                string                     `json:"tipo"`
	Razon               string                     `json:"razon"`
	Porcentaje          string                     `json:"porcentaje"`
	ValorMonedaNacional OpenETLValorMonedaNacional `json:"valor_moneda_nacional"`
}

// OpenETLValorMonedaNacional represents the monetary value in national currency.
type OpenETLValorMonedaNacional struct {
	Base  string `json:"base"`
	Valor string `json:"valor"`
}

// OpenETLFechaCompra represents purchase date information for DS items.
type OpenETLFechaCompra struct {
	FechaCompra string `json:"fecha_compra"`
	Codigo      string `json:"codigo"`
}

// OpenETLItem represents a line item in OpenETL format.
type OpenETLItem struct {
	DdoTipoItem             string              `json:"ddo_tipo_item"`
	DdoSecuencia            string              `json:"ddo_secuencia"`
	CprCodigo               string              `json:"cpr_codigo"`
	DdoCodigo               string              `json:"ddo_codigo"`
	DdoDescripcionUno       string              `json:"ddo_descripcion_uno"`
	DdoCantidad             string              `json:"ddo_cantidad"`
	UndCodigo               string              `json:"und_codigo"`
	DdoValorUnitario        string              `json:"ddo_valor_unitario"`
	DdoTotal                string              `json:"ddo_total"`
	DdoFechaCompra          *OpenETLFechaCompra `json:"ddo_fecha_compra,omitempty"`
	DdoInformacionAdicional []interface{}       `json:"ddo_informacion_adicional"`
}

// OpenETLTributo represents tax information in OpenETL format.
type OpenETLTributo struct {
	DdoSecuencia      string                    `json:"ddo_secuencia"`
	TriCodigo         string                    `json:"tri_codigo"`
	IidValor          string                    `json:"iid_valor"`
	IidMotivoExencion *string                   `json:"iid_motivo_exencion"`
	IidPorcentaje     *OpenETLTributoPorcentaje `json:"iid_porcentaje"`
}

// OpenETLTributoPorcentaje represents tax percentage details.
type OpenETLTributoPorcentaje struct {
	IidBase       string `json:"iid_base"`
	IidPorcentaje string `json:"iid_porcentaje"`
}

// DocumentRegistrationResponse represents the response from document registration.
type DocumentRegistrationResponse struct {
	Message              string              `json:"message"`
	Lote                 string              `json:"lote"`
	DocumentosProcesados []ProcessedDocument `json:"documentos_procesados"`
	DocumentosFallidos   []FailedDocument    `json:"documentos_fallidos"`
}

// ProcessedDocument represents a successfully processed document.
type ProcessedDocument struct {
	CdoID              int    `json:"cdo_id"`
	RfaPrefijo         string `json:"rfa_prefijo"`
	CdoConsecutivo     string `json:"cdo_consecutivo"`
	FechaProcesamiento string `json:"fecha_procesamiento"`
	HoraProcesamiento  string `json:"hora_procesamiento"`
	XmlBase64          string `json:"xml_base64,omitempty"`
	PdfBase64          string `json:"pdf_base64,omitempty"`
}

// FailedDocument represents a document that failed to process.
type FailedDocument struct {
	Documento          string   `json:"documento"`
	Consecutivo        string   `json:"consecutivo"`
	Prefijo            string   `json:"prefijo"`
	Errors             []string `json:"errors"`
	FechaProcesamiento string   `json:"fecha_procesamiento"`
	HoraProcesamiento  string   `json:"hora_procesamiento"`
}
