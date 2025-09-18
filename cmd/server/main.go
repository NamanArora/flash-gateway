package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/NamanArora/flash-gateway/internal/config"
	"github.com/NamanArora/flash-gateway/internal/guardrails"
	"github.com/NamanArora/flash-gateway/internal/guardrails/examples"
	"github.com/NamanArora/flash-gateway/internal/guardrails/openai"
	"github.com/NamanArora/flash-gateway/internal/router"
	"github.com/NamanArora/flash-gateway/internal/storage"
)

func main() {
	// Parse command line flags
	var configPath string
	flag.StringVar(&configPath, "config", "configs/providers.yaml", "Path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load config file (%v)", err)
	}

	// Initialize storage backend
	var storageBackend storage.StorageBackend
	if cfg.Logging.Enabled {
		storageBackend, err = setupStorage(cfg)
		if err != nil {
			if cfg.Logging.SkipOnError {
				log.Printf("Warning: Failed to setup storage, logging disabled: %v", err)
			} else {
				log.Fatalf("Failed to setup storage: %v", err)
			}
		} else {
			log.Println("✅ Storage backend initialized successfully")
		}
	}

	// Initialize async log writer
	var logWriter *storage.AsyncLogWriter
	if storageBackend != nil {
		flushInterval, err := time.ParseDuration(cfg.Logging.FlushInterval)
		if err != nil {
			log.Printf("Invalid flush interval, using default 1s: %v", err)
			flushInterval = time.Second
		}

		logWriter = storage.NewAsyncLogWriter(storage.AsyncLogWriterConfig{
			Backend:       storageBackend,
			BufferSize:    cfg.Logging.BufferSize,
			BatchSize:     cfg.Logging.BatchSize,
			FlushInterval: flushInterval,
			Workers:       cfg.Logging.Workers,
			Enabled:       cfg.Logging.Enabled,
			SkipOnError:   cfg.Logging.SkipOnError,
		})
		log.Printf("✅ Async log writer initialized with %d workers", cfg.Logging.Workers)
	}

	// Initialize guardrails system
	var guardrailExecutor *guardrails.Executor
	if cfg.Guardrails.Enabled {
		guardrailExecutor, err = setupGuardrails(cfg, storageBackend)
		if err != nil {
			log.Printf("Warning: Failed to setup guardrails: %v", err)
		} else {
			inputCount := len(cfg.Guardrails.InputGuardrails)
			outputCount := len(cfg.Guardrails.OutputGuardrails)
			log.Printf("✅ Guardrails system initialized (%d input, %d output)", inputCount, outputCount)
		}
	}

	// Initialize router with logging
	r := router.New(cfg, logWriter)
	if err := r.Initialize(); err != nil {
		log.Fatal("Failed to initialize router:", err)
	}
	
	// Set guardrail executor if available
	if guardrailExecutor != nil {
		r.SetGuardrailExecutor(guardrailExecutor)
	}

	// Create HTTP server
	server := &http.Server{
		Addr:         cfg.Server.Port,
		Handler:      r.Handler(),
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(cfg.Server.IdleTimeout) * time.Second,
	}

	// Start server in a goroutine
	go func() {
		fmt.Printf("🚀 Flash Gateway server starting on port %s\n", cfg.Server.Port)
		fmt.Println("📋 Available endpoints:")
		fmt.Println("   GET  /health - Health check")
		fmt.Println("   GET  /status - Server status")
		
		// Show logging status
		if cfg.Logging.Enabled && logWriter != nil {
			fmt.Println("   GET  /metrics - Logging metrics")
		}
		
		for _, provider := range cfg.Providers {
			fmt.Printf("   Provider: %s\n", provider.Name)
			for _, endpoint := range provider.Endpoints {
				for _, method := range endpoint.Methods {
					fmt.Printf("   %s %s - %s API\n", method, endpoint.Path, provider.Name)
				}
			}
		}
		
		fmt.Println("\n🔄 Proxying requests to configured AI providers...")
		if cfg.Logging.Enabled && logWriter != nil {
			fmt.Printf("📝 Request logging enabled (buffer: %d, workers: %d)\n", 
				cfg.Logging.BufferSize, cfg.Logging.Workers)
		} else {
			fmt.Println("📝 Request logging disabled")
		}

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("Server failed to start:", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("\n🛑 Shutting down server...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown HTTP server
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Error during server shutdown: %v", err)
	}

	// Shutdown logging system
	if logWriter != nil {
		fmt.Println("🔄 Shutting down logging system...")
		if err := logWriter.Close(); err != nil {
			log.Printf("Error closing log writer: %v", err)
		}
	}

	fmt.Println("✅ Server shutdown complete")
}

// setupStorage initializes the storage backend based on configuration
func setupStorage(cfg *config.Config) (storage.StorageBackend, error) {
	switch cfg.Storage.Type {
	case "postgres":
		return setupPostgreSQL(cfg)
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", cfg.Storage.Type)
	}
}

// setupPostgreSQL initializes PostgreSQL storage backend
func setupPostgreSQL(cfg *config.Config) (storage.StorageBackend, error) {
	pgCfg := cfg.Storage.Postgres
	
	// Build connection URL
	var connectionURL string
	if pgCfg.URL != "" && !strings.Contains(pgCfg.URL, "${") {
		connectionURL = pgCfg.URL
	} else if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		connectionURL = dbURL
	} else {
		// Build URL from individual components
		sslMode := pgCfg.SSLMode
		if sslMode == "" {
			sslMode = "disable"
		}
		connectionURL = fmt.Sprintf(
			"postgres://%s:%s@%s:%d/%s?sslmode=%s",
			pgCfg.Username,
			pgCfg.Password,
			pgCfg.Host,
			pgCfg.Port,
			pgCfg.Database,
			sslMode,
		)
	}

	log.Printf("Connecting to PostgreSQL database...")
	
	// Create storage backend
	return storage.NewPostgreSQLStorage(storage.PostgreSQLConfig{
		ConnectionURL:   connectionURL,
		MaxConnections:  pgCfg.MaxConnections,
		MaxIdleConns:    pgCfg.MaxIdleConns,
		ConnMaxLifetime: time.Duration(pgCfg.ConnMaxLifetime) * time.Minute,
	})
}

