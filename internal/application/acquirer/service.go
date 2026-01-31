package acquirer

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"3tcapital/ms_facturacion_core/internal/core/acquirer"
	"3tcapital/ms_facturacion_core/internal/core/dane"
)

// Service orchestrates acquirer-related use cases.
type Service struct {
	repo        acquirer.Repository
	daneService dane.Service
	log         *slog.Logger
}

// NewService creates a new acquirer service with the given repository and DANE service.
func NewService(repo acquirer.Repository, daneService dane.Service, log *slog.Logger) *Service {
	return &Service{
		repo:        repo,
		daneService: daneService,
		log:         log,
	}
}

// CreateAcquirerRequest represents the request to create an acquirer.
type CreateAcquirerRequest struct {
	OfeIdentificacion           string             `json:"ofe_identificacion"`
	AdqIdentificacion           string             `json:"adq_identificacion"`
	AdqTipoAdquirente           *string            `json:"adq_tipo_adquirente"`
	AdqIDPersonalizado          *string            `json:"adq_id_personalizado"`
	AdqInformacionPersonalizada *string            `json:"adq_informacion_personalizada"`
	AdqRazonSocial              string             `json:"adq_razon_social"`
	AdqNombreComercial          *string            `json:"adq_nombre_comercial"`
	AdqPrimerApellido           *string            `json:"adq_primer_apellido"`
	AdqSegundoApellido          *string            `json:"adq_segundo_apellido"`
	AdqPrimerNombre             *string            `json:"adq_primer_nombre"`
	AdqOtrosNombres             *string            `json:"adq_otros_nombres"`
	TdoCodigo                   string             `json:"tdo_codigo"`
	TojCodigo                   string             `json:"toj_codigo"`
	PaiCodigo                   string             `json:"pai_codigo"`
	DepCodigo                   *string            `json:"dep_codigo"`
	MunCodigo                   *string            `json:"mun_codigo"`
	CpoCodigo                   *string            `json:"cpo_codigo"`
	AdqDireccion                *string            `json:"adq_direccion"`
	AdqTelefono                 *string            `json:"adq_telefono"`
	PaiCodigoDomicilioFiscal    *string            `json:"pai_codigo_domicilio_fiscal"`
	DepCodigoDomicilioFiscal    *string            `json:"dep_codigo_domicilio_fiscal"`
	MunCodigoDomicilioFiscal    *string            `json:"mun_codigo_domicilio_fiscal"`
	CpoCodigoDomicilioFiscal    *string            `json:"cpo_codigo_domicilio_fiscal"`
	AdqDireccionDomicilioFiscal *string            `json:"adq_direccion_domicilio_fiscal"`
	AdqNombreContacto           *string            `json:"adq_nombre_contacto"`
	AdqFax                      *string            `json:"adq_fax"`
	AdqNotas                    *string            `json:"adq_notas"`
	AdqCorreo                   *string            `json:"adq_correo"`
	AdqMatriculaMercantil       *string            `json:"adq_matricula_mercantil"`
	AdqCorreosNotificacion      *string            `json:"adq_correos_notificacion"`
	RfiCodigo                   *string            `json:"rfi_codigo"`
	RefCodigo                   []string           `json:"ref_codigo"`
	ResponsableTributos         []string           `json:"responsable_tributos"`
	Contactos                   []acquirer.Contact `json:"contactos"`
}

// CreateAcquirerResponse represents the response from creating an acquirer.
type CreateAcquirerResponse struct {
	Success bool  `json:"success"`
	AdqID   int64 `json:"adq_id"`
}

