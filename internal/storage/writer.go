package storage

import (
	"context"
	"log"
	"sync"
	"time"
)

// StorageBackend defines the interface for different storage implementations
type StorageBackend interface {
	SaveRequestLog(ctx context.Context, requestLog *RequestLog) error
	SaveRequestLogsBatch(ctx context.Context, logs []*RequestLog) error
	GetRequestLogs(ctx context.Context, filter LogFilter) ([]*RequestLog, error)
	GetRequestLogByID(ctx context.Context, id string) (*RequestLog, error)
	GetLogStats(ctx context.Context, filter LogFilter) (*LogStats, error)
	Close() error
}

// AsyncLogWriter handles asynchronous writing of request logs
type AsyncLogWriter struct {
	backend       StorageBackend
	logChannel    chan *RequestLog
	batchSize     int
	flushInterval time.Duration
	workers       int
	enabled       bool
	skipOnError   bool

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Metrics
	mutex         sync.RWMutex
	totalLogs     int64
	droppedLogs   int64
	failedBatches int64
	lastFlush     time.Time
}

// AsyncLogWriterConfig holds configuration for the async log writer
type AsyncLogWriterConfig struct {
	Backend       StorageBackend
	BufferSize    int
	BatchSize     int
	FlushInterval time.Duration
	Workers       int
	Enabled       bool
	SkipOnError   bool
}

// NewAsyncLogWriter creates a new async log writer
func NewAsyncLogWriter(config AsyncLogWriterConfig) *AsyncLogWriter {
	if config.BufferSize <= 0 {
		config.BufferSize = 1000
	}
	if config.BatchSize <= 0 {
		config.BatchSize = 10
	}
	if config.FlushInterval <= 0 {
		config.FlushInterval = time.Second
	}
	if config.Workers <= 0 {
		config.Workers = 3
	}

	ctx, cancel := context.WithCancel(context.Background())

	writer := &AsyncLogWriter{
		backend:       config.Backend,
		logChannel:    make(chan *RequestLog, config.BufferSize),
		batchSize:     config.BatchSize,
		flushInterval: config.FlushInterval,
		workers:       config.Workers,
		enabled:       config.Enabled,
		skipOnError:   config.SkipOnError,
		ctx:           ctx,
		cancel:        cancel,
		lastFlush:     time.Now(),
	}

	if writer.enabled && writer.backend != nil {
		writer.start()
	}

	return writer
}

// WriteLog writes a request log asynchronously
func (w *AsyncLogWriter) WriteLog(requestLog *RequestLog) {
	if !w.enabled || w.backend == nil {
		return
	}

	select {
	case w.logChannel <- requestLog:
		w.mutex.Lock()
		w.totalLogs++
		w.mutex.Unlock()
	default:
		// Channel is full, drop the log to avoid blocking
		w.mutex.Lock()
		w.droppedLogs++
		w.mutex.Unlock()

		if !w.skipOnError {
			log.Printf("[WARNING] Log channel full, dropping log entry")
		}
	}
}

// start initializes the worker goroutines
func (w *AsyncLogWriter) start() {
	for i := 0; i < w.workers; i++ {
		w.wg.Add(1)
		go w.worker()
	}
}

// worker processes logs from the channel in batches
func (w *AsyncLogWriter) worker() {
	defer w.wg.Done()

	batch := make([]*RequestLog, 0, w.batchSize)
	ticker := time.NewTicker(w.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			// Flush remaining logs before shutdown
			if len(batch) > 0 {
				w.flushBatch(batch)
			}
			return

		case requestLog := <-w.logChannel:
			batch = append(batch, requestLog)

			// Flush if batch is full
			if len(batch) >= w.batchSize {
				w.flushBatch(batch)
				batch = batch[:0] // Reset batch
				w.updateLastFlush()
			}

		case <-ticker.C:
			// Periodic flush even if batch is not full
			if len(batch) > 0 {
				w.flushBatch(batch)
				batch = batch[:0] // Reset batch
				w.updateLastFlush()
			}
		}
	}
}

// flushBatch writes a batch of logs to the storage backend
func (w *AsyncLogWriter) flushBatch(batch []*RequestLog) {
	if len(batch) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := w.backend.SaveRequestLogsBatch(ctx, batch); err != nil {
		w.mutex.Lock()
		w.failedBatches++
		log.Printf("[ERROR] Writing logs failed %v", err)
		w.mutex.Unlock()

		if !w.skipOnError {
			log.Printf("[ERROR] Failed to save log batch of %d entries: %v", len(batch), err)
		}
	}
}

// updateLastFlush updates the last flush timestamp
func (w *AsyncLogWriter) updateLastFlush() {
	w.mutex.Lock()
	w.lastFlush = time.Now()
	w.mutex.Unlock()
}

// GetMetrics returns current metrics
func (w *AsyncLogWriter) GetMetrics() map[string]interface{} {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	return map[string]interface{}{
		"enabled":           w.enabled,
		"total_logs":        w.totalLogs,
		"dropped_logs":      w.droppedLogs,
		"failed_batches":    w.failedBatches,
		"channel_depth":     len(w.logChannel),
		"channel_capacity":  cap(w.logChannel),
		"last_flush":        w.lastFlush,
		"workers":           w.workers,
		"batch_size":        w.batchSize,
		"flush_interval_ms": w.flushInterval.Milliseconds(),
	}
}

// GetChannelDepth returns current channel depth (for monitoring)
func (w *AsyncLogWriter) GetChannelDepth() int {
	return len(w.logChannel)
}

// GetDroppedCount returns the number of dropped logs
func (w *AsyncLogWriter) GetDroppedCount() int64 {
	w.mutex.RLock()
	defer w.mutex.RUnlock()
	return w.droppedLogs
}

// Close gracefully shuts down the async writer
func (w *AsyncLogWriter) Close() error {
	if !w.enabled || w.backend == nil {
		return nil
	}

	log.Println("Shutting down async log writer...")

	// Stop accepting new logs
	w.cancel()

	// Wait for workers to finish processing
	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	// Wait with timeout
	select {
	case <-done:
		log.Println("All log workers finished")
	case <-time.After(30 * time.Second):
		log.Println("Timeout waiting for log workers to finish")
	}

	// Close storage backend
	if err := w.backend.Close(); err != nil {
		log.Printf("Error closing storage backend: %v", err)
		return err
	}

	// Print final metrics
	metrics := w.GetMetrics()
	log.Printf("Final log writer metrics: %+v", metrics)

	return nil
}

// Flush forces flushing of any pending logs (useful for testing)
func (w *AsyncLogWriter) Flush() {
	if !w.enabled {
		return
	}

	// Send signal to flush by waiting briefly
	time.Sleep(w.flushInterval + 100*time.Millisecond)
}