// exampleGuardrailFactory creates example guardrails
func exampleGuardrailFactory(name string, priority int, config map[string]interface{}) (guardrails.Guardrail, error) {
	switch name {
	case "input_example":
		return examples.NewInputExampleGuardrail(name, priority, config), nil
	case "output_example":
		return examples.NewOutputExampleGuardrail(name, priority, config), nil
	default:
		return nil, fmt.Errorf("unknown example guardrail: %s", name)
	}
}

// openaiGuardrailFactory creates OpenAI-based guardrails
func openaiGuardrailFactory(name string, priority int, config map[string]interface{}) (guardrails.Guardrail, error) {
	return openai.NewModerationGuardrail(name, priority, config), nil
}

// setupGuardrails initializes the guardrails system
func setupGuardrails(cfg *config.Config, storageBackend storage.StorageBackend) (*guardrails.Executor, error) {
	if !cfg.Guardrails.Enabled {
		return nil, fmt.Errorf("guardrails not enabled")
	}

	// Register example guardrails factory
	guardrails.Register("example", exampleGuardrailFactory)
	
	// Register OpenAI guardrails factory
	guardrails.Register("openai_moderation", openaiGuardrailFactory)
	
	// Parse timeout
	timeout, err := time.ParseDuration(cfg.Guardrails.Timeout)
	if err != nil {
		timeout = 5 * time.Second // Default timeout
		log.Printf("Invalid guardrails timeout, using default 5s: %v", err)
	}

	// Load input guardrails
	inputGuardrails, err := guardrails.LoadAll(cfg.Guardrails.InputGuardrails)
	if err != nil {
		log.Printf("Warning: Some input guardrails failed to load: %v", err)
	}

	// Load output guardrails
	outputGuardrails, err := guardrails.LoadAll(cfg.Guardrails.OutputGuardrails)
	if err != nil {
		log.Printf("Warning: Some output guardrails failed to load: %v", err)
	}

	// Create metrics writer if storage is available
	var metricsWriter *guardrails.MetricsWriter
	if storageBackend != nil {
		if pgStorage, ok := storageBackend.(*storage.PostgreSQLStorage); ok && pgStorage != nil {
			metricsWriter = guardrails.NewMetricsWriter(guardrails.MetricsWriterConfig{
				DB:         pgStorage.GetDB(), // We need to add this method to expose the DB
				BufferSize: cfg.Guardrails.MetricsBufferSize,
				BatchSize:  cfg.Guardrails.MetricsBatchSize,
				Workers:    cfg.Guardrails.MetricsWorkers,
			})
		}
	}

	// Create executor
	executor := guardrails.NewExecutor(guardrails.ExecutorConfig{
		InputGuardrails:  inputGuardrails,
		OutputGuardrails: outputGuardrails,
		MetricsWriter:    metricsWriter,
		Timeout:          timeout,
	})

	return executor, nil
}