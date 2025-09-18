package handlers

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/NamanArora/flash-gateway/internal/guardrails"
	"github.com/NamanArora/flash-gateway/internal/providers"
	"github.com/google/uuid"
)

// ProxyHandler handles HTTP requests and proxies them to the appropriate provider
type ProxyHandler struct {
	providers        map[string]providers.Provider
	routes          map[string]string // endpoint -> provider mapping
	guardrailExecutor *guardrails.Executor
	responseBuilder  *GuardrailResponseBuilder
}

// NewProxyHandler creates a new proxy handler
func NewProxyHandler() *ProxyHandler {
	return &ProxyHandler{
		providers:       make(map[string]providers.Provider),
		routes:          make(map[string]string),
		responseBuilder: NewGuardrailResponseBuilder(),
	}
}

// SetGuardrailExecutor sets the guardrail executor for this proxy handler
func (h *ProxyHandler) SetGuardrailExecutor(executor *guardrails.Executor) {
	h.guardrailExecutor = executor
}

// RegisterProvider registers a provider and its supported endpoints
func (h *ProxyHandler) RegisterProvider(provider providers.Provider) {
	h.providers[provider.GetName()] = provider
	
	// Register all supported endpoints for this provider
	for _, endpoint := range provider.SupportedEndpoints() {
		h.routes[endpoint] = provider.GetName()
		log.Printf("Registered endpoint %s with provider %s", endpoint, provider.GetName())
	}
}

