package numrot

// numrotDocumentInfoResponse represents the response from Numrot DocumentInfo API
type numrotDocumentInfoResponse struct {
	DocumentInfo      []NumrotDocumentInfoData `json:"DocumentInfo"`
	StatusCode        string                   `json:"StatusCode"`
	StatusDescription string                   `json:"StatusDescription"`
}

// NumrotDocumentInfoData represents a single document in DocumentInfo response (exported for transformer)
type NumrotDocumentInfoData struct {
	DocumentTypeId   string                `json:"DocumentTypeId"`
	DocumentTypeName string                `json:"DocumentTypeName"`
	Emisor           numrotEmisor          `json:"Emisor"`
	Estado           map[string]string     `json:"Estado"`
	NumeroDocumento  numrotNumeroDocumento `json:"NumeroDocumento"`
	Receptor         numrotReceptor        `json:"Receptor"`
	TotalEImpuestos  numrotTotalEImpuestos `json:"TotalEImpuestos"`
	UUID             string                `json:"UUID"`
	// Campos adicionales de SearchEstadosDIAN
	QR             string `json:"QR,omitempty"`
	SignatureValue string `json:"SignatureValue,omitempty"`
	Resolucion     string `json:"Resolucion,omitempty"`
	HoraDocumento  string `json:"HoraDocumento,omitempty"`
	// Campos opcionales para expansión futura
	DocumentTags    []interface{} `json:"DocumentTags"`
	Eventos         []interface{} `json:"Eventos"`
	Referencias     []interface{} `json:"Referencias"`
	ValidacionesDoc []interface{} `json:"ValidacionesDoc"`
	LegitimoTenedor struct {
		Nombre string `json:"Nombre"`
	} `json:"LegitimoTenedor"`
}

// numrotEmisor represents the document issuer
type numrotEmisor struct {
	Nombre    string `json:"Nombre"`
	NumeroDoc string `json:"NumeroDoc"`
}

// numrotReceptor represents the document receiver
type numrotReceptor struct {
	Nombre    string `json:"Nombre"`
	NumeroDoc string `json:"NumeroDoc"`
	TipoDoc   string `json:"TipoDoc"`
}

// numrotNumeroDocumento represents the document number details
type numrotNumeroDocumento struct {
	FechaEmision string `json:"FechaEmision"`
	Folio        string `json:"Folio"`
	Serie        string `json:"Serie"`
}

// numrotTotalEImpuestos represents tax and total amounts
type numrotTotalEImpuestos struct {
	Iva   float64 `json:"Iva"`
	Total float64 `json:"Total"`
}
