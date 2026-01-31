package provider

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"3tcapital/ms_facturacion_core/internal/core/provider"
)

// Service orchestrates provider-related use cases.
type Service struct {
	repo provider.Repository
}

// NewService creates a new provider service with the given repository.
func NewService(repo provider.Repository) *Service {
	return &Service{
		repo: repo,
	}
}

// CreateProviderRequest represents the request to create a provider.
type CreateProviderRequest struct {
	OfeIdentificacion           string   `json:"ofe_identificacion"`
	ProIdentificacion           string   `json:"pro_identificacion"`
	ProIDPersonalizado          *string  `json:"pro_id_personalizado"`
	ProRazonSocial              *string  `json:"pro_razon_social"`
	ProNombreComercial          *string  `json:"pro_nombre_comercial"`
	ProPrimerApellido           *string  `json:"pro_primer_apellido"`
	ProSegundoApellido          *string  `json:"pro_segundo_apellido"`
	ProPrimerNombre             *string  `json:"pro_primer_nombre"`
	ProOtrosNombres             *string  `json:"pro_otros_nombres"`
	TdoCodigo                   string   `json:"tdo_codigo"`
	TojCodigo                   string   `json:"toj_codigo"`
	PaiCodigo                   *string  `json:"pai_codigo"`
	DepCodigo                   *string  `json:"dep_codigo"`
	MunCodigo                   *string  `json:"mun_codigo"`
	CpoCodigo                   *string  `json:"cpo_codigo"`
	ProDireccion                *string  `json:"pro_direccion"`
	ProTelefono                 *string  `json:"pro_telefono"`
	PaiCodigoDomicilioFiscal    *string  `json:"pai_codigo_domicilio_fiscal"`
	DepCodigoDomicilioFiscal    *string  `json:"dep_codigo_domicilio_fiscal"`
	MunCodigoDomicilioFiscal    *string  `json:"mun_codigo_domicilio_fiscal"`
	CpoCodigoDomicilioFiscal    *string  `json:"cpo_codigo_domicilio_fiscal"`
	ProDireccionDomicilioFiscal *string  `json:"pro_direccion_domicilio_fiscal"`
	ProCorreo                   string   `json:"pro_correo"`
	ProCorreosNotificacion      *string  `json:"pro_correos_notificacion"`
	ProMatriculaMercantil       *string  `json:"pro_matricula_mercantil"`
	ProUsuariosRecepcion        []string `json:"pro_usuarios_recepcion"`
	RfiCodigo                   *string  `json:"rfi_codigo"`
	RefCodigo                   []string `json:"ref_codigo"`
	Estado                      *string  `json:"estado"`
}

// CreateProviderResponse represents the response from creating a provider.
type CreateProviderResponse struct {
	Success bool  `json:"success"`
	ProID   int64 `json:"pro_id"`
}

// UpdateProviderRequest represents the request to update a provider.
type UpdateProviderRequest struct {
	OfeIdentificacion           string   `json:"ofe_identificacion"`
	ProIdentificacion           string   `json:"pro_identificacion"`
	ProIDPersonalizado          *string  `json:"pro_id_personalizado"`
	ProRazonSocial              *string  `json:"pro_razon_social"`
	ProNombreComercial          *string  `json:"pro_nombre_comercial"`
	ProPrimerApellido           *string  `json:"pro_primer_apellido"`
	ProSegundoApellido          *string  `json:"pro_segundo_apellido"`
	ProPrimerNombre             *string  `json:"pro_primer_nombre"`
	ProOtrosNombres             *string  `json:"pro_otros_nombres"`
	TdoCodigo                   string   `json:"tdo_codigo"`
	TojCodigo                   string   `json:"toj_codigo"`
	PaiCodigo                   *string  `json:"pai_codigo"`
	DepCodigo                   *string  `json:"dep_codigo"`
	MunCodigo                   *string  `json:"mun_codigo"`
	CpoCodigo                   *string  `json:"cpo_codigo"`
	ProDireccion                *string  `json:"pro_direccion"`
	ProTelefono                 *string  `json:"pro_telefono"`
	PaiCodigoDomicilioFiscal    *string  `json:"pai_codigo_domicilio_fiscal"`
	DepCodigoDomicilioFiscal    *string  `json:"dep_codigo_domicilio_fiscal"`
	MunCodigoDomicilioFiscal    *string  `json:"mun_codigo_domicilio_fiscal"`
	CpoCodigoDomicilioFiscal    *string  `json:"cpo_codigo_domicilio_fiscal"`
	ProDireccionDomicilioFiscal *string  `json:"pro_direccion_domicilio_fiscal"`
	ProCorreo                   string   `json:"pro_correo"`
	ProCorreosNotificacion      *string  `json:"pro_correos_notificacion"`
	ProMatriculaMercantil       *string  `json:"pro_matricula_mercantil"`
	ProUsuariosRecepcion        []string `json:"pro_usuarios_recepcion"`
	RfiCodigo                   *string  `json:"rfi_codigo"`
	RefCodigo                   []string `json:"ref_codigo"`
	Estado                      *string  `json:"estado"`
}