// UpdateAcquirerRequest represents the request to update an acquirer.
type UpdateAcquirerRequest struct {
	OfeIdentificacion           string             `json:"ofe_identificacion"`
	AdqIdentificacion           string             `json:"adq_identificacion"`
	AdqTipoAdquirente           *string            `json:"adq_tipo_adquirente"`
	AdqIDPersonalizado          *string            `json:"adq_id_personalizado"`
	AdqInformacionPersonalizada *string            `json:"adq_informacion_personalizada"`
	AdqRazonSocial              string             `json:"adq_razon_social"`
	AdqNombreComercial          *string            `json:"adq_nombre_comercial"`
	AdqPrimerApellido           *string            `json:"adq_primer_apellido"`
	AdqSegundoApellido          *string            `json:"adq_segundo_apellido"`
	AdqPrimerNombre             *string            `json:"adq_primer_nombre"`
	AdqOtrosNombres             *string            `json:"adq_otros_nombres"`
	TdoCodigo                   string             `json:"tdo_codigo"`
	TojCodigo                   string             `json:"toj_codigo"`
	PaiCodigo                   string             `json:"pai_codigo"`
	DepCodigo                   *string            `json:"dep_codigo"`
	MunCodigo                   *string            `json:"mun_codigo"`
	CpoCodigo                   *string            `json:"cpo_codigo"`
	AdqDireccion                *string            `json:"adq_direccion"`
	AdqTelefono                 *string            `json:"adq_telefono"`
	PaiCodigoDomicilioFiscal    *string            `json:"pai_codigo_domicilio_fiscal"`
	DepCodigoDomicilioFiscal    *string            `json:"dep_codigo_domicilio_fiscal"`
	MunCodigoDomicilioFiscal    *string            `json:"mun_codigo_domicilio_fiscal"`
	CpoCodigoDomicilioFiscal    *string            `json:"cpo_codigo_domicilio_fiscal"`
	AdqDireccionDomicilioFiscal *string            `json:"adq_direccion_domicilio_fiscal"`
	AdqNombreContacto           *string            `json:"adq_nombre_contacto"`
	AdqFax                      *string            `json:"adq_fax"`
	AdqNotas                    *string            `json:"adq_notas"`
	AdqCorreo                   *string            `json:"adq_correo"`
	AdqMatriculaMercantil       *string            `json:"adq_matricula_mercantil"`
	AdqCorreosNotificacion      *string            `json:"adq_correos_notificacion"`
	RfiCodigo                   *string            `json:"rfi_codigo"`
	RefCodigo                   []string           `json:"ref_codigo"`
	ResponsableTributos         []string           `json:"responsable_tributos"`
	Contactos                   []acquirer.Contact `json:"contactos"`
}

// CreateAcquirer creates a new acquirer.
func (s *Service) CreateAcquirer(ctx context.Context, req CreateAcquirerRequest) (*CreateAcquirerResponse, error) {
	// Validate required fields
	if err := s.validateCreateRequest(req); err != nil {
		return nil, err
	}

	// Check if acquirer already exists
	adqIDPersonalizado := ""
	if req.AdqIDPersonalizado != nil {
		adqIDPersonalizado = *req.AdqIDPersonalizado
	}

	exists, err := s.repo.Exists(ctx, req.OfeIdentificacion, req.AdqIdentificacion, adqIDPersonalizado)
	if err != nil {
		return nil, fmt.Errorf("check acquirer existence: %w", err)
	}

	if exists {
		return nil, fmt.Errorf("el Adquiriente [%s] para el OFE [%s] ya existe", req.AdqIdentificacion, req.OfeIdentificacion)
	}

	// Convert request to domain entity
	acq := s.requestToAcquirer(req)

	// Enrich with DANE data (municipality and department names)
	s.enrichWithDANEData(ctx, &acq)

	// Create acquirer
	id, err := s.repo.Create(ctx, acq)
	if err != nil {
		return nil, fmt.Errorf("create acquirer: %w", err)
	}

	return &CreateAcquirerResponse{
		Success: true,
		AdqID:   id,
	}, nil
}

// UpdateAcquirer updates an existing acquirer.
func (s *Service) UpdateAcquirer(ctx context.Context, ofeIdentificacion, adqIdentificacion, adqIdPersonalizado string, req UpdateAcquirerRequest) error {
	// Validate required fields
	if err := s.validateUpdateRequest(req); err != nil {
		return err
	}

	// Check if acquirer exists
	exists, err := s.repo.Exists(ctx, ofeIdentificacion, adqIdentificacion, adqIdPersonalizado)
	if err != nil {
		return fmt.Errorf("check acquirer existence: %w", err)
	}

	if !exists {
		return fmt.Errorf("el Id del adquirente no existe")
	}

	// Convert request to domain entity
	acq := s.updateRequestToAcquirer(req)

	// Enrich with DANE data (municipality and department names)
	s.enrichWithDANEData(ctx, &acq)

	// Update acquirer
	if err := s.repo.Update(ctx, ofeIdentificacion, adqIdentificacion, adqIdPersonalizado, acq); err != nil {
		return fmt.Errorf("update acquirer: %w", err)
	}

	return nil
}

