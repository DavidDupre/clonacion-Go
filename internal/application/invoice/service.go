package invoice

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"3tcapital/ms_facturacion_core/internal/core/acquirer"
	"3tcapital/ms_facturacion_core/internal/core/invoice"
	"3tcapital/ms_facturacion_core/internal/core/provider"
	coreresolution "3tcapital/ms_facturacion_core/internal/core/resolution"
)

// Service orchestrates invoice-related use cases.
type Service struct {
	provider           invoice.Provider
	acquirerRepo       acquirer.Repository // Optional: nil if database not configured
	providerRepo       provider.Repository // Optional: nil if database not configured (used for DS documents)
	workerPoolSize     int                 // Number of workers for concurrent processing
	cdoAmbienteDefault string              // Default environment value for documents ("1"=production, "2"=test)
}

// NewService creates a new invoice service with the given invoice provider.
// acquirerRepo is optional - if nil, acquirer validation will be skipped.
// providerRepo is optional - if nil, provider validation will be skipped (used for DS documents).
// workerPoolSize is optional - if 0, defaults to 10
func NewService(provider invoice.Provider, acquirerRepo acquirer.Repository, providerRepo provider.Repository, cdoAmbienteDefault string) *Service {
	return NewServiceWithWorkerPool(provider, acquirerRepo, providerRepo, 10, cdoAmbienteDefault)
}

// NewServiceWithWorkerPool creates a new invoice service with worker pool configuration.
func NewServiceWithWorkerPool(provider invoice.Provider, acquirerRepo acquirer.Repository, providerRepo provider.Repository, workerPoolSize int, cdoAmbienteDefault string) *Service {
	if workerPoolSize <= 0 {
		workerPoolSize = 10
	}
	return &Service{
		provider:           provider,
		acquirerRepo:       acquirerRepo,
		providerRepo:       providerRepo,
		workerPoolSize:     workerPoolSize,
		cdoAmbienteDefault: cdoAmbienteDefault,
	}
}

// normalizeNIT extracts the base NIT without verification digit (DV).
// Examples:
//   - "860011153-6" -> "860011153"
//   - "860011153" -> "860011153"
func normalizeNIT(nit string) string {
	if nit == "" {
		return ""
	}
	// Split by "-" to separate NIT from DV
	parts := strings.Split(nit, "-")
	if len(parts) > 1 {
		return strings.TrimSpace(parts[0])
	}
	return strings.TrimSpace(nit)
}

// GetDocuments retrieves documents/invoices for a given query.
func (s *Service) GetDocuments(ctx context.Context, query invoice.DocumentQuery) ([]invoice.Document, error) {
	// Validate CompanyNit
	if query.CompanyNit == "" {
		return nil, fmt.Errorf("company nit is required")
	}

	// Basic NIT validation: should be numeric and have reasonable length
	if len(query.CompanyNit) < 9 || len(query.CompanyNit) > 15 {
		return nil, fmt.Errorf("invalid company nit format: must be between 9 and 15 characters")
	}

	// Validate dates
	if query.InitialDate == "" {
		return nil, fmt.Errorf("initial date is required")
	}

	if query.FinalDate == "" {
		return nil, fmt.Errorf("final date is required")
	}

	// Validate date format
	dateLayout := "2006-01-02"
	initialDate, err := time.Parse(dateLayout, query.InitialDate)
	if err != nil {
		return nil, fmt.Errorf("invalid initial date format: must be YYYY-MM-DD")
	}

	finalDate, err := time.Parse(dateLayout, query.FinalDate)
	if err != nil {
		return nil, fmt.Errorf("invalid final date format: must be YYYY-MM-DD")
	}

	// Validate date range
	if initialDate.After(finalDate) {
		return nil, fmt.Errorf("initial date must be before or equal to final date")
	}

	documents, err := s.provider.GetDocuments(ctx, query)
	if err != nil {
		return nil, err
	}

	return documents, nil
}

// GetDocumentByNumber retrieves a document/invoice by document number.
func (s *Service) GetDocumentByNumber(ctx context.Context, query invoice.DocumentByNumberQuery) ([]invoice.Document, error) {
	// Validate CompanyNit
	if query.CompanyNit == "" {
		return nil, fmt.Errorf("company nit is required")
	}

	// Basic NIT validation: should be numeric and have reasonable length
	if len(query.CompanyNit) < 9 || len(query.CompanyNit) > 15 {
		return nil, fmt.Errorf("invalid company nit format: must be between 9 and 15 characters")
	}

	// Validate DocumentNumber
	if query.DocumentNumber == "" {
		return nil, fmt.Errorf("document number is required")
	}

	// Validate SupplierNit
	if query.SupplierNit == "" {
		return nil, fmt.Errorf("supplier nit is required")
	}

	// Basic NIT validation for SupplierNit
	if len(query.SupplierNit) < 9 || len(query.SupplierNit) > 15 {
		return nil, fmt.Errorf("invalid supplier nit format: must be between 9 and 15 characters")
	}

	documents, err := s.provider.GetDocumentByNumber(ctx, query)
	if err != nil {
		return nil, err
	}

	return documents, nil
}

