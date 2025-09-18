package guardrails

import (
	"fmt"
	"sync"

	"github.com/NamanArora/flash-gateway/internal/config"
)

var (
	// Global registry for guardrail factories
	registry = make(map[string]GuardrailFactory)
	mu       sync.RWMutex
)

// Register allows custom guardrails to be registered
// This should be called during application initialization
func Register(name string, factory GuardrailFactory) {
	mu.Lock()
	defer mu.Unlock()
	
	if factory == nil {
		panic(fmt.Sprintf("guardrail factory for %s is nil", name))
	}
	
	registry[name] = factory
}

// Load creates a guardrail from configuration
func Load(config config.GuardrailConfig) (Guardrail, error) {
	if !config.Enabled {
		return nil, fmt.Errorf("guardrail %s is disabled", config.Name)
	}

	// Handle built-in example guardrails
	if config.Type == "example" {
		return loadExampleGuardrail(config)
	}
	
	// Look for custom guardrail in registry
	mu.RLock()
	factory, exists := registry[config.Type]
	mu.RUnlock()
	
	if !exists {
		return nil, fmt.Errorf("unknown guardrail type: %s", config.Type)
	}
	
	return factory(config.Name, config.Priority, config.Config)
}

// LoadAll creates all guardrails from a slice of configurations
func LoadAll(configs []config.GuardrailConfig) ([]Guardrail, error) {
	var guardrails []Guardrail
	var errors []string
	
	for _, cfg := range configs {
		if !cfg.Enabled {
			continue
		}
		
		guardrail, err := Load(cfg)
		if err != nil {
			errors = append(errors, fmt.Sprintf("failed to load guardrail %s: %v", cfg.Name, err))
			continue
		}
		
		guardrails = append(guardrails, guardrail)
	}
	
	// Return error if any guardrails failed to load
	if len(errors) > 0 {
		return guardrails, fmt.Errorf("errors loading guardrails: %v", errors)
	}
	
	return guardrails, nil
}

// loadExampleGuardrail loads built-in example guardrails
func loadExampleGuardrail(config config.GuardrailConfig) (Guardrail, error) {
	// Look for example guardrails in the registry
	mu.RLock()
	factory, exists := registry["example"]
	mu.RUnlock()
	
	if !exists {
		return nil, fmt.Errorf("example guardrail factory not registered")
	}
	
	return factory(config.Name, config.Priority, config.Config)
}

// GetRegistered returns all registered guardrail types
func GetRegistered() []string {
	mu.RLock()
	defer mu.RUnlock()
	
	types := make([]string, 0, len(registry))
	
	// Add example types
	types = append(types, "example")
	
	// Add custom types
	for typeName := range registry {
		types = append(types, typeName)
	}
	
	return types
}

// IsRegistered checks if a guardrail type is registered
func IsRegistered(guardrailType string) bool {
	if guardrailType == "example" {
		return true
	}
	
	mu.RLock()
	defer mu.RUnlock()
	
	_, exists := registry[guardrailType]
	return exists
}

// Unregister removes a guardrail type from the registry
// This is mainly useful for testing
func Unregister(name string) {
	mu.Lock()
	defer mu.Unlock()
	
	delete(registry, name)
}

// Clear removes all registered guardrail types
// This is mainly useful for testing
func Clear() {
	mu.Lock()
	defer mu.Unlock()
	
	registry = make(map[string]GuardrailFactory)
}