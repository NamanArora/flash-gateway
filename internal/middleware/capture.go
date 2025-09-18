package middleware

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/NamanArora/flash-gateway/internal/storage"
	"github.com/google/uuid"
)

// CaptureMiddleware captures request/response data for logging
type CaptureMiddleware struct {
	writer          *storage.AsyncLogWriter
	maxBodySize     int
	sensitiveHeaders map[string]bool
	skipHealthCheck bool
}

// CaptureConfig holds configuration for the capture middleware
type CaptureConfig struct {
	Writer           *storage.AsyncLogWriter
	MaxBodySize      int    // Maximum body size to capture (bytes)
	SkipHealthCheck  bool   // Skip logging for /health endpoint
}

// NewCaptureMiddleware creates a new capture middleware
func NewCaptureMiddleware(config CaptureConfig) *CaptureMiddleware {
	if config.MaxBodySize <= 0 {
		config.MaxBodySize = 6400 * 1024 // 64KB default
	}

	sensitiveHeaders := map[string]bool{
		"authorization": true,
		"x-api-key":     true,
		"cookie":        true,
		"x-auth-token":  true,
		"bearer":        true,
	}

	return &CaptureMiddleware{
		writer:           config.Writer,
		maxBodySize:      config.MaxBodySize,
		sensitiveHeaders: sensitiveHeaders,
		skipHealthCheck:  config.SkipHealthCheck,
	}
}

// Capture wraps an HTTP handler to capture request/response data
func (c *CaptureMiddleware) Capture(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip logging if writer is not available
		if c.writer == nil {
			next.ServeHTTP(w, r)
			return
		}

		// Skip health check if configured
		if c.skipHealthCheck && (r.URL.Path == "/health" || r.URL.Path == "/status") {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		requestID := uuid.New()

		// Create request log entry
		requestLog := storage.NewRequestLog()
		requestLog.RequestID = requestID
		requestLog.Timestamp = start
		requestLog.Endpoint = r.URL.Path
		requestLog.Method = r.Method
		userAgent := r.UserAgent()
		requestLog.UserAgent = &userAgent
		requestLog.RemoteAddr = &r.RemoteAddr

		// Extract session ID from headers or generate one
		sessionID := extractSessionID(r)
		if sessionID != "" {
			requestLog.SessionID = &sessionID
		}

		// Capture request headers (sanitized)
		requestLog.RequestHeaders = c.captureHeaders(r.Header)

		// Capture request body
		var requestBody string
		if r.Body != nil && (r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH") {
			body, err := c.captureBody(r.Body, c.maxBodySize)
			if err == nil {
				requestBody = body
				requestLog.RequestBody = &requestBody
				
				// Replace body with captured content
				r.Body = io.NopCloser(strings.NewReader(requestBody))
			}
		}

		// Create response capture writer
		captureWriter := &captureResponseWriter{
			ResponseWriter: w,
			statusCode:     200,
			body:          &bytes.Buffer{},
			maxBodySize:   c.maxBodySize,
		}

		// Add request ID to context for guardrails
		ctx := context.WithValue(r.Context(), "request_id", requestID)
		r = r.WithContext(ctx)

		// Process request
		next.ServeHTTP(captureWriter, r)

		// Calculate latency
		latency := time.Since(start)
		latencyMs := latency.Nanoseconds() / 1000000

		// Complete the log entry
		requestLog.StatusCode = &captureWriter.statusCode
		requestLog.LatencyMs = &latencyMs

		// Capture response headers
		requestLog.ResponseHeaders = c.captureHeaders(captureWriter.Header())

		// Capture response body
		if captureWriter.body.Len() > 0 {
			responseBody := captureWriter.body.String()
			log.Printf("[LOG] Response body 1: %v", responseBody)
			
			// Check if response is gzipped and decompress for logging
			contentEncoding := captureWriter.Header().Get("Content-Encoding")
			if strings.Contains(strings.ToLower(contentEncoding), "gzip") {
				if decompressed, err := decompressGzip([]byte(responseBody)); err == nil {
					responseBody = string(decompressed)
				} else {
					log.Printf("Warning: Failed to decompress gzipped response for logging: %v", err)
				}
			}
			
			requestLog.ResponseBody = &responseBody
		}

		// Determine provider from request path
		if provider := extractProvider(r.URL.Path); provider != "" {
			requestLog.Provider = &provider
		}

		// Add metadata
		requestLog.Metadata = map[string]interface{}{
			"request_size":  len(requestBody),
			"response_size": captureWriter.body.Len(),
			"content_type":  r.Header.Get("Content-Type"),
		}

		// Write log asynchronously
		c.writer.WriteLog(requestLog)
	})
}