// GetReceivedDocuments retrieves received documents/invoices for a given query.
func (s *Service) GetReceivedDocuments(ctx context.Context, query invoice.DocumentQuery) ([]invoice.Document, error) {
	// Validate CompanyNit
	if query.CompanyNit == "" {
		return nil, fmt.Errorf("company nit is required")
	}

	// Basic NIT validation: should be numeric and have reasonable length
	if len(query.CompanyNit) < 9 || len(query.CompanyNit) > 15 {
		return nil, fmt.Errorf("invalid company nit format: must be between 9 and 15 characters")
	}

	// Validate dates
	if query.InitialDate == "" {
		return nil, fmt.Errorf("initial date is required")
	}

	if query.FinalDate == "" {
		return nil, fmt.Errorf("final date is required")
	}

	// Validate date format
	dateLayout := "2006-01-02"
	initialDate, err := time.Parse(dateLayout, query.InitialDate)
	if err != nil {
		return nil, fmt.Errorf("invalid initial date format: must be YYYY-MM-DD")
	}

	finalDate, err := time.Parse(dateLayout, query.FinalDate)
	if err != nil {
		return nil, fmt.Errorf("invalid final date format: must be YYYY-MM-DD")
	}

	// Validate date range
	if initialDate.After(finalDate) {
		return nil, fmt.Errorf("initial date must be before or equal to final date")
	}

	documents, err := s.provider.GetReceivedDocuments(ctx, query)
	if err != nil {
		return nil, err
	}

	return documents, nil
}