// validateCreateRequest validates the create request.
func (s *Service) validateCreateRequest(req CreateAcquirerRequest) error {
	// Required field validations
	if req.OfeIdentificacion == "" {
		return fmt.Errorf("ofe_identificacion es requerido")
	}
	if req.AdqIdentificacion == "" {
		return fmt.Errorf("adq_identificacion es requerido")
	}
	if req.TdoCodigo == "" {
		return fmt.Errorf("tdo_codigo es requerido")
	}
	if req.TojCodigo == "" {
		return fmt.Errorf("toj_codigo es requerido")
	}
	if req.PaiCodigo == "" {
		return fmt.Errorf("pai_codigo es requerido")
	}

	// Validate based on tipo de organización jurídica (toj_codigo)
	// "1" = Persona Jurídica, requires razón social or nombre comercial
	// "2" = Persona Natural, requires primer nombre and primer apellido
	if req.TojCodigo == "1" {
		// Persona Jurídica
		if req.AdqRazonSocial == "" && (req.AdqNombreComercial == nil || strings.TrimSpace(*req.AdqNombreComercial) == "") {
			return fmt.Errorf("para persona jurídica, adq_razon_social o adq_nombre_comercial es requerido")
		}
	} else if req.TojCodigo == "2" {
		// Persona Natural
		if req.AdqPrimerNombre == nil || strings.TrimSpace(*req.AdqPrimerNombre) == "" {
			return fmt.Errorf("para persona natural, adq_primer_nombre es requerido")
		}
		if req.AdqPrimerApellido == nil || strings.TrimSpace(*req.AdqPrimerApellido) == "" {
			return fmt.Errorf("para persona natural, adq_primer_apellido es requerido")
		}
	}

	// Length validations for required fields
	if err := validateMaxLength(req.OfeIdentificacion, 20, "ofe_identificacion"); err != nil {
		return err
	}
	if err := validateMaxLength(req.AdqIdentificacion, 20, "adq_identificacion"); err != nil {
		return err
	}
	// Validate adq_razon_social length only if it has a value
	if req.AdqRazonSocial != "" {
		if err := validateMaxLength(req.AdqRazonSocial, 255, "adq_razon_social"); err != nil {
			return err
		}
	}
	if err := validateMaxLength(req.TdoCodigo, 10, "tdo_codigo"); err != nil {
		return err
	}
	if err := validateMaxLength(req.TojCodigo, 10, "toj_codigo"); err != nil {
		return err
	}
	if err := validateMaxLength(req.PaiCodigo, 10, "pai_codigo"); err != nil {
		return err
	}

	// Length validations for optional fields
	if err := validateMaxLengthPtr(req.AdqTipoAdquirente, 2, "adq_tipo_adquirente"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.AdqIDPersonalizado, 100, "adq_id_personalizado"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.AdqNombreComercial, 255, "adq_nombre_comercial"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.AdqPrimerApellido, 100, "adq_primer_apellido"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.AdqSegundoApellido, 100, "adq_segundo_apellido"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.AdqPrimerNombre, 100, "adq_primer_nombre"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.AdqOtrosNombres, 100, "adq_otros_nombres"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.DepCodigo, 10, "dep_codigo"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.MunCodigo, 10, "mun_codigo"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.CpoCodigo, 10, "cpo_codigo"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.AdqTelefono, 20, "adq_telefono"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.PaiCodigoDomicilioFiscal, 10, "pai_codigo_domicilio_fiscal"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.DepCodigoDomicilioFiscal, 10, "dep_codigo_domicilio_fiscal"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.MunCodigoDomicilioFiscal, 10, "mun_codigo_domicilio_fiscal"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.CpoCodigoDomicilioFiscal, 10, "cpo_codigo_domicilio_fiscal"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.AdqFax, 20, "adq_fax"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.AdqCorreo, 255, "adq_correo"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.AdqMatriculaMercantil, 50, "adq_matricula_mercantil"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.RfiCodigo, 10, "rfi_codigo"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.AdqDireccion, 255, "adq_direccion"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.AdqDireccionDomicilioFiscal, 255, "adq_direccion_domicilio_fiscal"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.AdqNombreContacto, 255, "adq_nombre_contacto"); err != nil {
		return err
	}

	// Email format validations
	if err := validateEmailPtr(req.AdqCorreo, "adq_correo"); err != nil {
		return err
	}
	if err := validateEmailListPtr(req.AdqCorreosNotificacion, "adq_correos_notificacion"); err != nil {
		return err
	}

	// Validate contacts if provided
	for i, contact := range req.Contactos {
		if contact.Nombre == "" {
			return fmt.Errorf("contacto %d: con_nombre es requerido", i+1)
		}
		if contact.Tipo == "" {
			return fmt.Errorf("contacto %d: con_tipo es requerido", i+1)
		}
		validTypes := map[string]bool{
			"AccountingContact": true,
			"DeliveryContact":   true,
			"BuyerContact":      true,
		}
		if !validTypes[contact.Tipo] {
			return fmt.Errorf("contacto %d: con_tipo debe ser AccountingContact, DeliveryContact o BuyerContact", i+1)
		}
	}

	return nil
}

