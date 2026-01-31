package invoice

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"3tcapital/ms_facturacion_core/internal/core/acquirer"
	"3tcapital/ms_facturacion_core/internal/core/invoice"
	"3tcapital/ms_facturacion_core/internal/core/provider"
	coreresolution "3tcapital/ms_facturacion_core/internal/core/resolution"
)

// DocumentJob represents a job to be processed by a worker
type DocumentJob struct {
	Document     invoice.OpenETLDocument
	DocumentType string
	Index        int
}

// DocumentResult represents the result of processing a document
type DocumentResult struct {
	Document     invoice.OpenETLDocument
	Failed       bool
	Error        error
	ErrorMessage string
	Index        int
}

// DocumentWorkerPool manages concurrent processing of documents
type DocumentWorkerPool struct {
	workerCount        int
	jobChan            chan DocumentJob
	resultChan         chan DocumentResult
	acquirerRepo       acquirer.Repository
	acquirerCache      *sync.Map // Cache for acquirer lookups: key = "ofe:adq", value = *acquirer.Acquirer
	providerRepo       provider.Repository
	providerCache      *sync.Map // Cache for provider lookups: key = "ofe:pro", value = *provider.Provider
	wg                 sync.WaitGroup
	ctx                context.Context
	cancel             context.CancelFunc
	provider           invoice.Provider // For querying resolutions
	cdoAmbienteDefault string           // Default environment value
	resolutionCache    *sync.Map        // Cache for resolutions: key = "ofe:resolution:prefix", value = *coreresolution.Resolution
}

// NewDocumentWorkerPool creates a new worker pool for document processing
func NewDocumentWorkerPool(ctx context.Context, workerCount int, acquirerRepo acquirer.Repository, providerRepo provider.Repository, provider invoice.Provider, cdoAmbienteDefault string) *DocumentWorkerPool {
	poolCtx, cancel := context.WithCancel(ctx)

	return &DocumentWorkerPool{
		workerCount:        workerCount,
		jobChan:            make(chan DocumentJob, workerCount*2), // Buffered channel
		resultChan:         make(chan DocumentResult, workerCount*2),
		acquirerRepo:       acquirerRepo,
		acquirerCache:      &sync.Map{},
		providerRepo:       providerRepo,
		providerCache:      &sync.Map{},
		provider:           provider,
		cdoAmbienteDefault: cdoAmbienteDefault,
		resolutionCache:    &sync.Map{},
		ctx:                poolCtx,
		cancel:             cancel,
	}
}

// Start starts the worker pool with the specified number of workers
func (p *DocumentWorkerPool) Start() {
	for i := 0; i < p.workerCount; i++ {
		p.wg.Add(1)
		go p.worker(i)
	}
}

// Stop stops the worker pool gracefully
func (p *DocumentWorkerPool) Stop() {
	close(p.jobChan)
	p.cancel()
	p.wg.Wait()
	close(p.resultChan)
}

// Submit submits a job to the worker pool
func (p *DocumentWorkerPool) Submit(job DocumentJob) error {
	select {
	case p.jobChan <- job:
		return nil
	case <-p.ctx.Done():
		return p.ctx.Err()
	}
}

// Results returns the channel for receiving results
func (p *DocumentWorkerPool) Results() <-chan DocumentResult {
	return p.resultChan
}

// worker processes jobs from the job channel
func (p *DocumentWorkerPool) worker(id int) {
	defer p.wg.Done()

	for job := range p.jobChan {
		result := p.processDocument(job)

		select {
		case p.resultChan <- result:
		case <-p.ctx.Done():
			return
		}
	}
}

