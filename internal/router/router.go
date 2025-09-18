package router

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/NamanArora/flash-gateway/internal/config"
	"github.com/NamanArora/flash-gateway/internal/guardrails"
	"github.com/NamanArora/flash-gateway/internal/handlers"
	"github.com/NamanArora/flash-gateway/internal/middleware"
	"github.com/NamanArora/flash-gateway/internal/providers"
	"github.com/NamanArora/flash-gateway/internal/providers/openai"
	"github.com/NamanArora/flash-gateway/internal/storage"
)

// Router manages HTTP routing and provider registration
type Router struct {
	proxyHandler *handlers.ProxyHandler
	config       *config.Config
	logWriter    *storage.AsyncLogWriter
	capture      *middleware.CaptureMiddleware
}

// New creates a new router instance
func New(cfg *config.Config, logWriter *storage.AsyncLogWriter) *Router {
	var capture *middleware.CaptureMiddleware
	if logWriter != nil {
		capture = middleware.NewCaptureMiddleware(middleware.CaptureConfig{
			Writer:          logWriter,
			MaxBodySize:     cfg.Logging.MaxBodySize,
			SkipHealthCheck: cfg.Logging.SkipHealthCheck,
		})
	}

	return &Router{
		proxyHandler: handlers.NewProxyHandler(),
		config:       cfg,
		logWriter:    logWriter,
		capture:      capture,
	}
}

// Initialize sets up all providers and routes
func (r *Router) Initialize() error {
	// Initialize providers based on configuration
	for _, providerConfig := range r.config.Providers {
		var provider providers.Provider

		switch providerConfig.Name {
		case "openai":
			provider = openai.New(providerConfig)
		default:
			return fmt.Errorf("unsupported provider: %s", providerConfig.Name)
		}

		// Register the provider
		r.proxyHandler.RegisterProvider(provider)
	}

	return nil
}

// Handler returns the main HTTP handler with all middleware applied
func (r *Router) Handler() http.Handler {
	// Create base handler
	handler := http.Handler(r.proxyHandler)

	// Add health check endpoint
	mux := http.NewServeMux()
	mux.Handle("/", handler)
	mux.HandleFunc("/health", r.healthCheckHandler)
	mux.HandleFunc("/status", r.statusHandler)

	// Add metrics endpoint if logging is enabled
	if r.logWriter != nil {
		mux.HandleFunc("/metrics", r.metricsHandler)
	}

	// Build middleware chain - order matters!
	// First middleware listed runs first (outermost layer)
	middlewares := []func(http.Handler) http.Handler{
		middleware.Recovery,    // 1. Catches panics (outermost)
		middleware.Logger,      // 2. Logs requests
		middleware.CORS,     // 3. CORS headers (disabled)
		middleware.ContentType, // 3. Sets content type
	}

	// Add capture middleware if logging is enabled
	// This runs last (innermost) to capture final request/response data
	if r.capture != nil {
		middlewares = append(middlewares, r.capture.Capture) // 4. Captures data
	}

	// Apply middleware chain using the simplified approach
	// This wraps: Recovery(Logger(ContentType(Capture(mux))))
	return middleware.ApplyChain(mux, middlewares...)
}

// healthCheckHandler provides a simple health check endpoint
func (r *Router) healthCheckHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "healthy"}`))
}

// statusHandler provides information about registered providers and endpoints
func (r *Router) statusHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	endpoints := r.proxyHandler.GetRegisteredEndpoints()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := fmt.Sprintf(`{
	"status": "running",
	"registered_endpoints": %d,
	"providers": %d
}`, len(endpoints), len(r.config.Providers))

	w.Write([]byte(response))
}

// metricsHandler provides logging metrics
func (r *Router) metricsHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if r.logWriter == nil {
		http.Error(w, "Logging not enabled", http.StatusServiceUnavailable)
		return
	}

	metrics := r.logWriter.GetMetrics()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(metrics); err != nil {
		http.Error(w, "Failed to encode metrics", http.StatusInternalServerError)
	}
}

// SetGuardrailExecutor sets the guardrail executor for the proxy handler
func (r *Router) SetGuardrailExecutor(executor interface{}) {
	// Import guardrails package to use the executor type
	if r.proxyHandler != nil {
		if guardrailExecutor, ok := executor.(*guardrails.Executor); ok {
			r.proxyHandler.SetGuardrailExecutor(guardrailExecutor)
		}
	}
}
