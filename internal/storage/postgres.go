package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

// PostgreSQLStorage implements StorageBackend for PostgreSQL
type PostgreSQLStorage struct {
	db *sql.DB
}

// PostgreSQLConfig holds configuration for PostgreSQL connection
type PostgreSQLConfig struct {
	ConnectionURL   string
	MaxConnections  int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// NewPostgreSQLStorage creates a new PostgreSQL storage backend
func NewPostgreSQLStorage(config PostgreSQLConfig) (*PostgreSQLStorage, error) {
	db, err := sql.Open("postgres", config.ConnectionURL)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	if config.MaxConnections > 0 {
		db.SetMaxOpenConns(config.MaxConnections)
	} else {
		db.SetMaxOpenConns(25)
	}
	
	if config.MaxIdleConns > 0 {
		db.SetMaxIdleConns(config.MaxIdleConns)
	} else {
		db.SetMaxIdleConns(5)
	}
	
	if config.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(config.ConnMaxLifetime)
	} else {
		db.SetConnMaxLifetime(time.Hour)
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Println("Connected to PostgreSQL successfully")

	return &PostgreSQLStorage{db: db}, nil
}

// SaveRequestLog saves a single request log
func (p *PostgreSQLStorage) SaveRequestLog(ctx context.Context, requestLog *RequestLog) error {
	return p.SaveRequestLogsBatch(ctx, []*RequestLog{requestLog})
}

// SaveRequestLogsBatch saves multiple request logs in a single transaction
func (p *PostgreSQLStorage) SaveRequestLogsBatch(ctx context.Context, logs []*RequestLog) error {
	if len(logs) == 0 {
		return nil
	}

	tx, err := p.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// Prepare batch insert
	query := `
		INSERT INTO request_logs (
			id, timestamp, session_id, request_id, endpoint, method, 
			status_code, latency_ms, provider, user_agent, remote_addr,
			request_headers, request_body, response_headers, response_body,
			error, metadata, created_at, updated_at
		) VALUES `

	values := make([]interface{}, 0, len(logs)*19)
	placeholders := make([]string, 0, len(logs))
	t := log.Printf

	for i, log := range logs {
		placeholderStart := i*19 + 1
		placeholders = append(placeholders, fmt.Sprintf(
			"($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d)",
			placeholderStart, placeholderStart+1, placeholderStart+2, placeholderStart+3,
			placeholderStart+4, placeholderStart+5, placeholderStart+6, placeholderStart+7,
			placeholderStart+8, placeholderStart+9, placeholderStart+10, placeholderStart+11,
			placeholderStart+12, placeholderStart+13, placeholderStart+14, placeholderStart+15,
			placeholderStart+16, placeholderStart+17, placeholderStart+18,
		))

		// Convert headers to JSON
		reqHeadersJSON, _ := json.Marshal(log.RequestHeaders)
		respHeadersJSON, _ := json.Marshal(log.ResponseHeaders)
		metadataJSON, _ := json.Marshal(log.Metadata)

		values = append(values,
			log.ID,
			log.Timestamp,
			log.SessionID,
			log.RequestID,
			log.Endpoint,
			log.Method,
			log.StatusCode,
			log.LatencyMs,
			log.Provider,
			log.UserAgent,
			log.RemoteAddr,
			reqHeadersJSON,
			log.RequestBody,
			respHeadersJSON,
			log.ResponseBody,
			log.Error,
			metadataJSON,
			log.CreatedAt,
			log.UpdatedAt,
		)
		t("[LOG] Response body: %v", *log.ResponseBody)
	}

	

	query += strings.Join(placeholders, ", ")

	_, err = tx.ExecContext(ctx, query, values...)
	if err != nil {
		return fmt.Errorf("failed to insert logs: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetRequestLogs retrieves request logs based on filter criteria
func (p *PostgreSQLStorage) GetRequestLogs(ctx context.Context, filter LogFilter) ([]*RequestLog, error) {
	query := `
		SELECT id, timestamp, session_id, request_id, endpoint, method,
			   status_code, latency_ms, provider, user_agent, remote_addr,
			   request_headers, request_body, response_headers, response_body,
			   error, metadata, created_at, updated_at
		FROM request_logs
		WHERE 1=1`

	args := make([]interface{}, 0)
	argCount := 0

	// Apply filters
	if filter.StartTime != nil {
		argCount++
		query += fmt.Sprintf(" AND timestamp >= $%d", argCount)
		args = append(args, *filter.StartTime)
	}
	
	if filter.EndTime != nil {
		argCount++
		query += fmt.Sprintf(" AND timestamp <= $%d", argCount)
		args = append(args, *filter.EndTime)
	}
	
	if filter.Endpoint != nil {
		argCount++
		query += fmt.Sprintf(" AND endpoint = $%d", argCount)
		args = append(args, *filter.Endpoint)
	}
	
	if filter.Method != nil {
		argCount++
		query += fmt.Sprintf(" AND method = $%d", argCount)
		args = append(args, *filter.Method)
	}
	
	if filter.StatusCode != nil {
		argCount++
		query += fmt.Sprintf(" AND status_code = $%d", argCount)
		args = append(args, *filter.StatusCode)
	}
	
	if filter.Provider != nil {
		argCount++
		query += fmt.Sprintf(" AND provider = $%d", argCount)
		args = append(args, *filter.Provider)
	}
	
	if filter.SessionID != nil {
		argCount++
		query += fmt.Sprintf(" AND session_id = $%d", argCount)
		args = append(args, *filter.SessionID)
	}
	
	if filter.HasError != nil && *filter.HasError {
		query += " AND error IS NOT NULL"
	} else if filter.HasError != nil && !*filter.HasError {
		query += " AND error IS NULL"
	}

	// Order by
	orderBy := "timestamp"
	if filter.OrderBy != "" {
		orderBy = filter.OrderBy
	}
	
	orderDir := "DESC"
	if filter.OrderDir != "" {
		orderDir = filter.OrderDir
	}
	
	query += fmt.Sprintf(" ORDER BY %s %s", orderBy, orderDir)

	// Limit and offset
	if filter.Limit > 0 {
		argCount++
		query += fmt.Sprintf(" LIMIT $%d", argCount)
		args = append(args, filter.Limit)
		
		if filter.Offset > 0 {
			argCount++
			query += fmt.Sprintf(" OFFSET $%d", argCount)
			args = append(args, filter.Offset)
		}
	}

	rows, err := p.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query logs: %w", err)
	}
	defer rows.Close()

	var logs []*RequestLog
	for rows.Next() {
		log := &RequestLog{}
		var reqHeadersJSON, respHeadersJSON, metadataJSON []byte

		err := rows.Scan(
			&log.ID,
			&log.Timestamp,
			&log.SessionID,
			&log.RequestID,
			&log.Endpoint,
			&log.Method,
			&log.StatusCode,
			&log.LatencyMs,
			&log.Provider,
			&log.UserAgent,
			&log.RemoteAddr,
			&reqHeadersJSON,
			&log.RequestBody,
			&respHeadersJSON,
			&log.ResponseBody,
			&log.Error,
			&metadataJSON,
			&log.CreatedAt,
			&log.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan log: %w", err)
		}

		// Unmarshal JSON fields
		if reqHeadersJSON != nil {
			json.Unmarshal(reqHeadersJSON, &log.RequestHeaders)
		}
		if respHeadersJSON != nil {
			json.Unmarshal(respHeadersJSON, &log.ResponseHeaders)
		}
		if metadataJSON != nil {
			json.Unmarshal(metadataJSON, &log.Metadata)
		}

		logs = append(logs, log)
	}

	return logs, nil
}

// GetRequestLogByID retrieves a single request log by ID
func (p *PostgreSQLStorage) GetRequestLogByID(ctx context.Context, id string) (*RequestLog, error) {
	logID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid UUID: %w", err)
	}

	query := `
		SELECT id, timestamp, session_id, request_id, endpoint, method,
			   status_code, latency_ms, provider, user_agent, remote_addr,
			   request_headers, request_body, response_headers, response_body,
			   error, metadata, created_at, updated_at
		FROM request_logs
		WHERE id = $1`

	log := &RequestLog{}
	var reqHeadersJSON, respHeadersJSON, metadataJSON []byte

	err = p.db.QueryRowContext(ctx, query, logID).Scan(
		&log.ID,
		&log.Timestamp,
		&log.SessionID,
		&log.RequestID,
		&log.Endpoint,
		&log.Method,
		&log.StatusCode,
		&log.LatencyMs,
		&log.Provider,
		&log.UserAgent,
		&log.RemoteAddr,
		&reqHeadersJSON,
		&log.RequestBody,
		&respHeadersJSON,
		&log.ResponseBody,
		&log.Error,
		&metadataJSON,
		&log.CreatedAt,
		&log.UpdatedAt,
	)
	
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get log: %w", err)
	}

	// Unmarshal JSON fields
	if reqHeadersJSON != nil {
		json.Unmarshal(reqHeadersJSON, &log.RequestHeaders)
	}
	if respHeadersJSON != nil {
		json.Unmarshal(respHeadersJSON, &log.ResponseHeaders)
	}
	if metadataJSON != nil {
		json.Unmarshal(metadataJSON, &log.Metadata)
	}

	return log, nil
}

// GetLogStats retrieves aggregated statistics
func (p *PostgreSQLStorage) GetLogStats(ctx context.Context, filter LogFilter) (*LogStats, error) {
	// This is a simplified implementation - in production you'd want more sophisticated aggregations
	stats := &LogStats{
		StatusCodeCounts: make(map[string]int64),
		ProviderStats:    make(map[string]int64),
	}

	// Get total count
	err := p.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM request_logs").Scan(&stats.TotalRequests)
	if err != nil {
		return nil, fmt.Errorf("failed to get total count: %w", err)
	}

	// Get average latency (for successful requests)
	err = p.db.QueryRowContext(ctx, 
		"SELECT COALESCE(AVG(latency_ms), 0) FROM request_logs WHERE latency_ms IS NOT NULL AND status_code < 400",
	).Scan(&stats.AverageLatency)
	if err != nil {
		return nil, fmt.Errorf("failed to get average latency: %w", err)
	}

	return stats, nil
}

// Close closes the database connection
func (p *PostgreSQLStorage) Close() error {
	if p.db != nil {
		return p.db.Close()
	}
	return nil
}

// GetDB returns the database connection for external use (e.g., guardrails metrics)
func (p *PostgreSQLStorage) GetDB() *sql.DB {
	return p.db
}