package middleware

import (
	"log"
	"net/http"
	"time"
)

// Middleware Execution Order:
// When multiple middlewares are applied, they form nested layers around the final handler.
// The first middleware in the chain becomes the outermost layer and runs first.
//
// Example chain: [Recovery, Logger, ContentType, Capture]
// Results in: Recovery(Logger(ContentType(Capture(handler))))
//
// Request flow:
//   1. Recovery middleware (catches panics)
//   2. Logger middleware (logs request start)  
//   3. ContentType middleware (sets headers)
//   4. Capture middleware (captures request data)
//   5. Final handler (processes request)
//   6. Capture middleware (captures response data)
//   7. ContentType middleware (finishes)
//   8. Logger middleware (logs completion)
//   9. Recovery middleware (finishes)

// Logger middleware logs HTTP requests
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// Create a response writer wrapper to capture status code
		wrapper := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}
		
		// Process request
		next.ServeHTTP(wrapper, r)
		
		// Log the request
		duration := time.Since(start)
		log.Printf("%s %s %d %v - %s", 
			r.Method, 
			r.URL.Path, 
			wrapper.statusCode, 
			duration,
			r.RemoteAddr,
		)
	})
}

// CORS middleware handles Cross-Origin Resource Sharing
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		w.Header().Set("Access-Control-Max-Age", "86400") // 24 hours
		
		// Handle preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

// Recovery middleware recovers from panics
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("Panic recovered: %v", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		
		next.ServeHTTP(w, r)
	})
}

// ContentType middleware ensures proper content type handling
func ContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// For API requests, ensure we accept JSON
		if r.Header.Get("Content-Type") == "" && (r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH") {
			r.Header.Set("Content-Type", "application/json")
		}
		
		next.ServeHTTP(w, r)
	})
}

// ApplyChain wraps a handler with multiple middleware functions in order.
// Middleware execution order: first middleware in the slice runs first (outermost).
// 
// Example: ApplyChain(handler, [Recovery, Logger, CORS])
// Results in: Recovery(Logger(CORS(handler)))
// Request flow: Recovery -> Logger -> CORS -> handler
func ApplyChain(handler http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	// Start with the base handler
	result := handler
	
	// Wrap with each middleware in reverse order
	// (so first middleware in slice becomes outermost)
	for i := len(middlewares) - 1; i >= 0; i-- {
		result = middlewares[i](result)
	}
	
	return result
}

// Chain is kept for backward compatibility but deprecated.
// Use ApplyChain instead for better readability.
func Chain(middlewares ...func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(final http.Handler) http.Handler {
		return ApplyChain(final, middlewares...)
	}
}

// responseWriter wraps http.ResponseWriter to capture status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captures the status code
func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}