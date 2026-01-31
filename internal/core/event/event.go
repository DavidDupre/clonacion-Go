package event

import (
	"fmt"
	"time"
)

// EventType represents the type of event to register.
type EventType string

const (
	// EventTypeAcuse represents "Acuse de recibo de factura electrónica de venta"
	EventTypeAcuse EventType = "ACUSE"
	// EventTypeReciboBien represents "Recibo del bien o prestación del servicio"
	EventTypeReciboBien EventType = "RECIBOBIEN"
	// EventTypeAceptacion represents "Aceptación expresa"
	EventTypeAceptacion EventType = "ACEPTACION"
	// EventTypeReclamo represents "Reclamo de factura electrónica de venta"
	EventTypeReclamo EventType = "RECLAMO"
)

// RadianCode represents the DIAN Radian code for events.
type RadianCode string

const (
	// RadianCodeAcuse is the code for "Acuse de recibo de factura electrónica de venta"
	RadianCodeAcuse RadianCode = "030"
	// RadianCodeReclamo is the code for "Reclamo de factura electrónica de venta"
	RadianCodeReclamo RadianCode = "031"
	// RadianCodeReciboBien is the code for "Recibo del bien o prestación del servicio"
	RadianCodeReciboBien RadianCode = "032"
	// RadianCodeAceptacion is the code for "Aceptación expresa"
	RadianCodeAceptacion RadianCode = "033"
)

// RejectionCode represents the rejection code for RECLAMO events.
type RejectionCode string

const (
	// RejectionCodeInconsistencias represents "Documento con inconsistencias"
	RejectionCodeInconsistencias RejectionCode = "01"
	// RejectionCodeNoEntregadaTotalmente represents "Mercancía no entregada totalmente"
	RejectionCodeNoEntregadaTotalmente RejectionCode = "02"
	// RejectionCodeNoEntregadaParcialmente represents "Mercancía no entregada parcialmente"
	RejectionCodeNoEntregadaParcialmente RejectionCode = "03"
	// RejectionCodeServicioNoPrestado represents "Servicio no prestado"
	RejectionCodeServicioNoPrestado RejectionCode = "04"
)

// Event represents a Radian event to be registered.
type Event struct {
	EventType              EventType
	DocumentNumber         string
	NombreGenerador        string
	ApellidoGenerador      string
	IdentificacionGenerador string
	RejectionCode          *RejectionCode
	EventGenerationDate   time.Time
}

// ToRadianCode translates an EventType to its corresponding RadianCode.
func ToRadianCode(eventType EventType) (RadianCode, error) {
	switch eventType {
	case EventTypeAcuse:
		return RadianCodeAcuse, nil
	case EventTypeReciboBien:
		return RadianCodeReciboBien, nil
	case EventTypeAceptacion:
		return RadianCodeAceptacion, nil
	case EventTypeReclamo:
		return RadianCodeReclamo, nil
	default:
		return "", fmt.Errorf("invalid event type: %s", eventType)
	}
}

// ValidateEventType checks if the event type is valid.
func ValidateEventType(eventType EventType) bool {
	switch eventType {
	case EventTypeAcuse, EventTypeReciboBien, EventTypeAceptacion, EventTypeReclamo:
		return true
	default:
		return false
	}
}

// ValidateRejectionCode checks if the rejection code is valid.
func ValidateRejectionCode(code RejectionCode) bool {
	switch code {
	case RejectionCodeInconsistencias, RejectionCodeNoEntregadaTotalmente,
		RejectionCodeNoEntregadaParcialmente, RejectionCodeServicioNoPrestado:
		return true
	default:
		return false
	}
}

// RequiresRejectionCode checks if the event type requires a rejection code.
func RequiresRejectionCode(eventType EventType) bool {
	return eventType == EventTypeReclamo
}

// Validate validates the event according to business rules.
func (e *Event) Validate() error {
	if !ValidateEventType(e.EventType) {
		return fmt.Errorf("invalid event type: %s", e.EventType)
	}

	if e.DocumentNumber == "" {
		return fmt.Errorf("document number is required")
	}

	if e.NombreGenerador == "" {
		return fmt.Errorf("nombre generador is required")
	}

	if e.ApellidoGenerador == "" {
		return fmt.Errorf("apellido generador is required")
	}

	if e.IdentificacionGenerador == "" {
		return fmt.Errorf("identificacion generador is required")
	}

	if RequiresRejectionCode(e.EventType) {
		if e.RejectionCode == nil {
			return fmt.Errorf("rejection code is required for RECLAMO events")
		}
		if !ValidateRejectionCode(*e.RejectionCode) {
			return fmt.Errorf("invalid rejection code: %s", *e.RejectionCode)
		}
	}

	return nil
}
