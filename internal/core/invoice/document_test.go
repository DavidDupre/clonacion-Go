package invoice

import (
	"testing"
	"time"
)

func TestDocument_Struct(t *testing.T) {
	doc := Document{
		OFE:         "123456789",
		Proveedor:   "Test Provider",
		Tipo:        "01",
		Prefijo:     "PRF",
		Consecutivo: "00000001",
		CUFE:        "c1234567890abcdef",
		Fecha:       time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		Hora:        "10:30:00",
		Valor:       150000.50,
		Marca:       false,
	}

	if doc.OFE != "123456789" {
		t.Errorf("expected OFE '123456789', got %q", doc.OFE)
	}

	if doc.Proveedor != "Test Provider" {
		t.Errorf("expected Proveedor 'Test Provider', got %q", doc.Proveedor)
	}

	if doc.Tipo != "01" {
		t.Errorf("expected Tipo '01', got %q", doc.Tipo)
	}

	if doc.Valor != 150000.50 {
		t.Errorf("expected Valor 150000.50, got %f", doc.Valor)
	}
}

func TestDocumentQuery_Struct(t *testing.T) {
	query := DocumentQuery{
		CompanyNit:  "860011153",
		InitialDate: "2025-12-01",
		FinalDate:   "2025-12-31",
	}

	if query.CompanyNit != "860011153" {
		t.Errorf("expected CompanyNit '860011153', got %q", query.CompanyNit)
	}

	if query.InitialDate != "2025-12-01" {
		t.Errorf("expected InitialDate '2025-12-01', got %q", query.InitialDate)
	}

	if query.FinalDate != "2025-12-31" {
		t.Errorf("expected FinalDate '2025-12-31', got %q", query.FinalDate)
	}
}