// processDocument processes a single document job
func (p *DocumentWorkerPool) processDocument(job DocumentJob) DocumentResult {
	result := DocumentResult{
		Document: job.Document,
		Index:    job.Index,
		Failed:   false,
	}

	// 1. Enrich cdo_ambiente if missing
	result.Document = enrichDocumentWithEnvironment(job.Document, p.cdoAmbienteDefault)

	// 2. Validate and enrich acquirer/provider based on document type
	// For DS documents, use provider repository; for FC/NC/ND, use acquirer repository
	if job.DocumentType == "DS" {
		// DS documents: validate and enrich with provider data
		// IMPORTANT: For DS documents, the mapping is inverted:
		// - adq_identificacion (from request) maps to ofe_identificacion (in provider table)
		// - ofe_identificacion (from request) maps to pro_identificacion (in provider table)
		// If no provider repository, enrich with OFE data only
		if p.providerRepo == nil {
			result.Document = enrichDocumentWithOFE(result.Document)
			return result
		}

		// Check cache first
		// Cache key uses normalized NITs to ensure cache hits work correctly regardless of DV format
		// (matching the inverted mapping used in FindByID)
		normalizedAdqNIT := normalizeNIT(job.Document.AdqIdentificacion)
		normalizedOfeNIT := normalizeNIT(job.Document.OfeIdentificacion)
		cacheKey := normalizedAdqNIT + ":" + normalizedOfeNIT
		if cached, ok := p.providerCache.Load(cacheKey); ok {
			if prov, ok := cached.(*provider.Provider); ok && prov != nil {
				// Enrich document with cached provider data
				result.Document = enrichDocumentWithProvider(result.Document, prov)
				// Enrich document with hardcoded OFE data
				result.Document = enrichDocumentWithOFE(result.Document)
				return result
			}
		}

		// Lookup provider from repository
		// Note: Parameters are inverted for DS - adq_identificacion maps to ofe_identificacion in DB
		// Use normalized NITs for consistent lookups
		prov, err := p.providerRepo.FindByID(p.ctx, normalizedAdqNIT, normalizedOfeNIT)
		if err != nil {
			result.Failed = true
			result.Error = err
			result.ErrorMessage = "Error al buscar proveedor: " + err.Error()
			return result
		}

		if prov == nil {
			result.Failed = true
			// Note: For DS, we search provider where ofe_identificacion=adq_identificacion and pro_identificacion=ofe_identificacion
			result.ErrorMessage = fmt.Sprintf("Proveedor no encontrado: ofe_identificacion=%s (BD, normalizado desde %s) y pro_identificacion=%s (BD, normalizado desde %s) - mapeado desde adq_identificacion=%s y ofe_identificacion=%s del request", normalizedAdqNIT, job.Document.AdqIdentificacion, normalizedOfeNIT, job.Document.OfeIdentificacion, job.Document.AdqIdentificacion, job.Document.OfeIdentificacion)
			return result
		}

		// Cache the provider for future use
		p.providerCache.Store(cacheKey, prov)

		// Enrich document with provider data
		result.Document = enrichDocumentWithProvider(job.Document, prov)
		// Enrich document with hardcoded OFE data
		result.Document = enrichDocumentWithOFE(result.Document)
		return result
	}

	// FC/NC/ND documents: validate and enrich with acquirer data (existing logic)
	// If no acquirer repository, enrich with OFE data only
	if p.acquirerRepo == nil {
		result.Document = enrichDocumentWithOFE(result.Document)
		return result
	}

	// Check cache first
	// Cache key uses normalized NITs to ensure cache hits work correctly regardless of DV format
	normalizedOfeNIT := normalizeNIT(job.Document.OfeIdentificacion)
	normalizedAdqNIT := normalizeNIT(job.Document.AdqIdentificacion)
	cacheKey := normalizedOfeNIT + ":" + normalizedAdqNIT
	if cached, ok := p.acquirerCache.Load(cacheKey); ok {
		if acq, ok := cached.(*acquirer.Acquirer); ok && acq != nil {
			// Enrich document with cached acquirer data
			result.Document = enrichDocumentWithAcquirer(result.Document, acq)
			// Enrich document with hardcoded OFE data
			result.Document = enrichDocumentWithOFE(result.Document)
			return result
		}
	}

	// Lookup acquirer from repository
	// Use normalized NITs for consistent lookups
	acq, err := p.acquirerRepo.FindByID(p.ctx, normalizedOfeNIT, normalizedAdqNIT, "")
	if err != nil {
		result.Failed = true
		result.Error = err
		result.ErrorMessage = "Error al buscar adquiriente: " + err.Error()
		return result
	}

	if acq == nil {
		result.Failed = true
		result.ErrorMessage = fmt.Sprintf("Adquiriente [%s] (normalizado desde %s) no encontrado para el OFE [%s] (normalizado desde %s)", normalizedAdqNIT, job.Document.AdqIdentificacion, normalizedOfeNIT, job.Document.OfeIdentificacion)
		return result
	}

	// Cache the acquirer for future use
	p.acquirerCache.Store(cacheKey, acq)

	// Enrich document with acquirer data
	result.Document = enrichDocumentWithAcquirer(job.Document, acq)
	// Enrich document with hardcoded OFE data
	result.Document = enrichDocumentWithOFE(result.Document)
	return result
}