// RegisterDocument registers documents with the invoice provider.
func (s *Service) RegisterDocument(ctx context.Context, req invoice.DocumentRegistrationRequest) (*invoice.DocumentRegistrationResponse, error) {
	// Validate that exactly one document type is provided
	typeCount := 0
	if len(req.Documentos.FC) > 0 {
		typeCount++
	}
	if len(req.Documentos.NC) > 0 {
		typeCount++
	}
	if len(req.Documentos.ND) > 0 {
		typeCount++
	}
	if len(req.Documentos.DS) > 0 {
		typeCount++
	}

	if typeCount == 0 {
		return nil, fmt.Errorf("no documents provided")
	}

	if typeCount > 1 {
		return nil, fmt.Errorf("only one document type (FC, NC, ND, or DS) can be provided per request")
	}

	// Get the documents to validate
	var documents []invoice.OpenETLDocument
	var documentType string

	if len(req.Documentos.FC) > 0 {
		documents = req.Documentos.FC
		documentType = "FC"
	} else if len(req.Documentos.NC) > 0 {
		documents = req.Documentos.NC
		documentType = "NC"
	} else if len(req.Documentos.ND) > 0 {
		documents = req.Documentos.ND
		documentType = "ND"
	} else {
		documents = req.Documentos.DS
		documentType = "DS"
	}

	// Validate each document
	for idx, doc := range documents {
		if err := s.validateDocument(doc, documentType, idx); err != nil {
			return nil, err
		}
	}

	// Use worker pool for concurrent processing if we have multiple documents and (acquirer or provider) repository
	// For small batches or no repository, use sequential processing for simplicity
	var validDocuments []invoice.OpenETLDocument
	var failedDocuments []invoice.FailedDocument

	hasRepo := (documentType == "DS" && s.providerRepo != nil) || (documentType != "DS" && s.acquirerRepo != nil)
	if hasRepo && len(documents) > 1 {
		// Use worker pool for concurrent processing
		pool := NewDocumentWorkerPool(ctx, s.workerPoolSize, s.acquirerRepo, s.providerRepo, s.provider, s.cdoAmbienteDefault)
		validDocuments, failedDocuments = pool.ProcessDocuments(ctx, documents, documentType)
	} else {
		// Sequential processing for small batches or when no acquirer repo
		now := time.Now()
		fechaProcesamiento := now.Format("2006-01-02")
		horaProcesamiento := now.Format("15:04:05")

		for _, doc := range documents {
			// 1. Enrich cdo_ambiente if missing
			doc = s.enrichDocumentWithEnvironment(doc)

			// 2. Validate acquirer/provider existence based on document type
			if documentType == "DS" {
				// DS documents: validate and enrich with provider data
				// IMPORTANT: For DS documents, the mapping is inverted:
				// - adq_identificacion (from request) maps to ofe_identificacion (in provider table)
				// - ofe_identificacion (from request) maps to pro_identificacion (in provider table)
				if s.providerRepo != nil {
					normalizedAdqNIT := normalizeNIT(doc.AdqIdentificacion)
					normalizedOfeNIT := normalizeNIT(doc.OfeIdentificacion)
					prov, err := s.providerRepo.FindByID(ctx, normalizedAdqNIT, normalizedOfeNIT)
					if err != nil {
						// Database error - add to failed documents
						failedDocuments = append(failedDocuments, invoice.FailedDocument{
							Documento:          documentType,
							Consecutivo:        doc.CdoConsecutivo,
							Prefijo:            doc.RfaPrefijo,
							Errors:             []string{fmt.Sprintf("Error al buscar proveedor: %v", err)},
							FechaProcesamiento: fechaProcesamiento,
							HoraProcesamiento:  horaProcesamiento,
						})
						continue
					}

					if prov == nil {
						// Provider not found - add to failed documents
						// Note: For DS, we search provider where ofe_identificacion=adq_identificacion and pro_identificacion=ofe_identificacion
						normalizedAdqNIT := normalizeNIT(doc.AdqIdentificacion)
						normalizedOfeNIT := normalizeNIT(doc.OfeIdentificacion)
						failedDocuments = append(failedDocuments, invoice.FailedDocument{
							Documento:          documentType,
							Consecutivo:        doc.CdoConsecutivo,
							Prefijo:            doc.RfaPrefijo,
							Errors:             []string{fmt.Sprintf("Proveedor no encontrado: ofe_identificacion=%s (BD, normalizado desde %s) y pro_identificacion=%s (BD, normalizado desde %s) - mapeado desde adq_identificacion=%s y ofe_identificacion=%s del request", normalizedAdqNIT, doc.AdqIdentificacion, normalizedOfeNIT, doc.OfeIdentificacion, doc.AdqIdentificacion, doc.OfeIdentificacion)},
							FechaProcesamiento: fechaProcesamiento,
							HoraProcesamiento:  horaProcesamiento,
						})
						continue
					}

					// Enrich document with provider data
					enrichedDoc := s.enrichDocumentWithProvider(doc, prov)
					// Enrich document with hardcoded OFE data
					enrichedDoc = s.enrichDocumentWithOFE(enrichedDoc)
					validDocuments = append(validDocuments, enrichedDoc)
				} else {
					// No provider repository - enrich with OFE data only
					enrichedDoc := s.enrichDocumentWithOFE(doc)
					validDocuments = append(validDocuments, enrichedDoc)
				}
			} else {
				// FC/NC/ND documents: validate and enrich with acquirer data
				if s.acquirerRepo != nil {
					normalizedOfeNIT := normalizeNIT(doc.OfeIdentificacion)
					normalizedAdqNIT := normalizeNIT(doc.AdqIdentificacion)
					acq, err := s.acquirerRepo.FindByID(ctx, normalizedOfeNIT, normalizedAdqNIT, "")
					if err != nil {
						// Database error - add to failed documents
						failedDocuments = append(failedDocuments, invoice.FailedDocument{
							Documento:          documentType,
							Consecutivo:        doc.CdoConsecutivo,
							Prefijo:            doc.RfaPrefijo,
							Errors:             []string{fmt.Sprintf("Error al buscar adquiriente: %v", err)},
							FechaProcesamiento: fechaProcesamiento,
							HoraProcesamiento:  horaProcesamiento,
						})
						continue
					}

					if acq == nil {
						// Acquirer not found - add to failed documents
						normalizedOfeNIT := normalizeNIT(doc.OfeIdentificacion)
						normalizedAdqNIT := normalizeNIT(doc.AdqIdentificacion)
						failedDocuments = append(failedDocuments, invoice.FailedDocument{
							Documento:          documentType,
							Consecutivo:        doc.CdoConsecutivo,
							Prefijo:            doc.RfaPrefijo,
							Errors:             []string{fmt.Sprintf("Adquiriente [%s] (normalizado desde %s) no encontrado para el OFE [%s] (normalizado desde %s)", normalizedAdqNIT, doc.AdqIdentificacion, normalizedOfeNIT, doc.OfeIdentificacion)},
							FechaProcesamiento: fechaProcesamiento,
							HoraProcesamiento:  horaProcesamiento,
						})
						continue
					}

					// Enrich document with acquirer data
					enrichedDoc := s.enrichDocumentWithAcquirer(doc, acq)
					// Enrich document with hardcoded OFE data
					enrichedDoc = s.enrichDocumentWithOFE(enrichedDoc)
					validDocuments = append(validDocuments, enrichedDoc)
				} else {
					// No acquirer repository - enrich with OFE data only
					enrichedDoc := s.enrichDocumentWithOFE(doc)
					validDocuments = append(validDocuments, enrichedDoc)
				}
			}
		}
	}

	// If all documents failed, return early with failed documents
	if len(validDocuments) == 0 {
		now := time.Now()
		fechaProcesamiento := now.Format("2006-01-02")
		horaProcesamiento := now.Format("15:04:05")
		// Ensure failedDocuments is never nil (use empty slice instead)
		if failedDocuments == nil {
			failedDocuments = make([]invoice.FailedDocument, 0)
		}
		return &invoice.DocumentRegistrationResponse{
			Message:              "Todos los documentos fallaron la validación",
			Lote:                 fmt.Sprintf("lote-%s-%s", fechaProcesamiento, horaProcesamiento),
			DocumentosProcesados: []invoice.ProcessedDocument{},
			DocumentosFallidos:   failedDocuments,
		}, nil
	}

	// Build request with only valid documents
	validReq := invoice.DocumentRegistrationRequest{
		Documentos: invoice.DocumentsByType{},
	}
	switch documentType {
	case "FC":
		validReq.Documentos.FC = validDocuments
	case "NC":
		validReq.Documentos.NC = validDocuments
	case "ND":
		validReq.Documentos.ND = validDocuments
	case "DS":
		validReq.Documentos.DS = validDocuments
	}

	// Call provider to register valid documents
	response, err := s.provider.RegisterDocument(ctx, validReq)
	if err != nil {
		return nil, err
	}

	// Merge failed documents from validation with provider response
	if len(failedDocuments) > 0 {
		// Ensure response.DocumentosFallidos is never nil before appending
		if response.DocumentosFallidos == nil {
			response.DocumentosFallidos = make([]invoice.FailedDocument, 0)
		}
		response.DocumentosFallidos = append(response.DocumentosFallidos, failedDocuments...)
	}

	// Ensure slices are never nil (use empty slice instead)
	if response.DocumentosProcesados == nil {
		response.DocumentosProcesados = make([]invoice.ProcessedDocument, 0)
	}
	if response.DocumentosFallidos == nil {
		response.DocumentosFallidos = make([]invoice.FailedDocument, 0)
	}

	return response, nil
}