// ListProvidersResponse represents the response from listing providers.
type ListProvidersResponse struct {
	Total     int                 `json:"total"`
	Filtrados int                 `json:"filtrados"`
	Data      []provider.Provider `json:"data"`
}

// CreateProvider creates a new provider.
func (s *Service) CreateProvider(ctx context.Context, req CreateProviderRequest) (*CreateProviderResponse, error) {
	// Validate required fields
	if err := s.validateCreateRequest(req); err != nil {
		return nil, err
	}

	// Check if provider already exists
	exists, err := s.repo.Exists(ctx, req.OfeIdentificacion, req.ProIdentificacion)
	if err != nil {
		return nil, fmt.Errorf("check provider existence: %w", err)
	}

	if exists {
		return nil, fmt.Errorf("ya existe un Proveedor con el numero de identificacion [%s] para el OFE [%s]", req.ProIdentificacion, req.OfeIdentificacion)
	}

	// Convert request to domain entity
	prov := s.requestToProvider(req)

	// Create provider
	id, err := s.repo.Create(ctx, prov)
	if err != nil {
		return nil, fmt.Errorf("create provider: %w", err)
	}

	return &CreateProviderResponse{
		Success: true,
		ProID:   id,
	}, nil
}

// UpdateProvider updates an existing provider.
func (s *Service) UpdateProvider(ctx context.Context, ofeIdentificacion, proIdentificacion string, req UpdateProviderRequest) error {
	// Validate required fields
	if err := s.validateUpdateRequest(req); err != nil {
		return err
	}

	// Check if provider exists
	exists, err := s.repo.Exists(ctx, ofeIdentificacion, proIdentificacion)
	if err != nil {
		return fmt.Errorf("check provider existence: %w", err)
	}

	if !exists {
		return fmt.Errorf("el Id del proveedor no existe")
	}

	// Convert request to domain entity
	prov := s.updateRequestToProvider(req)

	// Update provider
	if err := s.repo.Update(ctx, ofeIdentificacion, proIdentificacion, prov); err != nil {
		return fmt.Errorf("update provider: %w", err)
	}

	return nil
}

