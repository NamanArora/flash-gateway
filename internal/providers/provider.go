package providers

import (
	"context"
	"net/http"
)

// Provider defines the interface that all AI providers must implement
type Provider interface {
	// GetName returns the provider's name (e.g., "openai", "anthropic")
	GetName() string
	
	// GetBaseURL returns the base URL for the provider's API
	GetBaseURL() string
	
	// SupportedEndpoints returns a list of endpoints this provider supports
	SupportedEndpoints() []string
	
	// ProxyRequest handles the actual proxying of requests to the provider
	ProxyRequest(ctx context.Context, endpoint string, req *http.Request) (*http.Response, error)
	
	// TransformRequest allows provider-specific request transformations
	TransformRequest(endpoint string, req *http.Request) error
	
	// TransformResponse allows provider-specific response transformations  
	TransformResponse(endpoint string, resp *http.Response) error
}