// validateDocument validates a single document.
func (s *Service) validateDocument(doc invoice.OpenETLDocument, documentType string, index int) error {
	// Validate required fields
	if doc.TdeCodigo == "" {
		return fmt.Errorf("document %d: tde_codigo is required", index+1)
	}

	if doc.OfeIdentificacion == "" {
		return fmt.Errorf("document %d: ofe_identificacion is required", index+1)
	}

	if doc.AdqIdentificacion == "" {
		return fmt.Errorf("document %d: adq_identificacion is required", index+1)
	}

	// For NC and ND, rfa_resolucion can be empty (fields come empty and should remain empty)
	// Only require rfa_resolucion for FC and DS documents
	// Skip rfa_resolucion validation for NC and ND documents
	fmt.Printf("DEBUG validateDocument: documentType=%s, rfa_resolucion='%s', index=%d\n",
		documentType, doc.RfaResolucion, index)
	if doc.RfaResolucion == "" && documentType != "NC" && documentType != "ND" {
		return fmt.Errorf("document %d: rfa_resolucion is required", index+1)
	}

	if doc.CdoConsecutivo == "" {
		return fmt.Errorf("document %d: cdo_consecutivo is required", index+1)
	}

	if doc.CdoFecha == "" {
		return fmt.Errorf("document %d: cdo_fecha is required", index+1)
	}

	if doc.CdoHora == "" {
		return fmt.Errorf("document %d: cdo_hora is required", index+1)
	}

	if doc.MonCodigo == "" {
		return fmt.Errorf("document %d: mon_codigo is required", index+1)
	}

	if doc.CdoValorSinImpuestos == "" {
		return fmt.Errorf("document %d: cdo_valor_sin_impuestos is required", index+1)
	}

	if doc.CdoImpuestos == "" {
		return fmt.Errorf("document %d: cdo_impuestos is required", index+1)
	}

	if doc.CdoTotal == "" {
		return fmt.Errorf("document %d: cdo_total is required", index+1)
	}

	// Validate date format (YYYY-MM-DD)
	dateLayout := "2006-01-02"
	if _, err := time.Parse(dateLayout, doc.CdoFecha); err != nil {
		return fmt.Errorf("document %d: invalid cdo_fecha format: must be YYYY-MM-DD", index+1)
	}

	// Validate that cdo_fecha is today's date (DIAN FAD09e compliance)
	if err := validateIssueDateIsToday(doc.CdoFecha, index); err != nil {
		return err
	}

	// Validate time format (HH:mm:ss)
	timeLayout := "15:04:05"
	if _, err := time.Parse(timeLayout, doc.CdoHora); err != nil {
		return fmt.Errorf("document %d: invalid cdo_hora format: must be HH:mm:ss", index+1)
	}

	// Validate items array is not empty
	if len(doc.Items) == 0 {
		return fmt.Errorf("document %d: at least one item is required", index+1)
	}

	// Validate due date if provided
	if doc.CdoVencimiento != nil && *doc.CdoVencimiento != "" {
		// Validate date format (YYYY-MM-DD)
		dateLayout := "2006-01-02"
		dueDate, err := time.Parse(dateLayout, *doc.CdoVencimiento)
		if err != nil {
			return fmt.Errorf("document %d: invalid cdo_vencimiento format: must be YYYY-MM-DD", index+1)
		}

		// Parse document date for comparison
		docDate, err := time.Parse(dateLayout, doc.CdoFecha)
		if err != nil {
			// This should not happen as cdo_fecha is already validated above
			return fmt.Errorf("document %d: invalid cdo_fecha format for comparison with cdo_vencimiento", index+1)
		}

		// Validate that due date is on or after document date
		if dueDate.Before(docDate) {
			return fmt.Errorf("document %d: cdo_vencimiento (%s) must be on or after cdo_fecha (%s)",
				index+1, *doc.CdoVencimiento, doc.CdoFecha)
		}
	}

	// Validate document type code matches the array it's in
	expectedTypeCodes := map[string][]string{
		"FC": {"01"},
		"NC": {"03", "91"},
		"ND": {"04", "92"},
		"DS": {"05"},
	}

	validCodes := expectedTypeCodes[documentType]
	valid := false
	for _, code := range validCodes {
		if doc.TdeCodigo == code {
			valid = true
			break
		}
	}

	if !valid {
		return fmt.Errorf("document %d: tde_codigo %s does not match document type %s (expected: %v)", index+1, doc.TdeCodigo, documentType, validCodes)
	}

	// Validate top_codigo for DS documents
	if documentType == "DS" {
		if doc.TopCodigo == "" {
			return fmt.Errorf("document %d: top_codigo is required for DS documents", index+1)
		}
		if doc.TopCodigo != "10" {
			return fmt.Errorf("document %d: top_codigo must be \"10\" for DS documents, got: %s", index+1, doc.TopCodigo)
		}
	}

	return nil
}