// validateUpdateRequest validates the update request.
func (s *Service) validateUpdateRequest(req UpdateAcquirerRequest) error {
	// Required field validations
	if req.OfeIdentificacion == "" {
		return fmt.Errorf("ofe_identificacion es requerido")
	}
	if req.AdqIdentificacion == "" {
		return fmt.Errorf("adq_identificacion es requerido")
	}
	if req.TdoCodigo == "" {
		return fmt.Errorf("tdo_codigo es requerido")
	}
	if req.TojCodigo == "" {
		return fmt.Errorf("toj_codigo es requerido")
	}
	if req.PaiCodigo == "" {
		return fmt.Errorf("pai_codigo es requerido")
	}

	// Validate based on tipo de organización jurídica (toj_codigo)
	// "1" = Persona Jurídica, requires razón social or nombre comercial
	// "2" = Persona Natural, requires primer nombre and primer apellido
	if req.TojCodigo == "1" {
		// Persona Jurídica
		if req.AdqRazonSocial == "" && (req.AdqNombreComercial == nil || strings.TrimSpace(*req.AdqNombreComercial) == "") {
			return fmt.Errorf("para persona jurídica, adq_razon_social o adq_nombre_comercial es requerido")
		}
	} else if req.TojCodigo == "2" {
		// Persona Natural
		if req.AdqPrimerNombre == nil || strings.TrimSpace(*req.AdqPrimerNombre) == "" {
			return fmt.Errorf("para persona natural, adq_primer_nombre es requerido")
		}
		if req.AdqPrimerApellido == nil || strings.TrimSpace(*req.AdqPrimerApellido) == "" {
			return fmt.Errorf("para persona natural, adq_primer_apellido es requerido")
		}
	}

	// Length validations for required fields
	if err := validateMaxLength(req.OfeIdentificacion, 20, "ofe_identificacion"); err != nil {
		return err
	}
	if err := validateMaxLength(req.AdqIdentificacion, 20, "adq_identificacion"); err != nil {
		return err
	}
	// Validate adq_razon_social length only if it has a value
	if req.AdqRazonSocial != "" {
		if err := validateMaxLength(req.AdqRazonSocial, 255, "adq_razon_social"); err != nil {
			return err
		}
	}
	if err := validateMaxLength(req.TdoCodigo, 10, "tdo_codigo"); err != nil {
		return err
	}
	if err := validateMaxLength(req.TojCodigo, 10, "toj_codigo"); err != nil {
		return err
	}
	if err := validateMaxLength(req.PaiCodigo, 10, "pai_codigo"); err != nil {
		return err
	}

	// Length validations for optional fields
	if err := validateMaxLengthPtr(req.AdqTipoAdquirente, 2, "adq_tipo_adquirente"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.AdqIDPersonalizado, 100, "adq_id_personalizado"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.AdqNombreComercial, 255, "adq_nombre_comercial"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.AdqPrimerApellido, 100, "adq_primer_apellido"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.AdqSegundoApellido, 100, "adq_segundo_apellido"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.AdqPrimerNombre, 100, "adq_primer_nombre"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.AdqOtrosNombres, 100, "adq_otros_nombres"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.DepCodigo, 10, "dep_codigo"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.MunCodigo, 10, "mun_codigo"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.CpoCodigo, 10, "cpo_codigo"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.AdqTelefono, 20, "adq_telefono"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.PaiCodigoDomicilioFiscal, 10, "pai_codigo_domicilio_fiscal"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.DepCodigoDomicilioFiscal, 10, "dep_codigo_domicilio_fiscal"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.MunCodigoDomicilioFiscal, 10, "mun_codigo_domicilio_fiscal"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.CpoCodigoDomicilioFiscal, 10, "cpo_codigo_domicilio_fiscal"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.AdqFax, 20, "adq_fax"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.AdqCorreo, 255, "adq_correo"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.AdqMatriculaMercantil, 50, "adq_matricula_mercantil"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.RfiCodigo, 10, "rfi_codigo"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.AdqDireccion, 255, "adq_direccion"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.AdqDireccionDomicilioFiscal, 255, "adq_direccion_domicilio_fiscal"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.AdqNombreContacto, 255, "adq_nombre_contacto"); err != nil {
		return err
	}

	// Email format validations
	if err := validateEmailPtr(req.AdqCorreo, "adq_correo"); err != nil {
		return err
	}
	if err := validateEmailListPtr(req.AdqCorreosNotificacion, "adq_correos_notificacion"); err != nil {
		return err
	}

	// Validate contacts if provided
	for i, contact := range req.Contactos {
		if contact.Nombre == "" {
			return fmt.Errorf("contacto %d: con_nombre es requerido", i+1)
		}
		if contact.Tipo == "" {
			return fmt.Errorf("contacto %d: con_tipo es requerido", i+1)
		}
		validTypes := map[string]bool{
			"AccountingContact": true,
			"DeliveryContact":   true,
			"BuyerContact":      true,
		}
		if !validTypes[contact.Tipo] {
			return fmt.Errorf("contacto %d: con_tipo debe ser AccountingContact, DeliveryContact o BuyerContact", i+1)
		}
	}

	return nil
}

