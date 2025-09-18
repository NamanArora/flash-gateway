package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config holds the entire application configuration
type Config struct {
	Server     ServerConfig     `yaml:"server"`
	Storage    StorageConfig    `yaml:"storage"`
	Logging    LoggingConfig    `yaml:"logging"`
	Guardrails GuardrailsConfig `yaml:"guardrails"`
	Providers  []ProviderConfig `yaml:"providers"`
}

// ProviderConfig holds configuration for a provider
type ProviderConfig struct {
	Name      string           `yaml:"name"`
	BaseURL   string           `yaml:"base_url"`
	Endpoints []EndpointConfig `yaml:"endpoints"`
}

// EndpointConfig defines how an endpoint should be handled
type EndpointConfig struct {
	Path    string            `yaml:"path"`
	Methods []string          `yaml:"methods"`
	Headers map[string]string `yaml:"headers,omitempty"`
	Timeout int               `yaml:"timeout,omitempty"` // seconds
}

// ServerConfig holds server-specific configuration
type ServerConfig struct {
	Port         string `yaml:"port"`
	ReadTimeout  int    `yaml:"read_timeout"`   // seconds
	WriteTimeout int    `yaml:"write_timeout"`  // seconds
	IdleTimeout  int    `yaml:"idle_timeout"`   // seconds
}

// StorageConfig holds database configuration
type StorageConfig struct {
	Type       string           `yaml:"type"`       // "postgres", "memory"
	Postgres   PostgresConfig   `yaml:"postgres"`
}

// PostgresConfig holds PostgreSQL-specific configuration
type PostgresConfig struct {
	URL             string `yaml:"url"`
	Host            string `yaml:"host"`
	Port            int    `yaml:"port"`
	Database        string `yaml:"database"`
	Username        string `yaml:"username"`
	Password        string `yaml:"password"`
	SSLMode         string `yaml:"ssl_mode"`
	MaxConnections  int    `yaml:"max_connections"`
	MaxIdleConns    int    `yaml:"max_idle_conns"`
	ConnMaxLifetime int    `yaml:"conn_max_lifetime"` // minutes
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Enabled         bool   `yaml:"enabled"`
	BufferSize      int    `yaml:"buffer_size"`
	BatchSize       int    `yaml:"batch_size"`
	FlushInterval   string `yaml:"flush_interval"` // duration string like "1s"
	Workers         int    `yaml:"workers"`
	MaxBodySize     int    `yaml:"max_body_size"`     // bytes
	SkipHealthCheck bool   `yaml:"skip_health_check"`
	SkipOnError     bool   `yaml:"skip_on_error"`
}

// GuardrailsConfig holds guardrails configuration
type GuardrailsConfig struct {
	Enabled          bool                     `yaml:"enabled"`
	Timeout          string                   `yaml:"timeout"` // duration string like "5s"
	MetricsBufferSize int                    `yaml:"metrics_buffer_size"`
	MetricsBatchSize  int                    `yaml:"metrics_batch_size"`
	MetricsWorkers    int                    `yaml:"metrics_workers"`
	InputGuardrails   []GuardrailConfig       `yaml:"input_guardrails"`
	OutputGuardrails  []GuardrailConfig       `yaml:"output_guardrails"`
}

// GuardrailConfig holds configuration for a single guardrail
type GuardrailConfig struct {
	Name     string                 `yaml:"name"`
	Type     string                 `yaml:"type"` // "example" or custom type
	Enabled  bool                   `yaml:"enabled"`
	Priority int                    `yaml:"priority"`
	Config   map[string]interface{} `yaml:"config"`
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(configPath string) (*Config, error) {
	// Set defaults
	config := &Config{
		Server: ServerConfig{
			Port:         ":8080",
			ReadTimeout:  30,
			WriteTimeout: 30,
			IdleTimeout:  120,
		},
		Storage: StorageConfig{
			Type: "postgres",
			Postgres: PostgresConfig{
				URL:             os.Getenv("DATABASE_URL"),
				Host:            "localhost",
				Port:            5432,
				Database:        "gateway",
				Username:        "gateway",
				Password:        "gateway_pass",
				SSLMode:         "disable",
				MaxConnections:  25,
				MaxIdleConns:    5,
				ConnMaxLifetime: 60, // minutes
			},
		},
		Logging: LoggingConfig{
			Enabled:         true,
			BufferSize:      1000,
			BatchSize:       10,
			FlushInterval:   "1s",
			Workers:         3,
			MaxBodySize:     64 * 1024, // 64KB
			SkipHealthCheck: true,
			SkipOnError:     true,
		},
		Guardrails: GuardrailsConfig{
			Enabled:          false, // Disabled by default
			Timeout:          "5s",
			MetricsBufferSize: 1000,
			MetricsBatchSize:  10,
			MetricsWorkers:    2,
			InputGuardrails:   []GuardrailConfig{},
			OutputGuardrails:  []GuardrailConfig{},
		},
	}

	// Read config file if it exists
	if configPath != "" {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}

		if err := yaml.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}
	}

	return config, nil
}

// GetProviderConfig returns the configuration for a specific provider
func (c *Config) GetProviderConfig(providerName string) (*ProviderConfig, error) {
	for _, provider := range c.Providers {
		if provider.Name == providerName {
			return &provider, nil
		}
	}
	return nil, fmt.Errorf("provider %s not found in configuration", providerName)
}