// validateIssueDateIsToday validates that cdo_fecha is today's date (Colombia timezone).
// This is required by DIAN rule FAD09e: IssueDate must equal signature date.
// When documents are signed by Numrot, they use the current date, so cdo_fecha must match.
func validateIssueDateIsToday(cdoFecha string, index int) error {
	// Get current date in Colombia timezone (UTC-5)
	loc, err := time.LoadLocation("America/Bogota")
	if err != nil {
		// Fallback to UTC-5 offset if timezone data is not available
		loc = time.FixedZone("America/Bogota", -5*60*60)
	}
	today := time.Now().In(loc)
	todayDate := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, loc)

	// Parse the provided date in the same timezone
	providedDate, err := time.ParseInLocation("2006-01-02", cdoFecha, loc)
	if err != nil {
		return fmt.Errorf("document %d: invalid cdo_fecha format", index+1)
	}

	// Compare only the date part (year, month, day)
	if providedDate.Year() != todayDate.Year() || providedDate.Month() != todayDate.Month() || providedDate.Day() != todayDate.Day() {
		return fmt.Errorf("document %d: cdo_fecha must be today's date (%s) for DIAN FAD09e compliance. Provided: %s",
			index+1, todayDate.Format("2006-01-02"), cdoFecha)
	}
	return nil
}