// validateMaxLength validates that a string value does not exceed the maximum length.
func validateMaxLength(value string, maxLen int, fieldName string) error {
	if len(value) > maxLen {
		return fmt.Errorf("%s excede la longitud máxima de %d caracteres", fieldName, maxLen)
	}
	return nil
}

// validateMaxLengthPtr validates that a pointer string value does not exceed the maximum length.
func validateMaxLengthPtr(value *string, maxLen int, fieldName string) error {
	if value != nil && *value != "" {
		return validateMaxLength(*value, maxLen, fieldName)
	}
	return nil
}

// validateEmail validates that a string is a valid email format.
func validateEmail(email string, fieldName string) error {
	if email == "" {
		return nil // Empty email is allowed if field is optional
	}
	// Simple email regex pattern
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	if !emailRegex.MatchString(email) {
		return fmt.Errorf("%s tiene un formato de correo inválido", fieldName)
	}
	return nil
}

// validateEmailPtr validates that a pointer string is a valid email format.
func validateEmailPtr(email *string, fieldName string) error {
	if email != nil && *email != "" {
		return validateEmail(*email, fieldName)
	}
	return nil
}

// validateEmailList validates that a comma-separated string contains valid emails.
func validateEmailList(emails string, fieldName string) error {
	if emails == "" {
		return nil // Empty email list is allowed if field is optional
	}
	emailList := strings.Split(emails, ",")
	for i, email := range emailList {
		email = strings.TrimSpace(email)
		if email != "" {
			if err := validateEmail(email, fmt.Sprintf("%s (email %d)", fieldName, i+1)); err != nil {
				return err
			}
		}
	}
	return nil
}

// validateEmailListPtr validates that a pointer string contains valid comma-separated emails.
func validateEmailListPtr(emails *string, fieldName string) error {
	if emails != nil && *emails != "" {
		return validateEmailList(*emails, fieldName)
	}
	return nil
}

