package invoice

import (
	"context"
	"sync"
	"time"

	"3tcapital/ms_facturacion_core/internal/core/invoice"
)

// ResultAggregator aggregates results from multiple concurrent operations
type ResultAggregator struct {
	mu             sync.Mutex
	processedDocs  []invoice.ProcessedDocument
	failedDocs     []invoice.FailedDocument
	startTime      time.Time
	totalDocuments int
	processedCount int
	failedCount    int
}

// NewResultAggregator creates a new result aggregator
func NewResultAggregator(totalDocuments int) *ResultAggregator {
	return &ResultAggregator{
		processedDocs:  make([]invoice.ProcessedDocument, 0),
		failedDocs:     make([]invoice.FailedDocument, 0),
		startTime:      time.Now(),
		totalDocuments: totalDocuments,
	}
}

// AddProcessed adds a processed document to the aggregator
func (a *ResultAggregator) AddProcessed(doc invoice.ProcessedDocument) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.processedDocs = append(a.processedDocs, doc)
	a.processedCount++
}

// AddFailed adds a failed document to the aggregator
func (a *ResultAggregator) AddFailed(doc invoice.FailedDocument) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.failedDocs = append(a.failedDocs, doc)
	a.failedCount++
}

// GetResults returns the aggregated results
func (a *ResultAggregator) GetResults() ([]invoice.ProcessedDocument, []invoice.FailedDocument) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.processedDocs, a.failedDocs
}

// GetStats returns processing statistics
func (a *ResultAggregator) GetStats() ProcessingStats {
	a.mu.Lock()
	defer a.mu.Unlock()

	duration := time.Since(a.startTime)
	var throughput float64
	if duration.Seconds() > 0 {
		throughput = float64(a.processedCount) / duration.Seconds()
	}

	return ProcessingStats{
		TotalDocuments: a.totalDocuments,
		ProcessedCount: a.processedCount,
		FailedCount:    a.failedCount,
		Duration:       duration,
		Throughput:     throughput,
		SuccessRate:    float64(a.processedCount) / float64(a.totalDocuments) * 100,
	}
}

// ProcessingStats contains processing statistics
type ProcessingStats struct {
	TotalDocuments int
	ProcessedCount int
	FailedCount    int
	Duration       time.Duration
	Throughput     float64 // Documents per second
	SuccessRate    float64 // Percentage
}

// AggregateFromChannel aggregates results from a channel until context is done or channel is closed
func AggregateFromChannel(ctx context.Context, resultChan <-chan DocumentResult, documentType string) ([]invoice.OpenETLDocument, []invoice.FailedDocument) {
	now := time.Now()
	fechaProcesamiento := now.Format("2006-01-02")
	horaProcesamiento := now.Format("15:04:05")

	var validDocuments []invoice.OpenETLDocument
	var failedDocuments []invoice.FailedDocument

	for {
		select {
		case result, ok := <-resultChan:
			if !ok {
				// Channel closed
				return validDocuments, failedDocuments
			}

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
			return validDocuments, failedDocuments
		}
	}
}
