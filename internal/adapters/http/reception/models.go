package reception

// ConsultaDocumentosResponse represents the legacy response format
type ConsultaDocumentosResponse struct {
	Data []LegacyDocumentData `json:"data"`
}

// LegacyDocumentData represents a single document in legacy format
type LegacyDocumentData struct {
	ID                *int              `json:"id"`                 // null por ahora
	OfeIdentificacion string            `json:"ofe_identificacion"` // Receptor.NumeroDoc (cliente / OFE para legacy)
	ProIdentificacion string            `json:"pro_identificacion"` // Emisor.NumeroDoc (proveedor)
	CdoClasificacion  string            `json:"cdo_clasificacion"`  // Mapeado de DocumentTypeId
	Resolucion        *string           `json:"resolucion"`         // null por ahora
	Prefijo           string            `json:"prefijo"`            // NumeroDocumento.Serie
	Consecutivo       string            `json:"consecutivo"`        // NumeroDocumento.Folio
	FechaDocumento    string            `json:"fecha_documento"`    // NumeroDocumento.FechaEmision
	HoraDocumento     *string           `json:"hora_documento"`     // null por ahora
	Estado            string            `json:"estado"`             // "ACTIVO" estático
	CUFE              string            `json:"cufe"`               // UUID
	QR                *string           `json:"qr"`                 // null por ahora
	SignatureValue    *string           `json:"signaturevalue"`     // null por ahora
	UltimoEstado      *UltimoEstado     `json:"ultimo_estado"`      // Último estado del documento
	HistoricoEstados  []HistoricoEstado `json:"historico_estados"`  // Historial de estados
}

// UltimoEstado represents the last state of a document
type UltimoEstado struct {
	Estado           string `json:"estado"`
	Resultado        string `json:"resultado"`
	MensajeResultado string `json:"mensaje_resultado"`
	Archivo          string `json:"archivo"`
	XML              string `json:"xml"`
	Fecha            string `json:"fecha"`
}

// HistoricoEstado represents an event history entry
type HistoricoEstado struct {
	Estado           string  `json:"estado"`
	Resultado        string  `json:"resultado"`
	MensajeResultado *string `json:"mensaje_resultado"`
	Archivo          string  `json:"archivo"`
	XML              string  `json:"xml"`
	Fecha            string  `json:"fecha"`
}

// RegistrarEventoResponse represents the response for event registration
type RegistrarEventoResponse struct {
	Message  string   `json:"message"`  // Always "Solicitud Procesada"
	Exitosos []string `json:"exitosos"`  // Array of success messages, never null
	Fallidos []string `json:"fallidos"` // Array of failure messages, never null
}

// DocumentEventResult represents the result for a single document
type DocumentEventResult struct {
	CdoCufe      string              `json:"cdo_cufe"`
	Status       string              `json:"status"` // "success" or "error"
	Message      string              `json:"message"`
	ErrorDetails *EventErrorDetails  `json:"error_details,omitempty"`
	EventResults []EventTypeResult   `json:"event_results,omitempty"`
}

// EventErrorDetails provides detailed error information
type EventErrorDetails struct {
	Stage       string `json:"stage"` // "validation", "document_info", "event_registration"
	ErrorCode   string `json:"error_code,omitempty"`
	Description string `json:"description"`
}

// EventTypeResult represents the result for a specific event type
type EventTypeResult struct {
	TipoEvento      string `json:"tipo_evento"`
	Mensaje         string `json:"mensaje"`
	MensajeError    string `json:"mensaje_error,omitempty"`
	CodigoRespuesta string `json:"codigo_respuesta"`
}

// ConsultaDocumentosByProviderRequest represents the request body for querying documents by provider parameters
type ConsultaDocumentosByProviderRequest struct {
	Proveedor   string `json:"proveedor"`   // Provider NIT (SupplierNit)
	Consecutivo string `json:"consecutivo"`  // Document consecutive number
	Prefijo     string `json:"prefijo"`      // Document prefix
	Ofe         string `json:"ofe"`          // OFE NIT (CompanyNit)
	Tipo        string `json:"tipo"`         // Document type code (optional, for validation)
}