// ListProviders lists providers with pagination, search, and sorting.
func (s *Service) ListProviders(ctx context.Context, start, length int, buscar, columnaOrden, ordenDireccion string) (*ListProvidersResponse, error) {
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

	providers, filtered, err := s.repo.List(ctx, start, length, buscar, columnaOrden, ordenDireccion)
	if err != nil {
		return nil, fmt.Errorf("list providers: %w", err)
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

	return &ListProvidersResponse{
		Total:     total,
		Filtrados: filtered,
		Data:      providers,
	}, nil
}

// SearchProvider searches for providers by field, value, OFE, and filter type.
func (s *Service) SearchProvider(ctx context.Context, campoBuscar, valorBuscar, valorOfe, filtroColumnas string) ([]provider.Provider, error) {
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
	providers, err := s.repo.Search(ctx, campoBuscar, valorBuscar, valorOfe, filtroColumnas)
	if err != nil {
		return nil, fmt.Errorf("search providers: %w", err)
	}

	return providers, nil
}

// validateCreateRequest validates the create request.
func (s *Service) validateCreateRequest(req CreateProviderRequest) error {
	// Required field validations
	if req.OfeIdentificacion == "" {
		return fmt.Errorf("ofe_identificacion es requerido")
	}
	if req.ProIdentificacion == "" {
		return fmt.Errorf("pro_identificacion es requerido")
	}
	if req.TdoCodigo == "" {
		return fmt.Errorf("tdo_codigo es requerido")
	}
	if req.TojCodigo == "" {
		return fmt.Errorf("toj_codigo es requerido")
	}
	if req.ProCorreo == "" {
		return fmt.Errorf("pro_correo es requerido")
	}

	// Length validations for required fields
	if err := validateMaxLength(req.OfeIdentificacion, 20, "ofe_identificacion"); err != nil {
		return err
	}
	if err := validateMaxLength(req.ProIdentificacion, 20, "pro_identificacion"); err != nil {
		return err
	}
	if err := validateMaxLength(req.TdoCodigo, 10, "tdo_codigo"); err != nil {
		return err
	}
	if err := validateMaxLength(req.TojCodigo, 10, "toj_codigo"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.ProTelefono, 50, "pro_telefono"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.ProDireccionDomicilioFiscal, 255, "pro_direccion_domicilio_fiscal"); err != nil {
		return err
	}
	if err := validateMaxLength(req.ProCorreo, 255, "pro_correo"); err != nil {
		return err
	}

	// Length validations for optional fields
	if err := validateMaxLengthPtr(req.ProIDPersonalizado, 100, "pro_id_personalizado"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.ProRazonSocial, 255, "pro_razon_social"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.ProNombreComercial, 255, "pro_nombre_comercial"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.ProPrimerApellido, 100, "pro_primer_apellido"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.ProSegundoApellido, 100, "pro_segundo_apellido"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.ProPrimerNombre, 100, "pro_primer_nombre"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.ProOtrosNombres, 100, "pro_otros_nombres"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.PaiCodigo, 10, "pai_codigo"); err != nil {
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
	if err := validateMaxLengthPtr(req.ProMatriculaMercantil, 100, "pro_matricula_mercantil"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.RfiCodigo, 10, "rfi_codigo"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.Estado, 20, "estado"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.ProDireccion, 255, "pro_direccion"); err != nil {
		return err
	}

	// Email format validations
	if err := validateEmail(req.ProCorreo, "pro_correo"); err != nil {
		return err
	}
	if err := validateEmailListPtr(req.ProCorreosNotificacion, "pro_correos_notificacion"); err != nil {
		return err
	}

	// Estado validation
	if err := validateEstado(req.Estado, "estado"); err != nil {
		return err
	}

	// Validate based on tipo de organización jurídica (toj_codigo)
	// "1" = Persona Jurídica, requires razón social or nombre comercial
	// "2" = Persona Natural, requires primer nombre and primer apellido
	if req.TojCodigo == "1" {
		// Persona Jurídica
		if (req.ProRazonSocial == nil || strings.TrimSpace(*req.ProRazonSocial) == "") &&
			(req.ProNombreComercial == nil || strings.TrimSpace(*req.ProNombreComercial) == "") {
			return fmt.Errorf("para persona jurídica, pro_razon_social o pro_nombre_comercial es requerido")
		}
	} else if req.TojCodigo == "2" {
		// Persona Natural
		if req.ProPrimerNombre == nil || strings.TrimSpace(*req.ProPrimerNombre) == "" {
			return fmt.Errorf("para persona natural, pro_primer_nombre es requerido")
		}
		if req.ProPrimerApellido == nil || strings.TrimSpace(*req.ProPrimerApellido) == "" {
			return fmt.Errorf("para persona natural, pro_primer_apellido es requerido")
		}
	}

	return nil
}

// validateUpdateRequest validates the update request.
func (s *Service) validateUpdateRequest(req UpdateProviderRequest) error {
	// Required field validations
	if req.OfeIdentificacion == "" {
		return fmt.Errorf("ofe_identificacion es requerido")
	}
	if req.ProIdentificacion == "" {
		return fmt.Errorf("pro_identificacion es requerido")
	}
	if req.TdoCodigo == "" {
		return fmt.Errorf("tdo_codigo es requerido")
	}
	if req.TojCodigo == "" {
		return fmt.Errorf("toj_codigo es requerido")
	}
	if req.ProCorreo == "" {
		return fmt.Errorf("pro_correo es requerido")
	}

	// Length validations for required fields
	if err := validateMaxLength(req.OfeIdentificacion, 20, "ofe_identificacion"); err != nil {
		return err
	}
	if err := validateMaxLength(req.ProIdentificacion, 20, "pro_identificacion"); err != nil {
		return err
	}
	if err := validateMaxLength(req.TdoCodigo, 10, "tdo_codigo"); err != nil {
		return err
	}
	if err := validateMaxLength(req.TojCodigo, 10, "toj_codigo"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.ProTelefono, 50, "pro_telefono"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.ProDireccionDomicilioFiscal, 255, "pro_direccion_domicilio_fiscal"); err != nil {
		return err
	}
	if err := validateMaxLength(req.ProCorreo, 255, "pro_correo"); err != nil {
		return err
	}

	// Length validations for optional fields
	if err := validateMaxLengthPtr(req.ProIDPersonalizado, 100, "pro_id_personalizado"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.ProRazonSocial, 255, "pro_razon_social"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.ProNombreComercial, 255, "pro_nombre_comercial"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.ProPrimerApellido, 100, "pro_primer_apellido"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.ProSegundoApellido, 100, "pro_segundo_apellido"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.ProPrimerNombre, 100, "pro_primer_nombre"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.ProOtrosNombres, 100, "pro_otros_nombres"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.PaiCodigo, 10, "pai_codigo"); err != nil {
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
	if err := validateMaxLengthPtr(req.ProMatriculaMercantil, 100, "pro_matricula_mercantil"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.RfiCodigo, 10, "rfi_codigo"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.Estado, 20, "estado"); err != nil {
		return err
	}
	if err := validateMaxLengthPtr(req.ProDireccion, 255, "pro_direccion"); err != nil {
		return err
	}

	// Email format validations
	if err := validateEmail(req.ProCorreo, "pro_correo"); err != nil {
		return err
	}
	if err := validateEmailListPtr(req.ProCorreosNotificacion, "pro_correos_notificacion"); err != nil {
		return err
	}

	// Estado validation
	if err := validateEstado(req.Estado, "estado"); err != nil {
		return err
	}

	// Validate based on tipo de organización jurídica
	if req.TojCodigo == "1" {
		if (req.ProRazonSocial == nil || strings.TrimSpace(*req.ProRazonSocial) == "") &&
			(req.ProNombreComercial == nil || strings.TrimSpace(*req.ProNombreComercial) == "") {
			return fmt.Errorf("para persona jurídica, pro_razon_social o pro_nombre_comercial es requerido")
		}
	} else if req.TojCodigo == "2" {
		if req.ProPrimerNombre == nil || strings.TrimSpace(*req.ProPrimerNombre) == "" {
			return fmt.Errorf("para persona natural, pro_primer_nombre es requerido")
		}
		if req.ProPrimerApellido == nil || strings.TrimSpace(*req.ProPrimerApellido) == "" {
			return fmt.Errorf("para persona natural, pro_primer_apellido es requerido")
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

// validateEstado validates that estado is either "ACTIVO" or "INACTIVO".
func validateEstado(estado *string, fieldName string) error {
	if estado != nil && *estado != "" {
		estadoUpper := strings.ToUpper(*estado)
		if estadoUpper != "ACTIVO" && estadoUpper != "INACTIVO" {
			return fmt.Errorf("%s debe ser 'ACTIVO' o 'INACTIVO'", fieldName)
		}
	}
	return nil
}

// requestToProvider converts a create request to a domain entity.
func (s *Service) requestToProvider(req CreateProviderRequest) provider.Provider {
	estado := "ACTIVO"
	if req.Estado != nil && *req.Estado != "" {
		estado = *req.Estado
	}

	prov := provider.Provider{
		OfeIdentificacion:           req.OfeIdentificacion,
		ProIdentificacion:           req.ProIdentificacion,
		ProIDPersonalizado:          req.ProIDPersonalizado,
		ProRazonSocial:              req.ProRazonSocial,
		ProNombreComercial:          req.ProNombreComercial,
		ProPrimerApellido:           req.ProPrimerApellido,
		ProSegundoApellido:          req.ProSegundoApellido,
		ProPrimerNombre:             req.ProPrimerNombre,
		ProOtrosNombres:             req.ProOtrosNombres,
		TdoCodigo:                   req.TdoCodigo,
		TojCodigo:                   req.TojCodigo,
		PaiCodigo:                   req.PaiCodigo,
		DepCodigo:                   req.DepCodigo,
		MunCodigo:                   req.MunCodigo,
		CpoCodigo:                   req.CpoCodigo,
		ProDireccion:                req.ProDireccion,
		ProTelefono:                 req.ProTelefono,
		PaiCodigoDomicilioFiscal:    req.PaiCodigoDomicilioFiscal,
		DepCodigoDomicilioFiscal:    req.DepCodigoDomicilioFiscal,
		MunCodigoDomicilioFiscal:    req.MunCodigoDomicilioFiscal,
		CpoCodigoDomicilioFiscal:    req.CpoCodigoDomicilioFiscal,
		ProDireccionDomicilioFiscal: req.ProDireccionDomicilioFiscal,
		ProCorreo:                   req.ProCorreo,
		ProCorreosNotificacion:      req.ProCorreosNotificacion,
		ProMatriculaMercantil:       req.ProMatriculaMercantil,
		ProUsuariosRecepcion:        req.ProUsuariosRecepcion,
		RfiCodigo:                   req.RfiCodigo,
		RefCodigo:                   req.RefCodigo,
		Estado:                      estado,
	}

	// Normalize empty strings to nil for optional fields
	if req.ProIDPersonalizado != nil && strings.TrimSpace(*req.ProIDPersonalizado) == "" {
		prov.ProIDPersonalizado = nil
	}
	if req.ProRazonSocial != nil && strings.TrimSpace(*req.ProRazonSocial) == "" {
		prov.ProRazonSocial = nil
	}
	if req.ProNombreComercial != nil && strings.TrimSpace(*req.ProNombreComercial) == "" {
		prov.ProNombreComercial = nil
	}
	if req.ProTelefono != nil && strings.TrimSpace(*req.ProTelefono) == "" {
		prov.ProTelefono = nil
	}
	if req.ProDireccionDomicilioFiscal != nil && strings.TrimSpace(*req.ProDireccionDomicilioFiscal) == "" {
		prov.ProDireccionDomicilioFiscal = nil
	}

	return prov
}

// updateRequestToProvider converts an update request to a domain entity.
func (s *Service) updateRequestToProvider(req UpdateProviderRequest) provider.Provider {
	estado := "ACTIVO"
	if req.Estado != nil && *req.Estado != "" {
		estado = *req.Estado
	}

	prov := provider.Provider{
		OfeIdentificacion:           req.OfeIdentificacion,
		ProIdentificacion:           req.ProIdentificacion,
		ProIDPersonalizado:          req.ProIDPersonalizado,
		ProRazonSocial:              req.ProRazonSocial,
		ProNombreComercial:          req.ProNombreComercial,
		ProPrimerApellido:           req.ProPrimerApellido,
		ProSegundoApellido:          req.ProSegundoApellido,
		ProPrimerNombre:             req.ProPrimerNombre,
		ProOtrosNombres:             req.ProOtrosNombres,
		TdoCodigo:                   req.TdoCodigo,
		TojCodigo:                   req.TojCodigo,
		PaiCodigo:                   req.PaiCodigo,
		DepCodigo:                   req.DepCodigo,
		MunCodigo:                   req.MunCodigo,
		CpoCodigo:                   req.CpoCodigo,
		ProDireccion:                req.ProDireccion,
		ProTelefono:                 req.ProTelefono,
		PaiCodigoDomicilioFiscal:    req.PaiCodigoDomicilioFiscal,
		DepCodigoDomicilioFiscal:    req.DepCodigoDomicilioFiscal,
		MunCodigoDomicilioFiscal:    req.MunCodigoDomicilioFiscal,
		CpoCodigoDomicilioFiscal:    req.CpoCodigoDomicilioFiscal,
		ProDireccionDomicilioFiscal: req.ProDireccionDomicilioFiscal,
		ProCorreo:                   req.ProCorreo,
		ProCorreosNotificacion:      req.ProCorreosNotificacion,
		ProMatriculaMercantil:       req.ProMatriculaMercantil,
		ProUsuariosRecepcion:        req.ProUsuariosRecepcion,
		RfiCodigo:                   req.RfiCodigo,
		RefCodigo:                   req.RefCodigo,
		Estado:                      estado,
	}

	// Normalize empty strings to nil for optional fields
	if req.ProIDPersonalizado != nil && strings.TrimSpace(*req.ProIDPersonalizado) == "" {
		prov.ProIDPersonalizado = nil
	}
	if req.ProRazonSocial != nil && strings.TrimSpace(*req.ProRazonSocial) == "" {
		prov.ProRazonSocial = nil
	}
	if req.ProNombreComercial != nil && strings.TrimSpace(*req.ProNombreComercial) == "" {
		prov.ProNombreComercial = nil
	}
	if req.ProTelefono != nil && strings.TrimSpace(*req.ProTelefono) == "" {
		prov.ProTelefono = nil
	}
	if req.ProDireccionDomicilioFiscal != nil && strings.TrimSpace(*req.ProDireccionDomicilioFiscal) == "" {
		prov.ProDireccionDomicilioFiscal = nil
	}

	return prov
}