// ServeHTTP implements http.Handler interface
func (h *ProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Find the provider for this endpoint
	providerName, exists := h.routes[r.URL.Path]
	if !exists {
		http.Error(w, fmt.Sprintf("Endpoint %s not found", r.URL.Path), http.StatusNotFound)
		return
	}

	provider, exists := h.providers[providerName]
	if !exists {
		http.Error(w, fmt.Sprintf("Provider %s not available", providerName), http.StatusInternalServerError)
		return
	}

	// Validate HTTP method for this endpoint
	if !h.isMethodAllowed(r.URL.Path, r.Method, provider) {
		http.Error(w, fmt.Sprintf("Method %s not allowed for endpoint %s", r.Method, r.URL.Path), http.StatusMethodNotAllowed)
		return
	}

	// Get request ID from context (set by capture middleware)
	requestID := h.getRequestIDFromContext(r.Context())
	
	// Extract request body for guardrails (if applicable)
	var requestBody string
	if r.Body != nil && (r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH") {
		bodyBytes, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("Error reading request body: %v", err)
			http.Error(w, "Error reading request body", http.StatusBadRequest)
			return
		}
		requestBody = string(bodyBytes)
		
		// Replace the body so it can be read again by the provider
		r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	// Run input guardrails if enabled and executor is available
	if h.guardrailExecutor != nil && len(requestBody) > 0 {
		result, err := h.guardrailExecutor.ExecuteInput(r.Context(), requestID, requestBody)
		if err != nil {
			log.Printf("Input guardrails execution error: %v", err)
			h.returnGuardrailError(w, "input_guardrails_error", "Failed to execute input guardrails", "", http.StatusInternalServerError)
			return
		}
		
		if !result.Passed {
			log.Printf("Input guardrail failed: %s - %s", result.FailedGuardrail, result.FailureReason)
			
			// Generate API-compatible blocked response
			overrideResponse, err := h.responseBuilder.BuildResponse(r.URL.Path)
			if err != nil {
				log.Printf("Error building override response: %v", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			
			// Add guardrail context for capture middleware
			guardrailCtx := &GuardrailBlockContext{
				Blocked:          true,
				Layer:            "input",
				GuardrailName:    result.FailedGuardrail,
				GuardrailReason:  result.FailureReason,
				OriginalResponse: nil, // No original response for input blocks
				OverrideResponse: overrideResponse,
			}
			
			ctx := context.WithValue(r.Context(), "guardrail_block", guardrailCtx)
			r = r.WithContext(ctx)
			
			// Write API-compatible response to client
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK) // Return 200, not error code
			w.Write(overrideResponse)
			return
		}
		
		// Check if any input guardrail modified the request content
		for _, gr := range result.Results {
			if gr != nil && gr.Result != nil && gr.Result.ModifiedContent != nil {
				modifiedBody := *gr.Result.ModifiedContent
				log.Printf("Input guardrail modified request content (guardrail: %s)", gr.Name)
				
				// Update request body with modified content
				requestBody = modifiedBody
				r.Body = io.NopCloser(bytes.NewReader([]byte(modifiedBody)))
				break // Use first modification found
			}
		}
	}

	// Proxy the request
	resp, err := provider.ProxyRequest(r.Context(), r.URL.Path, r)
	if err != nil {
		log.Printf("Proxy request failed: %v", err)
		http.Error(w, "Proxy request failed", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Read response body for guardrails
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response body: %v", err)
		http.Error(w, "Error reading response body", http.StatusInternalServerError)
		return
	}

	// Keep original response body for client (might be compressed)
	originalResponseBody := responseBody

	// Check if response is compressed and decompress for guardrails
	contentEncoding := resp.Header.Get("Content-Encoding")
	if strings.Contains(strings.ToLower(contentEncoding), "gzip") {
		if decompressed, err := decompressGzip(responseBody); err == nil {
			responseBody = decompressed // Use decompressed for guardrails
		} else {
			log.Printf("Warning: Failed to decompress response for guardrails: %v", err)
			// Continue with original data - guardrails might fail but won't crash
		}
	}

	// Run output guardrails if enabled and executor is available (now on decompressed data)
	if h.guardrailExecutor != nil && len(responseBody) > 0 {
		result, err := h.guardrailExecutor.ExecuteOutput(r.Context(), requestID, string(responseBody))
		if err != nil {
			log.Printf("Output guardrails execution error: %v", err)
			h.returnGuardrailError(w, "output_guardrails_error", "Failed to execute output guardrails", "", http.StatusInternalServerError)
			return
		}
		
		if !result.Passed {
			log.Printf("Output guardrail failed: %s - %s", result.FailedGuardrail, result.FailureReason)
			
			// Generate API-compatible blocked response
			overrideResponse, err := h.responseBuilder.BuildResponse(r.URL.Path)
			if err != nil {
				log.Printf("Error building override response: %v", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			
			// Re-run guardrails with response data for metrics collection
			_, metricsErr := h.guardrailExecutor.ExecuteOutputWithResponses(
				r.Context(), requestID, string(responseBody), 
				originalResponseBody, overrideResponse)
			if metricsErr != nil {
				log.Printf("Error executing guardrails with response data: %v", metricsErr)
			}
			
			// Add guardrail context for capture middleware
			guardrailCtx := &GuardrailBlockContext{
				Blocked:          true,
				Layer:            "output",
				GuardrailName:    result.FailedGuardrail,
				GuardrailReason:  result.FailureReason,
				OriginalResponse: originalResponseBody, // Store original AI response
				OverrideResponse: overrideResponse,
			}
			
			ctx := context.WithValue(r.Context(), "guardrail_block", guardrailCtx)
			r = r.WithContext(ctx)
			
			// Override the response that will be written to client
			originalResponseBody = overrideResponse
			
			// Copy response headers but update content length
			corsHeaders := map[string]bool{
				"Access-Control-Allow-Origin":      true,
				"Access-Control-Allow-Methods":     true,
				"Access-Control-Allow-Headers":     true,
				"Access-Control-Max-Age":           true,
				"Access-Control-Allow-Credentials": true,
				"Access-Control-Expose-Headers":    true,
			}
			
			for key, values := range resp.Header {
				for _, value := range values {
					if corsHeaders[key] {
						w.Header().Set(key, value)
					} else {
						w.Header().Add(key, value)
					}
				}
			}
			
			// Update content length for new response
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(overrideResponse)))
			w.Header().Set("Content-Type", "application/json")
			
			// Set response status code - use 200 for blocked content
			w.WriteHeader(http.StatusOK)
			
			// Write override response to client
			if _, err := w.Write(overrideResponse); err != nil {
				log.Printf("Error writing override response: %v", err)
			}
			return
		}
	}

	// Copy response headers
	corsHeaders := map[string]bool{
		"Access-Control-Allow-Origin":      true,
		"Access-Control-Allow-Methods":     true,
		"Access-Control-Allow-Headers":     true,
		"Access-Control-Max-Age":           true,
		"Access-Control-Allow-Credentials": true,
		"Access-Control-Expose-Headers":    true,
	}
	
	for key, values := range resp.Header {
		for _, value := range values {
			// Use Set() for CORS headers to overwrite (prevent duplicates)
			// Use Add() for other headers to preserve multiple values
			if corsHeaders[key] {
				w.Header().Set(key, value)
			} else {
				w.Header().Add(key, value)
			}
		}
	}

	// Set response status code
	w.WriteHeader(resp.StatusCode)

	// Write original response body (compressed if it was compressed)
	if _, err := w.Write(originalResponseBody); err != nil {
		log.Printf("Error writing response body: %v", err)
	}
}

// isMethodAllowed checks if the HTTP method is allowed for the endpoint
func (h *ProxyHandler) isMethodAllowed(endpoint, method string, provider providers.Provider) bool {
	// This is a simplified check - in a real implementation, you'd want to
	// check the endpoint configuration from the provider
	// For now, we'll allow all methods that make sense for AI APIs
	allowedMethods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}
	
	method = strings.ToUpper(method)
	for _, allowed := range allowedMethods {
		if method == allowed {
			return true
		}
	}
	return false
}

// GetRegisteredEndpoints returns all registered endpoints
func (h *ProxyHandler) GetRegisteredEndpoints() []string {
	endpoints := make([]string, 0, len(h.routes))
	for endpoint := range h.routes {
		endpoints = append(endpoints, endpoint)
	}
	return endpoints
}

// getRequestIDFromContext extracts request ID from context
// This should be set by the capture middleware
func (h *ProxyHandler) getRequestIDFromContext(ctx context.Context) uuid.UUID {
	// Try to get request ID from context
	if requestID, ok := ctx.Value("request_id").(uuid.UUID); ok {
		return requestID
	}
	
	// If not found, generate a new one
	// This shouldn't normally happen if capture middleware is working
	return uuid.New()
}

// returnGuardrailError returns a standardized error response for guardrail violations
func (h *ProxyHandler) returnGuardrailError(w http.ResponseWriter, errorType, message, guardrailName string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	errorResponse := map[string]interface{}{
		"error":   errorType,
		"message": message,
	}
	
	if guardrailName != "" {
		errorResponse["guardrail"] = guardrailName
	}
	
	// Add additional context
	errorResponse["status"] = "blocked"
	errorResponse["timestamp"] = "2024-01-01T00:00:00Z" // This could be actual timestamp
	
	if err := json.NewEncoder(w).Encode(errorResponse); err != nil {
		log.Printf("Error encoding guardrail error response: %v", err)
	}
}

// decompressGzip decompresses gzip-compressed data for guardrails processing
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