// enrichDocumentWithAcquirer enriches a document with acquirer data from the database.
// It maps acquirer fields to document fields, preferring fiscal address fields when available.
func (s *Service) enrichDocumentWithAcquirer(doc invoice.OpenETLDocument, acq *acquirer.Acquirer) invoice.OpenETLDocument {
	// Map acquirer data to document fields
	// Use fiscal address fields if available, otherwise use regular address fields
	direccion := acq.AdqDireccionDomicilioFiscal
	if direccion == nil || *direccion == "" {
		direccion = acq.AdqDireccion
	}

	munCodigo := acq.MunCodigoDomicilioFiscal
	if munCodigo == nil || *munCodigo == "" {
		munCodigo = acq.MunCodigo
	}

	depCodigo := acq.DepCodigoDomicilioFiscal
	if depCodigo == nil || *depCodigo == "" {
		depCodigo = acq.DepCodigo
	}

	paiCodigo := acq.PaiCodigoDomicilioFiscal
	if paiCodigo == nil || *paiCodigo == "" {
		paiCodigoStr := acq.PaiCodigo
		paiCodigo = &paiCodigoStr
	}

	// Get municipality name (prefer fiscal, fallback to regular)
	munNombre := acq.MunNombreDomicilioFiscal
	if munNombre == nil || *munNombre == "" {
		munNombre = acq.MunNombre
	}

	// Get department name (prefer fiscal, fallback to regular)
	depNombre := acq.DepNombreDomicilioFiscal
	if depNombre == nil || *depNombre == "" {
		depNombre = acq.DepNombre
	}

	// Get postal code (prefer fiscal, fallback to regular)
	cpoCodigo := acq.CpoCodigoDomicilioFiscal
	if cpoCodigo == nil || *cpoCodigo == "" {
		cpoCodigo = acq.CpoCodigo
	}

	// Set acquirer fields in document (only if not already set in request)
	if doc.AdqRazonSocial == nil || *doc.AdqRazonSocial == "" {
		doc.AdqRazonSocial = &acq.AdqRazonSocial
	}
	if doc.AdqDireccion == nil || *doc.AdqDireccion == "" {
		doc.AdqDireccion = direccion
	}
	if doc.AdqMunicipioCodigo == nil || *doc.AdqMunicipioCodigo == "" {
		doc.AdqMunicipioCodigo = munCodigo
	}
	if doc.AdqMunicipioNombre == nil || *doc.AdqMunicipioNombre == "" {
		doc.AdqMunicipioNombre = munNombre
	}
	if doc.AdqDepartamentoCodigo == nil || *doc.AdqDepartamentoCodigo == "" {
		doc.AdqDepartamentoCodigo = depCodigo
	}
	if doc.AdqDepartamentoNombre == nil || *doc.AdqDepartamentoNombre == "" {
		doc.AdqDepartamentoNombre = depNombre
	}
	if doc.AdqPaisCodigo == nil || *doc.AdqPaisCodigo == "" {
		doc.AdqPaisCodigo = paiCodigo
	}
	if doc.AdqCpoCodigo == nil || *doc.AdqCpoCodigo == "" {
		doc.AdqCpoCodigo = cpoCodigo
	}

	// Set default country name if not provided
	if doc.AdqPaisNombre == nil || *doc.AdqPaisNombre == "" {
		if paiCodigo != nil && *paiCodigo == "CO" {
			paisNombre := "Colombia"
			doc.AdqPaisNombre = &paisNombre
		}
	}

	return doc
}

