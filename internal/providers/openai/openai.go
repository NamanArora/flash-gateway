package openai

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/NamanArora/flash-gateway/internal/config"
)

// Provider implements the providers.Provider interface for OpenAI
type Provider struct {
	config config.ProviderConfig
	client *http.Client
}

// New creates a new OpenAI provider instance
func New(cfg config.ProviderConfig) *Provider {
	return &Provider{
		config: cfg,
		client: &http.Client{
			Transport: &http.Transport{
				DisableCompression: true, // Don't auto-decompress gzip responses for true pass-through proxy
			},
			Timeout: 60 * time.Second, // Default timeout
		},
	}
}

// GetName returns the provider name
func (p *Provider) GetName() string {
	return p.config.Name
}

// GetBaseURL returns the OpenAI API base URL
func (p *Provider) GetBaseURL() string {
	if p.config.BaseURL != "" {
		return p.config.BaseURL
	}
	return "https://api.openai.com"
}

// SupportedEndpoints returns the list of supported OpenAI endpoints
func (p *Provider) SupportedEndpoints() []string {
	endpoints := make([]string, len(p.config.Endpoints))
	for i, endpoint := range p.config.Endpoints {
		endpoints[i] = endpoint.Path
	}
	return endpoints
}

// ProxyRequest proxies the request to OpenAI API
func (p *Provider) ProxyRequest(ctx context.Context, endpoint string, req *http.Request) (*http.Response, error) {
	// Create target URL
	targetURL := p.GetBaseURL() + endpoint
	
	// Create new request with context
	proxyReq, err := http.NewRequestWithContext(ctx, req.Method, targetURL, req.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to create proxy request: %w", err)
	}

	// Copy all headers from original request to proxy request
	for key, values := range req.Header {
		for _, value := range values {
			proxyReq.Header.Add(key, value)
		}
	}

	// TODO: Add support for Brotli (br), Zstandard (zstd), and deflate compression formats
	// Currently only gzip is supported for response decompression in logging
	// Force gzip by removing other compression formats from Accept-Encoding
	acceptEncoding := proxyReq.Header.Get("Accept-Encoding")
	if strings.Contains(acceptEncoding, "br") || strings.Contains(acceptEncoding, "zstd") || strings.Contains(acceptEncoding, "deflate") {
		// Remove unsupported compression formats: 'br' (Brotli), 'zstd' (Zstandard), 'deflate'
		acceptEncoding = strings.ReplaceAll(acceptEncoding, "br", "")
		acceptEncoding = strings.ReplaceAll(acceptEncoding, "zstd", "")
		acceptEncoding = strings.ReplaceAll(acceptEncoding, "deflate", "")
		// Clean up any double commas or leading/trailing commas
		acceptEncoding = strings.ReplaceAll(acceptEncoding, ",,", ",")
		acceptEncoding = strings.Trim(acceptEncoding, ", ")
		if acceptEncoding == "" {
			acceptEncoding = "gzip"  // Only gzip to ensure we can decompress for logging
		}
		proxyReq.Header.Set("Accept-Encoding", acceptEncoding)
	}

	// Apply request transformations
	if err := p.TransformRequest(endpoint, proxyReq); err != nil {
		return nil, fmt.Errorf("request transformation failed: %w", err)
	}

	// Make the request
	resp, err := p.client.Do(proxyReq)
	if err != nil {
		return nil, fmt.Errorf("proxy request failed: %w", err)
	}

	// Apply response transformations
	if err := p.TransformResponse(endpoint, resp); err != nil {
		resp.Body.Close()
		return nil, fmt.Errorf("response transformation failed: %w", err)
	}

	return resp, nil
}

// TransformRequest applies OpenAI-specific request transformations
func (p *Provider) TransformRequest(endpoint string, req *http.Request) error {
	// Set default content type if not present
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// Apply endpoint-specific headers from config
	endpointConfig := p.getEndpointConfig(endpoint)
	if endpointConfig != nil {
		for key, value := range endpointConfig.Headers {
			req.Header.Set(key, value)
		}
	}

	return nil
}

// TransformResponse applies OpenAI-specific response transformations
func (p *Provider) TransformResponse(endpoint string, resp *http.Response) error {
	// For now, we don't need any OpenAI-specific response transformations
	// This method is here for future extensibility
	return nil
}

// getEndpointConfig returns the configuration for a specific endpoint
func (p *Provider) getEndpointConfig(endpoint string) *config.EndpointConfig {
	for _, ep := range p.config.Endpoints {
		if ep.Path == endpoint {
			return &ep
		}
	}
	return nil
}