// requestToAcquirer converts a create request to a domain entity.
func (s *Service) requestToAcquirer(req CreateAcquirerRequest) acquirer.Acquirer {
	acq := acquirer.Acquirer{
		OfeIdentificacion:           req.OfeIdentificacion,
		AdqIdentificacion:           req.AdqIdentificacion,
		AdqTipoAdquirente:           req.AdqTipoAdquirente,
		AdqIDPersonalizado:          req.AdqIDPersonalizado,
		AdqInformacionPersonalizada: req.AdqInformacionPersonalizada,
		AdqRazonSocial:              req.AdqRazonSocial,
		AdqNombreComercial:          req.AdqNombreComercial,
		AdqPrimerApellido:           req.AdqPrimerApellido,
		AdqSegundoApellido:          req.AdqSegundoApellido,
		AdqPrimerNombre:             req.AdqPrimerNombre,
		AdqOtrosNombres:             req.AdqOtrosNombres,
		TdoCodigo:                   req.TdoCodigo,
		TojCodigo:                   req.TojCodigo,
		PaiCodigo:                   req.PaiCodigo,
		DepCodigo:                   req.DepCodigo,
		MunCodigo:                   req.MunCodigo,
		CpoCodigo:                   req.CpoCodigo,
		AdqDireccion:                req.AdqDireccion,
		AdqTelefono:                 req.AdqTelefono,
		PaiCodigoDomicilioFiscal:    req.PaiCodigoDomicilioFiscal,
		DepCodigoDomicilioFiscal:    req.DepCodigoDomicilioFiscal,
		MunCodigoDomicilioFiscal:    req.MunCodigoDomicilioFiscal,
		CpoCodigoDomicilioFiscal:    req.CpoCodigoDomicilioFiscal,
		AdqDireccionDomicilioFiscal: req.AdqDireccionDomicilioFiscal,
		AdqNombreContacto:           req.AdqNombreContacto,
		AdqFax:                      req.AdqFax,
		AdqNotas:                    req.AdqNotas,
		AdqCorreo:                   req.AdqCorreo,
		AdqMatriculaMercantil:       req.AdqMatriculaMercantil,
		AdqCorreosNotificacion:      req.AdqCorreosNotificacion,
		RfiCodigo:                   req.RfiCodigo,
		RefCodigo:                   req.RefCodigo,
		ResponsableTributos:         req.ResponsableTributos,
		Contactos:                   req.Contactos,
	}

	// Normalize empty strings to nil for optional fields
	if req.AdqIDPersonalizado != nil && strings.TrimSpace(*req.AdqIDPersonalizado) == "" {
		acq.AdqIDPersonalizado = nil
	}
	if req.AdqInformacionPersonalizada != nil && strings.TrimSpace(*req.AdqInformacionPersonalizada) == "" {
		acq.AdqInformacionPersonalizada = nil
	}

	return acq
}