// enrichDocumentWithProvider enriches a document with provider data from the database.
// It maps provider fields to document fields (specifically for DS documents where adq_identificacion represents the provider).
// IMPORTANT: For DS documents, the provider lookup uses inverted mapping:
// - adq_identificacion (from request) → ofe_identificacion (in provider table)
// - ofe_identificacion (from request) → pro_identificacion (in provider table)
// It prefers fiscal address fields when available.
func (s *Service) enrichDocumentWithProvider(doc invoice.OpenETLDocument, prov *provider.Provider) invoice.OpenETLDocument {
	// Map provider data to document fields
	// Use fiscal address fields if available, otherwise use regular address fields
	var direccion string
	if prov.ProDireccionDomicilioFiscal != nil && *prov.ProDireccionDomicilioFiscal != "" {
		direccion = *prov.ProDireccionDomicilioFiscal
	} else if prov.ProDireccion != nil {
		direccion = *prov.ProDireccion
	}

	munCodigo := prov.MunCodigoDomicilioFiscal
	if munCodigo == nil || *munCodigo == "" {
		munCodigo = prov.MunCodigo
	}

	depCodigo := prov.DepCodigoDomicilioFiscal
	if depCodigo == nil || *depCodigo == "" {
		depCodigo = prov.DepCodigo
	}

	paiCodigo := prov.PaiCodigoDomicilioFiscal
	if paiCodigo == nil || *paiCodigo == "" {
		paiCodigo = prov.PaiCodigo
	}

	// Get postal code (prefer fiscal, fallback to regular)
	cpoCodigo := prov.CpoCodigoDomicilioFiscal
	if cpoCodigo == nil || *cpoCodigo == "" {
		cpoCodigo = prov.CpoCodigo
	}

	// Set provider fields in document (only if not already set in request)
	// Note: For DS documents, provider data maps to adq_* fields in the document
	if doc.AdqRazonSocial == nil || *doc.AdqRazonSocial == "" {
		if prov.ProRazonSocial != nil && *prov.ProRazonSocial != "" {
			doc.AdqRazonSocial = prov.ProRazonSocial
		} else if prov.ProNombreComercial != nil && *prov.ProNombreComercial != "" {
			// Fallback to commercial name if business name not available
			doc.AdqRazonSocial = prov.ProNombreComercial
		}
	}
	if doc.AdqDireccion == nil || *doc.AdqDireccion == "" {
		if direccion != "" {
			doc.AdqDireccion = &direccion
		}
	}
	if doc.AdqMunicipioCodigo == nil || *doc.AdqMunicipioCodigo == "" {
		doc.AdqMunicipioCodigo = munCodigo
	}
	// Note: Provider doesn't have municipality/department names, only codes
	// The client should provide names or they can be looked up from DANE if needed
	if doc.AdqDepartamentoCodigo == nil || *doc.AdqDepartamentoCodigo == "" {
		doc.AdqDepartamentoCodigo = depCodigo
	}
	if doc.AdqPaisCodigo == nil || *doc.AdqPaisCodigo == "" {
		doc.AdqPaisCodigo = paiCodigo
	}
	if doc.AdqCpoCodigo == nil || *doc.AdqCpoCodigo == "" {
		doc.AdqCpoCodigo = cpoCodigo
	}

	// Set default country name if not provided
	if doc.AdqPaisNombre == nil || *doc.AdqPaisNombre == "" {
		if paiCodigo != nil && *paiCodigo == "CO" {
			paisNombre := "Colombia"
			doc.AdqPaisNombre = &paisNombre
		}
	}

	return doc
}

// enrichDocumentWithOFE enriches a document with hardcoded OFE information.
// These values are hardcoded as per business requirements:
// - ofe_razon_social: "Positiva SAS"
// - ofe_direccion: "CLL 50 - 96"
// - ofe_municipio_codigo: "05380"
// - ofe_municipio_nombre: "LA ESTRELLA"
// - ofe_departamento_codigo: "05"
// - ofe_departamento_nombre: "ANTIOQUIA"
func (s *Service) enrichDocumentWithOFE(doc invoice.OpenETLDocument) invoice.OpenETLDocument {
	// Hardcoded OFE values
	ofeRazonSocial := "Positiva SAS"
	ofeDireccion := "CLL 50 - 96"
	ofeMunicipioCodigo := "05380"
	ofeMunicipioNombre := "LA ESTRELLA"
	ofeDepartamentoCodigo := "05"
	ofeDepartamentoNombre := "ANTIOQUIA"

	// Set OFE fields (override any values provided in request)
	doc.OfeRazonSocial = &ofeRazonSocial
	doc.OfeDireccion = &ofeDireccion
	doc.OfeMunicipioCodigo = &ofeMunicipioCodigo
	doc.OfeMunicipioNombre = &ofeMunicipioNombre
	doc.OfeDepartamentoCodigo = &ofeDepartamentoCodigo
	doc.OfeDepartamentoNombre = &ofeDepartamentoNombre

	return doc
}

// enrichDocumentWithEnvironment enriches a document with default environment if missing.
func (s *Service) enrichDocumentWithEnvironment(doc invoice.OpenETLDocument) invoice.OpenETLDocument {
	if doc.CdoAmbiente == nil || *doc.CdoAmbiente == "" {
		doc.CdoAmbiente = &s.cdoAmbienteDefault
	}
	return doc
}

// findResolution finds a resolution by OFE, number, and prefix.
func (s *Service) findResolution(ctx context.Context, ofeIdentificacion, resolutionNumber, prefix string) (*coreresolution.Resolution, error) {
	// Extract base NIT (without DV) for resolution query
	// The system receives NIT with DV (e.g., "860011153-6") but Numrot API expects only the base NIT (e.g., "860011153")
	baseNIT := ofeIdentificacion
	if parts := strings.Split(ofeIdentificacion, "-"); len(parts) > 1 {
		baseNIT = parts[0]
	}

	// Query resolutions from provider using base NIT
	resolutions, err := s.provider.GetResolutions(ctx, baseNIT)
	if err != nil {
		return nil, fmt.Errorf("error al consultar resoluciones para OFE [%s]: %w", baseNIT, err)
	}

	// Find matching resolution
	for i := range resolutions {
		if resolutions[i].ResolutionNumber == resolutionNumber && resolutions[i].Prefix == prefix {
			return &resolutions[i], nil
		}
	}

	return nil, fmt.Errorf("resolución [%s] con prefijo [%s] no encontrada para el OFE [%s]",
		resolutionNumber, prefix, ofeIdentificacion)
}

