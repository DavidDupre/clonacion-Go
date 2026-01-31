package acquirer

// Contact represents a contact associated with an acquirer.
type Contact struct {
	Nombre        string `json:"con_nombre"`
	Direccion     string `json:"con_direccion"`
	Telefono      string `json:"con_telefono"`
	Correo        string `json:"con_correo"`
	Observaciones string `json:"con_observaciones"`
	Tipo          string `json:"con_tipo"` // AccountingContact, DeliveryContact, BuyerContact
}
