# Flash Gateway

[![Go Version](https://img.shields.io/badge/Go-1.20+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Docker](https://img.shields.io/badge/Docker-supported-blue?style=flat&logo=docker)](https://www.docker.com/)

A high-performance, production-ready proxy server for AI providers like OpenAI, built with Go. Features request logging, guardrails system, and extensible architecture for multiple AI providers.

## Features

- **Multi-provider Support**: Currently supports OpenAI, easily extensible to Anthropic, Google, and others
- **Advanced Guardrails System**:
  - **Parallel Execution**: Same priority guardrails run concurrently for minimal latency impact
  - **Priority-based Processing**: Sequential execution across different priority levels
  - **First-fail Mechanism**: Immediate halt on any guardrail failure
  - **Highly Extensible**: Add custom guardrails by simply implementing the `Guardrail` interface
- **Request Logging**: Asynchronous PostgreSQL logging with comprehensive metrics
- **Automated Database Management**: Zero-touch schema migrations run automatically on startup
- **Configuration-driven**: Add new endpoints, providers, and guardrails via YAML
- **Ultra-Low Latency**: Parallel guardrails execution, async processing, connection pooling, and optimized middleware
- **Docker Ready**: Complete containerization with docker-compose setup
- **Observability**: Health checks, metrics, and comprehensive logging with guardrail performance tracking
- **Security**: Environment variable support, header sanitization, and configurable content filtering

## Quick Start

### Option 1: Docker (Recommended)

1. **Clone and configure**
   ```bash
   git clone https://github.com/yourusername/flash-gateway.git
   cd flash-gateway
   cp configs/providers.example.yaml configs/providers.yaml
   ```

2. **Set your OpenAI API key**
   ```bash
   export OPENAI_API_KEY=your_api_key_here
   ```

3. **Start with Docker Compose**
   ```bash
   docker-compose up -d
   ```
   *Database migrations run automatically on first startup*

4. **Verify it's running**
   ```bash
   curl http://localhost:8080/health
   # Response: {"status": "healthy"}
   ```

### Option 2: Binary Installation

1. **Build from source**
   ```bash
   git clone https://github.com/yourusername/flash-gateway.git
   cd flash-gateway
   go build -o gateway cmd/server/main.go
   ```

2. **Configure and run**
   ```bash
   cp configs/providers.example.yaml configs/providers.yaml
   export OPENAI_API_KEY=your_api_key_here
   ./gateway -config configs/providers.yaml
   ```

## How It Works

Flash Gateway acts as an intelligent proxy between your applications and AI providers. Here's how a request flows through the system:

```
Client Request
     ↓
┌─────────────────────────────────────────────────────────────┐
│                   Flash Gateway                             │
│                                                             │
│  1. Recovery Middleware     ← Catches panics               │
│           ↓                                                 │
│  2. Logger Middleware       ← Logs request info            │
│           ↓                                                 │
│  3. CORS Middleware         ← Handles CORS headers         │
│           ↓                                                 │
│  4. ContentType Middleware  ← Sets content types           │
│           ↓                                                 │
│  5. Capture Middleware      ← Captures req/resp for logs   │
│           ↓                                                 │
│  6. ProxyHandler (Router)   ← Routes to provider           │
│           ↓                                                 │
│  7. Input Guardrails        ← Parallel execution by        │
│     ┌─────────────────┐       priority groups             │
│     │ G1 │ G2 │ G3 │...│     ← Same priority = parallel   │
│     │ Priority 0 ────│     ← Lower number = higher priority│
│     └─────────────────┘                                   │
│     ┌─────────────────┐     ← First-fail mechanism        │
│     │ G4 │ G5 │...   │                                   │
│     │ Priority 1 ────│     ← Next priority group         │
│     └─────────────────┘                                   │
│           ↓                                                 │
│  8. Provider (OpenAI)       ← Proxies to AI service        │
│           ↓                                                 │
│  9. Output Guardrails       ← Same parallel execution      │
│     ┌─────────────────┐       pattern as input           │
│     │ G6 │ G7 │ G8 │...│     ← Response validation         │
│     │ Priority 0 ────│     ← Can override unsafe content  │
│     └─────────────────┘                                   │
│           ↓                                                 │
│ 10. Async Logging           ← Stores logs & metrics in     │
│                               PostgreSQL (non-blocking)   │
│                                                             │
└─────────────────────────────────────────────────────────────┘
     ↓
Client Response
```

### Middleware Pipeline Details

1. **Recovery**: Catches any panics and returns proper HTTP error responses
2. **Logger**: Logs basic request information (method, path, duration)
3. **CORS**: Adds CORS headers for cross-origin requests (configurable)
4. **ContentType**: Ensures proper content-type headers
5. **Capture**: Captures full request/response data for async logging
6. **ProxyHandler**: Routes requests and orchestrates guardrails execution
7. **Input Guardrails**:
   - **Parallel Execution**: Same priority guardrails run concurrently for low latency
   - **Priority Groups**: Different priorities run sequentially (lower number = higher priority)
   - **First-Fail**: Execution stops immediately if any guardrail fails
   - **Extensible**: Just implement the `Guardrail` interface to add new checks
8. **Provider**: Forwards request to the appropriate AI service (OpenAI, etc.)
9. **Output Guardrails**:
   - **Same Architecture**: Parallel execution with priority groups and first-fail
   - **Response Override**: Can replace unsafe AI responses with safe alternatives
   - **Metrics Tracking**: All guardrail executions are tracked with performance data
10. **Async Logging**: Stores comprehensive logs and guardrail metrics without blocking responses

## Docker Setup

### Full Stack with Docker Compose

The `docker-compose.yml` provides a complete development environment:

```yaml
# PostgreSQL database for request logging
postgres:
  image: postgres:15-alpine
  ports: ["5432:5432"]
  environment:
    POSTGRES_DB: gateway
    POSTGRES_USER: gateway
    POSTGRES_PASSWORD: gateway_pass

# Flash Gateway application
gateway:
  build: .
  ports: ["8080:8080"]
  environment:
    - DATABASE_URL=postgres://gateway:gateway_pass@postgres:5432/gateway?sslmode=disable
    - OPENAI_API_KEY=${OPENAI_API_KEY}
  depends_on:
    postgres: { condition: service_healthy }
```

### Commands

```bash
# Start all services (runs migrations automatically)
docker-compose up -d

# View logs (includes migration output)
docker-compose logs -f gateway

# Stop all services
docker-compose down

# Rebuild after code changes
docker-compose up --build -d gateway
```

### Automated Database Migrations

Flash Gateway includes a zero-configuration database migration system:

**How it works:**
- **Automatic Execution**: Migrations run automatically when the gateway container starts
- **Idempotent**: Safe to restart containers - migrations only run if needed
- **Single Schema File**: All database schema consolidated in `migrations/schema.sql`
- **Health Check Integration**: Gateway won't start if migrations fail

**What gets created:**
- `request_logs` table with indexes for request tracking
- `guardrail_metrics` table for performance monitoring
- Views for common queries (`recent_request_logs`, `guardrail_performance_summary`)
- Triggers and functions for automatic timestamp updates

**Fresh vs Existing Databases:**
```bash
# Fresh database - migrations run automatically
docker-compose up -d

# Existing database - migrations skipped if already applied
docker-compose restart gateway
```

**Manual Migration (if needed):**
```bash
# Run migrations manually
docker exec gateway-app ./migrations/run-migrations.sh

# Connect to database directly
docker exec -it gateway-postgres psql -U gateway -d gateway
```

### Production Docker Setup

For production, create a custom docker-compose override:

```yaml
# docker-compose.prod.yml
version: '3.8'
services:
  gateway:
    restart: always
    environment:
      - LOG_LEVEL=info
      - DATABASE_URL=${DATABASE_URL}
      - OPENAI_API_KEY=${OPENAI_API_KEY}

  postgres:
    restart: always
    volumes:
      - /var/lib/postgresql/data:/var/lib/postgresql/data
    environment:
      - POSTGRES_PASSWORD=${POSTGRES_PASSWORD}
```

## API Endpoints

### System Endpoints
- `GET /health` - Health check
- `GET /status` - Server status and provider info
- `GET /metrics` - Logging and performance metrics

### OpenAI Endpoints (Proxied)
All OpenAI API endpoints are supported:

- `POST /v1/chat/completions` - Chat completions
- `POST /v1/completions` - Legacy completions
- `POST /v1/embeddings` - Text embeddings
- `GET /v1/models` - Available models
- `POST /v1/audio/speech` - Text-to-speech
- `POST /v1/audio/transcriptions` - Audio transcription
- `POST /v1/images/generations` - Image generation
- `POST /v1/fine-tuning/jobs` - Fine-tuning
- And many more...

## Usage Examples

### Basic Chat Completion

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${OPENAI_API_KEY}" \
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

### With Guardrails Enabled

The gateway automatically applies configured guardrails:

```bash
# This request will be checked by OpenAI Moderation API
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${OPENAI_API_KEY}" \
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [{"role": "user", "content": "Tell me something harmful"}]
  }'
# Response: {"error": {"message": "I cannot service this request", "type": "content_policy_violation"}}
```

### Check Server Status

```bash
curl http://localhost:8080/status
```

Response:
```json
{
  "status": "running",
  "providers": {
    "openai": {
      "endpoints": ["/v1/chat/completions", "/v1/completions", "..."]
    }
  }
}
```

## Configuration

### Environment Variables

```bash
# Required
export OPENAI_API_KEY=your_api_key_here

# Optional - Database (defaults to docker-compose values)
export DATABASE_URL=postgres://user:pass@localhost:5432/gateway?sslmode=disable

# Optional - Logging
export LOG_LEVEL=info
```

### Configuration File

Copy `configs/providers.example.yaml` to `configs/providers.yaml` and customize:

```yaml
server:
  port: ":8080"
  read_timeout: 30    # seconds
  write_timeout: 30   # seconds
  idle_timeout: 120   # seconds

storage:
  type: "postgres"
  postgres:
    url: "${DATABASE_URL}"  # Uses environment variable
    max_connections: 25
    max_idle_conns: 5
    conn_max_lifetime: 60  # minutes

logging:
  enabled: true
  buffer_size: 1000        # Channel buffer size
  batch_size: 10           # Batch insert size
  flush_interval: "1s"     # Force flush interval
  workers: 3               # Number of worker goroutines
  max_body_size: 65536     # Max body size to capture (64KB)
  skip_health_check: true  # Don't log /health requests
  skip_on_error: true      # Don't block requests if logging fails

guardrails:
  enabled: true
  timeout: "5s"
  input_guardrails:
    - name: "openai_moderation"
      type: "openai_moderation"
      enabled: true
      config:
        api_key: "${OPENAI_API_KEY}"
        block_on_flag: true
        categories: ["hate", "violence", "sexual", "self-harm"]

providers:
  - name: openai
    base_url: https://api.openai.com
    endpoints:
      - path: /v1/chat/completions
        methods: ["POST"]
        timeout: 60
      # ... more endpoints
```

### Guardrails Configuration

Built-in guardrails include:

1. **OpenAI Moderation**: Uses OpenAI's moderation API to check for harmful content
2. **Example Guardrails**: Demonstration guardrails for testing

Custom guardrails can be added by implementing the `Guardrail` interface.

## Production Deployment

### System Requirements

- **CPU**: 2+ cores recommended
- **Memory**: 512MB minimum, 2GB+ recommended
- **Storage**: 10GB+ for logs (with log rotation)
- **Network**: Reliable internet connection for AI provider APIs

### Production Checklist

- [ ] Set strong PostgreSQL password
- [ ] Configure log rotation
- [ ] Set up monitoring and alerting
- [ ] Enable HTTPS with reverse proxy (nginx/Cloudflare)
- [ ] Configure firewall rules
- [ ] Set up backup strategy for database
- [ ] Monitor disk space for logs
- [ ] Configure resource limits in docker-compose

### Monitoring

Monitor these metrics:

- **Health**: `GET /health` endpoint
- **Request logs**: PostgreSQL `request_logs` table
- **Performance**: `GET /metrics` endpoint
- **Error rates**: Check application logs
- **Database**: Monitor PostgreSQL performance

## Development

### Adding New Providers

1. **Create provider implementation**:
   ```go
   // internal/providers/anthropic/provider.go
   type AnthropicProvider struct {}

   func (p *AnthropicProvider) GetName() string { return "anthropic" }
   func (p *AnthropicProvider) ProxyRequest(req *http.Request) (*http.Response, error) {
       // Implementation
   }
   // ... implement other Provider interface methods
   ```

2. **Register in router**:
   ```go
   // internal/router/router.go
   case "anthropic":
       provider = anthropic.NewAnthropicProvider(providerConfig)
   ```

3. **Add to configuration**:
   ```yaml
   providers:
     - name: anthropic
       base_url: https://api.anthropic.com
       endpoints:
         - path: /v1/messages
           methods: ["POST"]
           timeout: 60
   ```

### Testing

```bash
# Run unit tests
go test ./...

# Test with Docker
docker-compose up -d
```

## Dashboard

Flash Gateway includes a web dashboard for monitoring and testing AI gateway traffic.

### Features
- **Real-time Request Logs**: View all API requests with detailed information
- **Guardrail Metrics**: Monitor content filtering and safety measures
- **API Playground**: Test endpoints directly through the web interface
- **Response Override Tracking**: See when and how guardrails modify responses

### Quick Start

After running the Docker setup, the dashboard is automatically available:

```bash
# Start all services including dashboard
docker-compose up -d

# Access points:
# - Gateway API: http://localhost:8080
# - Dashboard UI: http://localhost:5173
# - Dashboard API: http://localhost:4000
```

### Dashboard Components

#### Request Logs Page
- Paginated view of all API requests
- Filterable by endpoint, method, status code
- Click any row to view full request/response details
- Real-time updates as new requests come in

#### Guardrail Metrics Page
- Monitor all guardrail executions
- See performance metrics (duration, pass/fail rates)
- Track response overrides when content is blocked
- Filter by guardrail name, layer (input/output), or status

#### Playground Page
- Interactive API testing interface
- Supports both `/v1/chat/completions` and `/v1/responses` endpoints
- Configurable system prompts
- Real-time conversation interface
- Request/response logging in browser console

### Development

The dashboard supports hot-reload during development:

```bash
# Dashboard runs in development mode by default
docker-compose up -d

# View dashboard logs
docker-compose logs -f dashboard

# Rebuild after changes
docker-compose up --build -d dashboard
```

### Production Deployment

For production, set the environment to optimize builds:

```bash
# Set production environment
NODE_ENV=production docker-compose up --build -d
```

### Dashboard API Endpoints

- `GET /api/health` - Health check with database status
- `GET /api/request-logs` - Paginated request logs
- `GET /api/request-logs/:id` - Individual request details
- `GET /api/guardrail-metrics` - Paginated guardrail metrics

### Architecture

The dashboard consists of:
- **Frontend**: React + TypeScript + Vite dev server (port 5173)
- **Backend**: Express.js API server (port 4000)
- **Database**: Shared PostgreSQL with gateway for real-time data

All components run in Docker containers with automatic service discovery and health checks.

## Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for details.

### Quick Start for Contributors

1. Fork the repository
2. Create a feature branch: `git checkout -b feature/amazing-feature`
3. Make your changes and add tests
4. Run tests: `go test ./...`
5. Commit: `git commit -m 'feat: add amazing feature'`
6. Push: `git push origin feature/amazing-feature`
7. Open a Pull Request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Support

- **Documentation**: Check this README and [CONTRIBUTING.md](CONTRIBUTING.md)
- **Issues**: [GitHub Issues](https://github.com/yourusername/flash-gateway/issues)
- **Security**: See our [Security Policy](SECURITY.md)

## Roadmap

- [ ] Support for Anthropic Claude API
- [ ] Support for Google Gemini API
- [ ] Enhanced rate limiting and caching
- [ ] Webhook support for async processing
- [ ] Enhanced metrics and monitoring
- [ ] Plugin system for custom middleware
- [ ] Load balancing across multiple provider instances

---

**Built by the Flash Gateway team**