// updateRequestToAcquirer converts an update request to a domain entity.
func (s *Service) updateRequestToAcquirer(req UpdateAcquirerRequest) acquirer.Acquirer {
	acq := acquirer.Acquirer{
		OfeIdentificacion:           req.OfeIdentificacion,
		AdqIdentificacion:           req.AdqIdentificacion,
		AdqTipoAdquirente:           req.AdqTipoAdquirente,
		AdqIDPersonalizado:          req.AdqIDPersonalizado,
		AdqInformacionPersonalizada: req.AdqInformacionPersonalizada,
		AdqRazonSocial:              req.AdqRazonSocial,
		AdqNombreComercial:          req.AdqNombreComercial,
		AdqPrimerApellido:           req.AdqPrimerApellido,
		AdqSegundoApellido:          req.AdqSegundoApellido,
		AdqPrimerNombre:             req.AdqPrimerNombre,
		AdqOtrosNombres:             req.AdqOtrosNombres,
		TdoCodigo:                   req.TdoCodigo,
		TojCodigo:                   req.TojCodigo,
		PaiCodigo:                   req.PaiCodigo,
		DepCodigo:                   req.DepCodigo,
		MunCodigo:                   req.MunCodigo,
		CpoCodigo:                   req.CpoCodigo,
		AdqDireccion:                req.AdqDireccion,
		AdqTelefono:                 req.AdqTelefono,
		PaiCodigoDomicilioFiscal:    req.PaiCodigoDomicilioFiscal,
		DepCodigoDomicilioFiscal:    req.DepCodigoDomicilioFiscal,
		MunCodigoDomicilioFiscal:    req.MunCodigoDomicilioFiscal,
		CpoCodigoDomicilioFiscal:    req.CpoCodigoDomicilioFiscal,
		AdqDireccionDomicilioFiscal: req.AdqDireccionDomicilioFiscal,
		AdqNombreContacto:           req.AdqNombreContacto,
		AdqFax:                      req.AdqFax,
		AdqNotas:                    req.AdqNotas,
		AdqCorreo:                   req.AdqCorreo,
		AdqMatriculaMercantil:       req.AdqMatriculaMercantil,
		AdqCorreosNotificacion:      req.AdqCorreosNotificacion,
		RfiCodigo:                   req.RfiCodigo,
		RefCodigo:                   req.RefCodigo,
		ResponsableTributos:         req.ResponsableTributos,
		Contactos:                   req.Contactos,
	}

	// Normalize empty strings to nil for optional fields
	if req.AdqIDPersonalizado != nil && strings.TrimSpace(*req.AdqIDPersonalizado) == "" {
		acq.AdqIDPersonalizado = nil
	}
	if req.AdqInformacionPersonalizada != nil && strings.TrimSpace(*req.AdqInformacionPersonalizada) == "" {
		acq.AdqInformacionPersonalizada = nil
	}

	return acq
}

// ListAcquirersResponse represents the response from listing acquirers.
type ListAcquirersResponse struct {
	Total     int            `json:"total"`
	Filtrados int            `json:"filtrados"`
	Data      []AdquirenteDTO `json:"data"`
}

// ListAcquirers lists acquirers with pagination, search, and sorting.
func (s *Service) ListAcquirers(ctx context.Context, start, length int, buscar, columnaOrden, ordenDireccion string) (*ListAcquirersResponse, error) {
	// Default values
	if start < 0 {
		start = 0
	}
	if columnaOrden == "" {
		columnaOrden = "codigo"
	}
	if ordenDireccion == "" {
		ordenDireccion = "asc"
	}

	acquirers, filtered, err := s.repo.List(ctx, start, length, buscar, columnaOrden, ordenDireccion)
	if err != nil {
		return nil, fmt.Errorf("list acquirers: %w", err)
	}

	// Calculate total (if length == -1, filtered is the total)
	total := filtered
	if length != -1 {
		// Get total count without pagination
		_, total, err = s.repo.List(ctx, 0, -1, buscar, columnaOrden, ordenDireccion)
		if err != nil {
			return nil, fmt.Errorf("get total count: %w", err)
		}
	}

	return &ListAcquirersResponse{
		Total:     total,
		Filtrados: filtered,
		Data:      ToAdquirenteDTOList(acquirers),
	}, nil
}

// SearchAcquirer searches for acquirers by field, value, OFE, and filter type.
func (s *Service) SearchAcquirer(ctx context.Context, campoBuscar, valorBuscar, valorOfe, filtroColumnas string) ([]AdquirenteDTO, error) {
	// Validate campoBuscar is not empty
	if campoBuscar == "" {
		return nil, fmt.Errorf("campoBuscar es requerido")
	}

	// Validate valorBuscar is not empty
	if valorBuscar == "" {
		return nil, fmt.Errorf("valorBuscar es requerido")
	}

	// Validate valorOfe is not empty
	if valorOfe == "" {
		return nil, fmt.Errorf("valorOfe es requerido")
	}

	// Validate filtroColumnas
	if filtroColumnas != "exacto" && filtroColumnas != "basico" {
		return nil, fmt.Errorf("filtroColumnas debe ser 'exacto' o 'basico'")
	}

	// Call repository to search
	acquirers, err := s.repo.Search(ctx, campoBuscar, valorBuscar, valorOfe, filtroColumnas)
	if err != nil {
		return nil, fmt.Errorf("search acquirers: %w", err)
	}

	s.log.Debug("SearchAcquirer found acquirers",
		"campoBuscar", campoBuscar,
		"valorBuscar", valorBuscar,
		"valorOfe", valorOfe,
		"filtroColumnas", filtroColumnas,
		"count", len(acquirers))

	// Convert to DTO
	dtos := ToAdquirenteDTOList(acquirers)
	s.log.Debug("SearchAcquirer converted to DTOs", "dto_count", len(dtos))
	return dtos, nil
}

