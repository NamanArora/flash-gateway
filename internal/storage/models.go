package storage

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// RequestLog represents a single API request/response log entry
type RequestLog struct {
	ID             uuid.UUID              `json:"id" db:"id"`
	Timestamp      time.Time              `json:"timestamp" db:"timestamp"`
	SessionID      *string                `json:"session_id,omitempty" db:"session_id"`
	RequestID      uuid.UUID              `json:"request_id" db:"request_id"`
	Endpoint       string                 `json:"endpoint" db:"endpoint"`
	Method         string                 `json:"method" db:"method"`
	StatusCode     *int                   `json:"status_code,omitempty" db:"status_code"`
	LatencyMs      *int64                 `json:"latency_ms,omitempty" db:"latency_ms"`
	Provider       *string                `json:"provider,omitempty" db:"provider"`
	UserAgent      *string                `json:"user_agent,omitempty" db:"user_agent"`
	RemoteAddr     *string                `json:"remote_addr,omitempty" db:"remote_addr"`
	RequestHeaders map[string]interface{} `json:"request_headers,omitempty" db:"request_headers"`
	RequestBody    *string                `json:"request_body,omitempty" db:"request_body"`
	ResponseHeaders map[string]interface{} `json:"response_headers,omitempty" db:"response_headers"`
	ResponseBody   *string                `json:"response_body,omitempty" db:"response_body"`
	Error          *string                `json:"error,omitempty" db:"error"`
	Metadata       map[string]interface{} `json:"metadata,omitempty" db:"metadata"`
	CreatedAt      time.Time              `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at" db:"updated_at"`
}

// LogFilter represents filtering options for querying logs
type LogFilter struct {
	StartTime   *time.Time `json:"start_time,omitempty"`
	EndTime     *time.Time `json:"end_time,omitempty"`
	Endpoint    *string    `json:"endpoint,omitempty"`
	Method      *string    `json:"method,omitempty"`
	StatusCode  *int       `json:"status_code,omitempty"`
	Provider    *string    `json:"provider,omitempty"`
	SessionID   *string    `json:"session_id,omitempty"`
	HasError    *bool      `json:"has_error,omitempty"`
	Limit       int        `json:"limit"`
	Offset      int        `json:"offset"`
	OrderBy     string     `json:"order_by"`
	OrderDir    string     `json:"order_dir"`
}

// LogStats represents aggregated statistics about logs
type LogStats struct {
	TotalRequests    int64                  `json:"total_requests"`
	AverageLatency   float64                `json:"average_latency_ms"`
	ErrorRate        float64                `json:"error_rate"`
	RequestsPerHour  int64                  `json:"requests_per_hour"`
	TopEndpoints     []EndpointStats        `json:"top_endpoints"`
	StatusCodeCounts map[string]int64       `json:"status_code_counts"`
	ProviderStats    map[string]int64       `json:"provider_stats"`
}

// EndpointStats represents statistics for a specific endpoint
type EndpointStats struct {
	Endpoint       string  `json:"endpoint"`
	RequestCount   int64   `json:"request_count"`
	AverageLatency float64 `json:"average_latency_ms"`
	ErrorRate      float64 `json:"error_rate"`
}

// MarshalHeaders converts headers map to JSON for database storage
func MarshalHeaders(headers map[string]interface{}) ([]byte, error) {
	if headers == nil {
		return nil, nil
	}
	return json.Marshal(headers)
}

// UnmarshalHeaders converts JSON from database to headers map
func UnmarshalHeaders(data []byte) (map[string]interface{}, error) {
	if data == nil {
		return nil, nil
	}
	var headers map[string]interface{}
	err := json.Unmarshal(data, &headers)
	return headers, err
}

// SanitizeForLog removes sensitive information from headers
func SanitizeForLog(headers map[string]interface{}) map[string]interface{} {
	if headers == nil {
		return nil
	}
	
	sanitized := make(map[string]interface{})
	sensitiveHeaders := map[string]bool{
		"authorization": true,
		"x-api-key":     true,
		"cookie":        true,
		"x-auth-token":  true,
		"bearer":        true,
	}
	
	for key, value := range headers {
		lowerKey := key
		if sensitiveHeaders[lowerKey] {
			sanitized[key] = "[REDACTED]"
		} else {
			sanitized[key] = value
		}
	}
	
	return sanitized
}

// TruncateBody truncates request/response body if too large
func TruncateBody(body string, maxLength int) string {
	if len(body) <= maxLength {
		return body
	}
	return body[:maxLength] + "... [truncated]"
}

// NewRequestLog creates a new request log with default values
func NewRequestLog() *RequestLog {
	now := time.Now()
	return &RequestLog{
		ID:        uuid.New(),
		RequestID: uuid.New(),
		Timestamp: now,
		CreatedAt: now,
		UpdatedAt: now,
	}
}