package guardrails

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"sync"
	"time"
)

// MetricsWriter handles asynchronous writing of guardrail metrics to the database
type MetricsWriter struct {
	db          *sql.DB
	channel     chan *Metric
	batchSize   int
	workers     int
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	
	// Metrics for monitoring
	mutex       sync.RWMutex
	totalWrites int64
	droppedWrites int64
	failedBatches int64
}

// MetricsWriterConfig holds configuration for the metrics writer
type MetricsWriterConfig struct {
	DB         *sql.DB
	BufferSize int
	BatchSize  int
	Workers    int
}

// NewMetricsWriter creates a new metrics writer
func NewMetricsWriter(config MetricsWriterConfig) *MetricsWriter {
	if config.BufferSize <= 0 {
		config.BufferSize = 1000
	}
	if config.BatchSize <= 0 {
		config.BatchSize = 10
	}
	if config.Workers <= 0 {
		config.Workers = 2
	}

	ctx, cancel := context.WithCancel(context.Background())
	
	writer := &MetricsWriter{
		db:        config.DB,
		channel:   make(chan *Metric, config.BufferSize),
		batchSize: config.BatchSize,
		workers:   config.Workers,
		ctx:       ctx,
		cancel:    cancel,
	}

	// Start worker goroutines
	writer.start()
	
	return writer
}

// Write queues a metric for asynchronous writing
func (m *MetricsWriter) Write(metric *Metric) {
	if metric == nil {
		return
	}
	
	// Set created time if not already set
	if metric.CreatedAt.IsZero() {
		metric.CreatedAt = time.Now()
	}
	
	select {
	case m.channel <- metric:
		m.mutex.Lock()
		m.totalWrites++
		m.mutex.Unlock()
	default:
		// Channel is full, drop the metric to avoid blocking
		m.mutex.Lock()
		m.droppedWrites++
		m.mutex.Unlock()
		log.Printf("[WARNING] Guardrail metrics channel full, dropping metric for %s", metric.GuardrailName)
	}
}

// start initializes worker goroutines
func (m *MetricsWriter) start() {
	for i := 0; i < m.workers; i++ {
		m.wg.Add(1)
		go m.worker()
	}
}

// worker processes metrics from the channel in batches
func (m *MetricsWriter) worker() {
	defer m.wg.Done()
	
	batch := make([]*Metric, 0, m.batchSize)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-m.ctx.Done():
			// Flush remaining metrics before shutdown
			if len(batch) > 0 {
				m.flushBatch(batch)
			}
			return
			
		case metric := <-m.channel:
			batch = append(batch, metric)
			
			// Flush if batch is full
			if len(batch) >= m.batchSize {
				m.flushBatch(batch)
				batch = batch[:0] // Reset batch
			}
			
		case <-ticker.C:
			// Periodic flush even if batch is not full
			if len(batch) > 0 {
				m.flushBatch(batch)
				batch = batch[:0] // Reset batch
			}
		}
	}
}

// flushBatch writes a batch of metrics to the database
func (m *MetricsWriter) flushBatch(batch []*Metric) {
	if len(batch) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	if err := m.saveBatch(ctx, batch); err != nil {
		m.mutex.Lock()
		m.failedBatches++
		m.mutex.Unlock()
		log.Printf("[ERROR] Failed to save guardrail metrics batch of %d entries: %v", len(batch), err)
	}
}

// saveBatch performs batch insert of metrics
func (m *MetricsWriter) saveBatch(ctx context.Context, batch []*Metric) error {
	query := `
		INSERT INTO guardrail_metrics (
			id, request_id, guardrail_name, layer, priority,
			start_time, end_time, duration_ms, passed, score,
			error, metadata, original_response, override_response, 
			response_overridden, created_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)`

	stmt, err := m.db.PrepareContext(ctx, query)
	if err != nil {
		return err
	}
	defer stmt.Close()

	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, metric := range batch {
		// Marshal metadata to JSON
		var metadataJSON []byte
		if metric.Metadata != nil {
			metadataJSON, _ = json.Marshal(metric.Metadata)
		}

		_, err = tx.StmtContext(ctx, stmt).ExecContext(
			ctx,
			metric.ID,
			metric.RequestID,
			metric.GuardrailName,
			metric.Layer,
			metric.Priority,
			metric.StartTime,
			metric.EndTime,
			metric.DurationMs,
			metric.Passed,
			metric.Score,
			metric.Error,
			metadataJSON,
			metric.OriginalResponse,
			metric.OverrideResponse,
			metric.ResponseOverridden,
			metric.CreatedAt,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetMetrics returns current metrics for monitoring
func (m *MetricsWriter) GetMetrics() map[string]interface{} {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	return map[string]interface{}{
		"total_writes":     m.totalWrites,
		"dropped_writes":   m.droppedWrites,
		"failed_batches":   m.failedBatches,
		"channel_depth":    len(m.channel),
		"channel_capacity": cap(m.channel),
		"workers":          m.workers,
		"batch_size":       m.batchSize,
	}
}

// Close gracefully shuts down the metrics writer
func (m *MetricsWriter) Close() error {
	log.Println("Shutting down guardrail metrics writer...")
	
	// Stop accepting new metrics
	m.cancel()
	
	// Wait for workers to finish processing
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()
	
	// Wait with timeout
	select {
	case <-done:
		log.Println("All guardrail metrics workers finished")
	case <-time.After(30 * time.Second):
		log.Println("Timeout waiting for guardrail metrics workers to finish")
	}
	
	// Print final metrics
	metrics := m.GetMetrics()
	log.Printf("Final guardrail metrics writer stats: %+v", metrics)
	
	return nil
}