// buildDivipolaCode constructs a DIVIPOLA code by concatenating department code and municipality code.
// The municipality code is zero-padded to 3 digits if necessary.
// Example: buildDivipolaCode("05", "1") returns "05001"
// Example: buildDivipolaCode("05", "001") returns "05001"
func buildDivipolaCode(depCodigo, munCodigo string) string {
	// Ensure mun_codigo has exactly 3 digits (zero-pad if necessary)
	// Remove any leading/trailing whitespace first
	munCodigo = strings.TrimSpace(munCodigo)

	// Pad with zeros on the left to ensure 3 digits
	munPadded := munCodigo
	if len(munCodigo) < 3 {
		// Pad with zeros on the left
		munPadded = strings.Repeat("0", 3-len(munCodigo)) + munCodigo
	}

	return depCodigo + munPadded
}

// enrichWithDANEData enriches the acquirer entity with municipality and department names
// from DANE API based on DIVIPOLA codes (concatenation of dep_codigo + mun_codigo).
// If DANE service is unavailable or codes are invalid, it logs a warning but does not fail
// the operation (graceful degradation).
func (s *Service) enrichWithDANEData(ctx context.Context, acq *acquirer.Acquirer) {
	// Query DANE for regular municipality if both dep_codigo and mun_codigo are provided
	if acq.DepCodigo != nil && *acq.DepCodigo != "" && acq.MunCodigo != nil && *acq.MunCodigo != "" {
		codigoDivipola := buildDivipolaCode(*acq.DepCodigo, *acq.MunCodigo)
		municipality, err := s.daneService.GetMunicipalityByCode(ctx, codigoDivipola)
		if err != nil {
			s.log.Warn("Failed to get municipality name from DANE",
				"error", err,
				"dep_codigo", *acq.DepCodigo,
				"mun_codigo", *acq.MunCodigo,
				"codigo_divipola", codigoDivipola,
				"adq_identificacion", acq.AdqIdentificacion)
		} else {
			acq.MunNombre = &municipality.Nombre
			acq.DepNombre = &municipality.DepNombre
			// Do NOT update dep_codigo or mun_codigo - keep the original values from the payload
			// DANE returns the full DIVIPOLA code (e.g., "05001"), but we store the codes as they come in the payload
		}
	}

	// Query DANE for fiscal municipality if both dep_codigo_domicilio_fiscal and mun_codigo_domicilio_fiscal are provided
	if acq.DepCodigoDomicilioFiscal != nil && *acq.DepCodigoDomicilioFiscal != "" &&
		acq.MunCodigoDomicilioFiscal != nil && *acq.MunCodigoDomicilioFiscal != "" {
		codigoDivipola := buildDivipolaCode(*acq.DepCodigoDomicilioFiscal, *acq.MunCodigoDomicilioFiscal)
		municipality, err := s.daneService.GetMunicipalityByCode(ctx, codigoDivipola)
		if err != nil {
			s.log.Warn("Failed to get fiscal municipality name from DANE",
				"error", err,
				"dep_codigo_domicilio_fiscal", *acq.DepCodigoDomicilioFiscal,
				"mun_codigo_domicilio_fiscal", *acq.MunCodigoDomicilioFiscal,
				"codigo_divipola", codigoDivipola,
				"adq_identificacion", acq.AdqIdentificacion)
		} else {
			acq.MunNombreDomicilioFiscal = &municipality.Nombre
			acq.DepNombreDomicilioFiscal = &municipality.DepNombre
			// Do NOT update dep_codigo_domicilio_fiscal or mun_codigo_domicilio_fiscal - keep the original values from the payload
			// DANE returns the full DIVIPOLA code (e.g., "05001"), but we store the codes as they come in the payload
		}
	}
}