// enrichDocumentWithAcquirer enriches a document with acquirer data
// This is a copy of the method from service.go to avoid circular dependencies
func enrichDocumentWithAcquirer(doc invoice.OpenETLDocument, acq *acquirer.Acquirer) invoice.OpenETLDocument {
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

// enrichDocumentWithProvider enriches a document with provider data
// This is a copy of the method from service.go to avoid circular dependencies
// For DS documents, provider data maps to adq_* fields in the document
func enrichDocumentWithProvider(doc invoice.OpenETLDocument, prov *provider.Provider) invoice.OpenETLDocument {
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
func enrichDocumentWithOFE(doc invoice.OpenETLDocument) invoice.OpenETLDocument {
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
// This is a standalone helper function to avoid circular dependencies.
func enrichDocumentWithEnvironment(doc invoice.OpenETLDocument, defaultEnv string) invoice.OpenETLDocument {
	if doc.CdoAmbiente == nil || *doc.CdoAmbiente == "" {
		doc.CdoAmbiente = &defaultEnv
	}
	return doc
}

// enrichDocumentWithResolution enriches a document with resolution data if fields are missing.
// documentType is used to determine if resolution validation is required (DS doesn't require it).
func (p *DocumentWorkerPool) enrichDocumentWithResolution(doc invoice.OpenETLDocument, documentType string) (invoice.OpenETLDocument, error) {
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
	resolution, err := p.findResolution(doc.OfeIdentificacion, doc.RfaResolucion, doc.RfaPrefijo)
	if err != nil {
		// For DS, NC, and ND, if resolution is not found, it's not a fatal error
		if documentType == "DS" || documentType == "NC" || documentType == "ND" {
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
func (p *DocumentWorkerPool) validateResolutionData(doc invoice.OpenETLDocument) error {
	// Find resolution
	resolution, err := p.findResolution(doc.OfeIdentificacion, doc.RfaResolucion, doc.RfaPrefijo)
	if err != nil {
		return err
	}

	// Validate consecutivo is within range
	consecutivo, err := strconv.ParseInt(doc.CdoConsecutivo, 10, 64)
	if err != nil {
		return fmt.Errorf("consecutivo inválido [%s]: %v", doc.CdoConsecutivo, err)
	}

	if consecutivo < resolution.FromNumber || consecutivo > resolution.ToNumber {
		return fmt.Errorf("consecutivo [%s] fuera del rango autorizado [%d-%d] para la resolución [%s]",
			doc.CdoConsecutivo, resolution.FromNumber, resolution.ToNumber, doc.RfaResolucion)
	}

	// Validate date is within validity period
	docDate, err := time.Parse("2006-01-02", doc.CdoFecha)
	if err != nil {
		return fmt.Errorf("fecha de documento inválida [%s]: %v", doc.CdoFecha, err)
	}

	if docDate.Before(resolution.ValidDateFrom) || docDate.After(resolution.ValidDateTo) {
		return fmt.Errorf("fecha [%s] fuera de la vigencia [%s - %s] de la resolución [%s]",
			doc.CdoFecha,
			resolution.ValidDateFrom.Format("2006-01-02"),
			resolution.ValidDateTo.Format("2006-01-02"),
			doc.RfaResolucion)
	}

	return nil
}

// findResolution finds a resolution by OFE, number, and prefix (with caching).
func (p *DocumentWorkerPool) findResolution(ofeIdentificacion, resolutionNumber, prefix string) (*coreresolution.Resolution, error) {
	// Extract base NIT (without DV) for resolution query
	// The system receives NIT with DV (e.g., "860011153-6") but Numrot API expects only the base NIT (e.g., "860011153")
	baseNIT := ofeIdentificacion
	if parts := strings.Split(ofeIdentificacion, "-"); len(parts) > 1 {
		baseNIT = parts[0]
	}

	// Check cache first (using base NIT for cache key)
	cacheKey := baseNIT + ":" + resolutionNumber + ":" + prefix
	if cached, ok := p.resolutionCache.Load(cacheKey); ok {
		if resolution, ok := cached.(*coreresolution.Resolution); ok {
			return resolution, nil
		}
	}

	// Query resolutions from provider using base NIT
	resolutions, err := p.provider.GetResolutions(p.ctx, baseNIT)
	if err != nil {
		return nil, fmt.Errorf("error al consultar resoluciones para OFE [%s]: %w", baseNIT, err)
	}

	// Find matching resolution
	for i := range resolutions {
		if resolutions[i].ResolutionNumber == resolutionNumber && resolutions[i].Prefix == prefix {
			// Cache the resolution
			p.resolutionCache.Store(cacheKey, &resolutions[i])
			return &resolutions[i], nil
		}
	}

	return nil, fmt.Errorf("resolución [%s] con prefijo [%s] no encontrada para el OFE [%s]",
		resolutionNumber, prefix, ofeIdentificacion)
}

// ProcessDocuments processes documents concurrently using the worker pool
func (p *DocumentWorkerPool) ProcessDocuments(ctx context.Context, documents []invoice.OpenETLDocument, documentType string) ([]invoice.OpenETLDocument, []invoice.FailedDocument) {
	startTime := time.Now()
	now := time.Now()
	fechaProcesamiento := now.Format("2006-01-02")
	horaProcesamiento := now.Format("15:04:05")

	// Start the worker pool
	p.Start()
	defer func() {
		p.Stop()
		duration := time.Since(startTime)
		// Note: We don't have logger here, but metrics can be added via aggregator
		_ = duration // Suppress unused variable warning
	}()

	// Submit all jobs
	for i, doc := range documents {
		job := DocumentJob{
			Document:     doc,
			DocumentType: documentType,
			Index:        i,
		}
		if err := p.Submit(job); err != nil {
			// Context cancelled or pool stopped
			return make([]invoice.OpenETLDocument, 0), []invoice.FailedDocument{
				{
					Documento:          documentType,
					Consecutivo:        doc.CdoConsecutivo,
					Prefijo:            doc.RfaPrefijo,
					Errors:             []string{"Worker pool stopped: " + err.Error()},
					FechaProcesamiento: fechaProcesamiento,
					HoraProcesamiento:  horaProcesamiento,
				},
			}
		}
	}

	// Collect results
	validDocuments := make([]invoice.OpenETLDocument, 0)
	failedDocuments := make([]invoice.FailedDocument, 0)
	resultsReceived := 0
	expectedResults := len(documents)

	for resultsReceived < expectedResults {
		select {
		case result, ok := <-p.Results():
			if !ok {
				// Channel closed
				// Ensure slices are never nil (use empty slice instead)
				if validDocuments == nil {
					validDocuments = make([]invoice.OpenETLDocument, 0)
				}
				if failedDocuments == nil {
					failedDocuments = make([]invoice.FailedDocument, 0)
				}
				return validDocuments, failedDocuments
			}
			resultsReceived++

			if result.Failed {
				failedDocuments = append(failedDocuments, invoice.FailedDocument{
					Documento:          documentType,
					Consecutivo:        result.Document.CdoConsecutivo,
					Prefijo:            result.Document.RfaPrefijo,
					Errors:             []string{result.ErrorMessage},
					FechaProcesamiento: fechaProcesamiento,
					HoraProcesamiento:  horaProcesamiento,
				})
			} else {
				validDocuments = append(validDocuments, result.Document)
			}

		case <-ctx.Done():
			// Context cancelled
			// Ensure slices are never nil (use empty slice instead)
			if validDocuments == nil {
				validDocuments = make([]invoice.OpenETLDocument, 0)
			}
			if failedDocuments == nil {
				failedDocuments = make([]invoice.FailedDocument, 0)
			}
			return validDocuments, failedDocuments
		}
	}

	// Ensure slices are never nil (use empty slice instead)
	if validDocuments == nil {
		validDocuments = make([]invoice.OpenETLDocument, 0)
	}
	if failedDocuments == nil {
		failedDocuments = make([]invoice.FailedDocument, 0)
	}

	return validDocuments, failedDocuments
}