// captureHeaders captures and sanitizes HTTP headers
func (c *CaptureMiddleware) captureHeaders(headers http.Header) map[string]interface{} {
	captured := make(map[string]interface{})
	
	for key, values := range headers {
		lowerKey := strings.ToLower(key)
		
		if c.sensitiveHeaders[lowerKey] {
			captured[key] = "[REDACTED]"
		} else {
			// Store as string if single value, array if multiple
			if len(values) == 1 {
				captured[key] = values[0]
			} else {
				captured[key] = values
			}
		}
	}
	
	return captured
}

// captureBody captures request/response body with size limit
func (c *CaptureMiddleware) captureBody(body io.ReadCloser, maxSize int) (string, error) {
	defer body.Close()
	
	// Use LimitReader to prevent reading too much data
	limitReader := io.LimitReader(body, int64(maxSize))
	
	buf := &bytes.Buffer{}
	_, err := buf.ReadFrom(limitReader)
	if err != nil {
		return "", err
	}
	
	captured := buf.String()
	log.Printf("Extracted body: %v", captured)
	
	// Add truncation marker if we hit the limit
	if buf.Len() >= maxSize {
		captured += "\n... [TRUNCATED]"
	}
	
	return captured, nil
}

// extractSessionID extracts session ID from various headers
func extractSessionID(r *http.Request) string {
	// Try different common session headers
	if sessionID := r.Header.Get("X-Session-ID"); sessionID != "" {
		return sessionID
	}
	
	if sessionID := r.Header.Get("X-Request-ID"); sessionID != "" {
		return sessionID
	}
	
	if sessionID := r.Header.Get("X-Correlation-ID"); sessionID != "" {
		return sessionID
	}
	
	// Try to extract from Authorization header (last part after space)
	if auth := r.Header.Get("Authorization"); auth != "" {
		parts := strings.Split(auth, " ")
		if len(parts) > 1 {
			// Use last 8 characters as session identifier (for grouping)
			token := parts[len(parts)-1]
			if len(token) > 8 {
				return token[len(token)-8:]
			}
		}
	}
	
	return ""
}

// extractProvider determines the AI provider from the request path
func extractProvider(path string) string {
	if strings.HasPrefix(path, "/v1/") {
		// Most paths starting with /v1/ are OpenAI
		return "openai"
	}
	
	if strings.Contains(path, "anthropic") {
		return "anthropic"
	}
	
	if strings.Contains(path, "messages") {
		return "anthropic"
	}
	
	return ""
}

// captureResponseWriter wraps http.ResponseWriter to capture response data
type captureResponseWriter struct {
	http.ResponseWriter
	statusCode  int
	body        *bytes.Buffer
	maxBodySize int
}

// WriteHeader captures the status code
func (w *captureResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

// Write captures the response body while writing to the client
func (w *captureResponseWriter) Write(data []byte) (int, error) {
	// Write to client first
	n, err := w.ResponseWriter.Write(data)
	
	// Capture response body if under size limit
	if w.body.Len()+len(data) <= w.maxBodySize {
		w.body.Write(data)
	} else if w.body.Len() < w.maxBodySize {
		// Write partial data up to limit
		remaining := w.maxBodySize - w.body.Len()
		w.body.Write(data[:remaining])
		w.body.WriteString("\n... [TRUNCATED]")
	}
	
	return n, err
}

// Header returns the header map
func (w *captureResponseWriter) Header() http.Header {
	return w.ResponseWriter.Header()
}

// Flush implements http.Flusher if the underlying ResponseWriter supports it
func (w *captureResponseWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// Hijack implements http.Hijacker if the underlying ResponseWriter supports it
func (w *captureResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := w.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, fmt.Errorf("hijacking not supported")
}

// decompressGzip decompresses gzip-compressed data for logging purposes
func decompressGzip(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer reader.Close()
	
	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read decompressed data: %w", err)
	}
	
	return decompressed, nil
}