// enrichDocumentWithResolution enriches a document with resolution data if fields are missing.
// documentType is used to determine if resolution validation is required (DS doesn't require it).
func (s *Service) enrichDocumentWithResolution(ctx context.Context, doc invoice.OpenETLDocument, documentType string) (invoice.OpenETLDocument, error) {
	// Check if resolution fields are already present
	hasAllFields := doc.RfaFechaInicio != nil && *doc.RfaFechaInicio != "" &&
		doc.RfaFechaFin != nil && *doc.RfaFechaFin != "" &&
		doc.RfaNumeroInicio != nil && *doc.RfaNumeroInicio != "" &&
		doc.RfaNumeroFin != nil && *doc.RfaNumeroFin != ""

	if hasAllFields {
		return doc, nil // All fields present, no need to enrich
	}

	// For DS, NC, and ND documents, resolution is optional - if not found, return without error
	// For FC documents, resolution is required
	findResolution := true
	if (documentType == "DS" || documentType == "NC" || documentType == "ND") && (doc.RfaResolucion == "" || doc.RfaPrefijo == "") {
		// DS, NC, and ND don't require resolution if not provided
		findResolution = false
	}

	if !findResolution {
		// For DS, NC, or ND without resolution, return document as-is
		return doc, nil
	}

	// Find resolution
	resolution, err := s.findResolution(ctx, doc.OfeIdentificacion, doc.RfaResolucion, doc.RfaPrefijo)
	if err != nil {
		// For DS, if resolution is not found, it's not a fatal error
		if documentType == "DS" {
			return doc, nil // Return document without resolution data
		}
		return doc, err
	}

	// Populate missing fields
	if doc.RfaFechaInicio == nil || *doc.RfaFechaInicio == "" {
		fechaInicio := resolution.ValidDateFrom.Format("2006-01-02")
		doc.RfaFechaInicio = &fechaInicio
	}

	if doc.RfaFechaFin == nil || *doc.RfaFechaFin == "" {
		fechaFin := resolution.ValidDateTo.Format("2006-01-02")
		doc.RfaFechaFin = &fechaFin
	}

	if doc.RfaNumeroInicio == nil || *doc.RfaNumeroInicio == "" {
		numInicio := fmt.Sprintf("%d", resolution.FromNumber)
		doc.RfaNumeroInicio = &numInicio
	}

	if doc.RfaNumeroFin == nil || *doc.RfaNumeroFin == "" {
		numFin := fmt.Sprintf("%d", resolution.ToNumber)
		doc.RfaNumeroFin = &numFin
	}

	return doc, nil
}

// validateResolutionData validates that the document data matches the resolution.
func (s *Service) validateResolutionData(ctx context.Context, doc invoice.OpenETLDocument, index int) error {
	// Find resolution
	resolution, err := s.findResolution(ctx, doc.OfeIdentificacion, doc.RfaResolucion, doc.RfaPrefijo)
	if err != nil {
		return err
	}

	// Validate consecutivo is within range
	consecutivo, err := strconv.ParseInt(doc.CdoConsecutivo, 10, 64)
	if err != nil {
		return fmt.Errorf("document %d: consecutivo inválido [%s]: %v", index+1, doc.CdoConsecutivo, err)
	}

	if consecutivo < resolution.FromNumber || consecutivo > resolution.ToNumber {
		return fmt.Errorf("document %d: consecutivo [%s] fuera del rango autorizado [%d-%d] para la resolución [%s]",
			index+1, doc.CdoConsecutivo, resolution.FromNumber, resolution.ToNumber, doc.RfaResolucion)
	}

	// Validate date is within validity period
	docDate, err := time.Parse("2006-01-02", doc.CdoFecha)
	if err != nil {
		return fmt.Errorf("document %d: fecha de documento inválida [%s]: %v", index+1, doc.CdoFecha, err)
	}

	if docDate.Before(resolution.ValidDateFrom) || docDate.After(resolution.ValidDateTo) {
		return fmt.Errorf("document %d: fecha [%s] fuera de la vigencia [%s - %s] de la resolución [%s]",
			index+1, doc.CdoFecha,
			resolution.ValidDateFrom.Format("2006-01-02"),
			resolution.ValidDateTo.Format("2006-01-02"),
			doc.RfaResolucion)
	}

	return nil
}
