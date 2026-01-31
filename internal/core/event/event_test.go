package event

import (
	"testing"
	"time"
)

func TestToRadianCode(t *testing.T) {
	tests := []struct {
		name      string
		eventType EventType
		want      RadianCode
		wantErr   bool
	}{
		{
			name:      "ACUSE to 030",
			eventType: EventTypeAcuse,
			want:      RadianCodeAcuse,
			wantErr:   false,
		},
		{
			name:      "RECIBOBIEN to 032",
			eventType: EventTypeReciboBien,
			want:      RadianCodeReciboBien,
			wantErr:   false,
		},
		{
			name:      "ACEPTACION to 033",
			eventType: EventTypeAceptacion,
			want:      RadianCodeAceptacion,
			wantErr:   false,
		},
		{
			name:      "RECLAMO to 031",
			eventType: EventTypeReclamo,
			want:      RadianCodeReclamo,
			wantErr:   false,
		},
		{
			name:      "invalid event type",
			eventType: EventType("INVALID"),
			want:      "",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ToRadianCode(tt.eventType)
			if (err != nil) != tt.wantErr {
				t.Errorf("ToRadianCode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ToRadianCode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateEventType(t *testing.T) {
	tests := []struct {
		name      string
		eventType EventType
		want      bool
	}{
		{"valid ACUSE", EventTypeAcuse, true},
		{"valid RECIBOBIEN", EventTypeReciboBien, true},
		{"valid ACEPTACION", EventTypeAceptacion, true},
		{"valid RECLAMO", EventTypeReclamo, true},
		{"invalid type", EventType("INVALID"), false},
		{"empty type", EventType(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidateEventType(tt.eventType); got != tt.want {
				t.Errorf("ValidateEventType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateRejectionCode(t *testing.T) {
	tests := []struct {
		name string
		code RejectionCode
		want bool
	}{
		{"valid 01", RejectionCodeInconsistencias, true},
		{"valid 02", RejectionCodeNoEntregadaTotalmente, true},
		{"valid 03", RejectionCodeNoEntregadaParcialmente, true},
		{"valid 04", RejectionCodeServicioNoPrestado, true},
		{"invalid code", RejectionCode("99"), false},
		{"empty code", RejectionCode(""), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidateRejectionCode(tt.code); got != tt.want {
				t.Errorf("ValidateRejectionCode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRequiresRejectionCode(t *testing.T) {
	tests := []struct {
		name      string
		eventType EventType
		want      bool
	}{
		{"RECLAMO requires rejection code", EventTypeReclamo, true},
		{"ACUSE does not require", EventTypeAcuse, false},
		{"RECIBOBIEN does not require", EventTypeReciboBien, false},
		{"ACEPTACION does not require", EventTypeAceptacion, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RequiresRejectionCode(tt.eventType); got != tt.want {
				t.Errorf("RequiresRejectionCode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEvent_Validate(t *testing.T) {
	validRejectionCode := RejectionCodeInconsistencias
	invalidRejectionCode := RejectionCode("99")

	tests := []struct {
		name    string
		event   Event
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid ACUSE event",
			event: Event{
				EventType:              EventTypeAcuse,
				DocumentNumber:         "FAC12345",
				NombreGenerador:        "Mauricio",
				ApellidoGenerador:      "Alemán",
				IdentificacionGenerador: "1061239585",
				EventGenerationDate:    time.Now(),
			},
			wantErr: false,
		},
		{
			name: "valid RECLAMO event with rejection code",
			event: Event{
				EventType:              EventTypeReclamo,
				DocumentNumber:         "FAC12345",
				NombreGenerador:        "Mauricio",
				ApellidoGenerador:      "Alemán",
				IdentificacionGenerador: "1061239585",
				RejectionCode:          &validRejectionCode,
				EventGenerationDate:    time.Now(),
			},
			wantErr: false,
		},
		{
			name: "invalid event type",
			event: Event{
				EventType:              EventType("INVALID"),
				DocumentNumber:         "FAC12345",
				NombreGenerador:        "Mauricio",
				ApellidoGenerador:      "Alemán",
				IdentificacionGenerador: "1061239585",
				EventGenerationDate:    time.Now(),
			},
			wantErr: true,
			errMsg:  "invalid event type",
		},
		{
			name: "missing document number",
			event: Event{
				EventType:              EventTypeAcuse,
				DocumentNumber:         "",
				NombreGenerador:        "Mauricio",
				ApellidoGenerador:      "Alemán",
				IdentificacionGenerador: "1061239585",
				EventGenerationDate:    time.Now(),
			},
			wantErr: true,
			errMsg:  "document number is required",
		},
		{
			name: "missing nombre generador",
			event: Event{
				EventType:              EventTypeAcuse,
				DocumentNumber:         "FAC12345",
				NombreGenerador:        "",
				ApellidoGenerador:      "Alemán",
				IdentificacionGenerador: "1061239585",
				EventGenerationDate:    time.Now(),
			},
			wantErr: true,
			errMsg:  "nombre generador is required",
		},
		{
			name: "missing apellido generador",
			event: Event{
				EventType:              EventTypeAcuse,
				DocumentNumber:         "FAC12345",
				NombreGenerador:        "Mauricio",
				ApellidoGenerador:      "",
				IdentificacionGenerador: "1061239585",
				EventGenerationDate:    time.Now(),
			},
			wantErr: true,
			errMsg:  "apellido generador is required",
		},
		{
			name: "missing identificacion generador",
			event: Event{
				EventType:              EventTypeAcuse,
				DocumentNumber:         "FAC12345",
				NombreGenerador:        "Mauricio",
				ApellidoGenerador:      "Alemán",
				IdentificacionGenerador: "",
				EventGenerationDate:    time.Now(),
			},
			wantErr: true,
			errMsg:  "identificacion generador is required",
		},
		{
			name: "RECLAMO without rejection code",
			event: Event{
				EventType:              EventTypeReclamo,
				DocumentNumber:         "FAC12345",
				NombreGenerador:        "Mauricio",
				ApellidoGenerador:      "Alemán",
				IdentificacionGenerador: "1061239585",
				RejectionCode:          nil,
				EventGenerationDate:    time.Now(),
			},
			wantErr: true,
			errMsg:  "rejection code is required",
		},
		{
			name: "RECLAMO with invalid rejection code",
			event: Event{
				EventType:              EventTypeReclamo,
				DocumentNumber:         "FAC12345",
				NombreGenerador:        "Mauricio",
				ApellidoGenerador:      "Alemán",
				IdentificacionGenerador: "1061239585",
				RejectionCode:          &invalidRejectionCode,
				EventGenerationDate:    time.Now(),
			},
			wantErr: true,
			errMsg:  "invalid rejection code",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.event.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Event.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errMsg != "" {
				if err == nil || err.Error() == "" {
					t.Errorf("Event.Validate() expected error message containing %q, got %v", tt.errMsg, err)
				}
			}
		})
	}
}
