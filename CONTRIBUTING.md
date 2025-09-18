# Contributing to Flash Gateway

Thank you for your interest in contributing to Flash Gateway! This document provides guidelines and information for contributors.

## 🤝 Code of Conduct

By participating in this project, you agree to abide by our [Code of Conduct](CODE_OF_CONDUCT.md). Please read it before contributing.

## 🚀 Getting Started

### Prerequisites

- Go 1.20 or higher
- Docker and Docker Compose (for local development)
- Git

### Development Setup

1. **Fork and clone the repository**
   ```bash
   git clone https://github.com/YOUR_USERNAME/flash-gateway.git
   cd flash-gateway
   ```

2. **Set up your development environment**
   ```bash
   # Copy example configuration
   cp configs/providers.example.yaml configs/providers.yaml

   # Edit the configuration with your API keys
   # Add your OpenAI API key to the environment
   export OPENAI_API_KEY=your_api_key_here
   ```

3. **Start the development environment**
   ```bash
   # Start PostgreSQL database
   docker-compose up -d postgres

   # Build and run the gateway
   go build -o gateway cmd/server/main.go
   ./gateway -config configs/providers.yaml
   ```

4. **Verify the setup**
   ```bash
   curl http://localhost:8080/health
   ```

## 📝 How to Contribute

### Reporting Issues

Before creating a new issue, please:

1. **Search existing issues** to avoid duplicates
2. **Use the issue templates** provided
3. **Provide detailed information** including:
   - Go version
   - Operating system
   - Steps to reproduce
   - Expected vs actual behavior
   - Relevant logs or error messages

### Submitting Pull Requests

1. **Create a feature branch**
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Make your changes**
   - Follow our coding standards (see below)
   - Add tests for new functionality
   - Update documentation as needed
   - Ensure all tests pass

3. **Commit your changes**
   ```bash
   git add .
   git commit -m "feat: add new provider support for Anthropic"
   ```

4. **Push and create a PR**
   ```bash
   git push origin feature/your-feature-name
   ```
   Then create a pull request using our PR template.

### Commit Message Guidelines

We follow the [Conventional Commits](https://www.conventionalcommits.org/) specification:

- `feat:` New features
- `fix:` Bug fixes
- `docs:` Documentation changes
- `test:` Adding or updating tests
- `refactor:` Code refactoring
- `chore:` Maintenance tasks

Examples:
```
feat: add support for Anthropic Claude API
fix: handle connection timeout gracefully
docs: update Docker setup instructions
test: add integration tests for guardrails
```

## 🧪 Testing

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...
```

### Writing Tests

- Add unit tests for new functions and methods
- Add integration tests for new providers or major features
- Ensure tests are isolated and don't depend on external services
- Use meaningful test names that describe what is being tested

### Test Structure

```go
func TestProviderName_Method(t *testing.T) {
    // Arrange
    provider := setupTestProvider()

    // Act
    result := provider.Method(input)

    // Assert
    if result != expected {
        t.Errorf("Expected %v, got %v", expected, result)
    }
}
```

## 📚 Adding New Features

### Adding a New AI Provider

1. **Create provider directory**
   ```
   internal/providers/newprovider/
   ├── provider.go
   ├── provider_test.go
   └── types.go
   ```

2. **Implement the Provider interface**
   ```go
   type Provider interface {
       GetName() string
       ProxyRequest(req *http.Request) (*http.Response, error)
       SupportedEndpoints() []string
       TransformRequest(req *http.Request) error
   }
   ```

3. **Register the provider** in `internal/router/router.go`

4. **Add configuration** in `configs/providers.yaml`

5. **Add tests** and documentation

### Adding New Guardrails

1. **Implement the Guardrail interface**
   ```go
   type Guardrail interface {
       Name() string
       Priority() int
       Validate(ctx context.Context, request GuardrailRequest) (*Result, error)
   }
   ```

2. **Register the guardrail** in `cmd/server/main.go`

3. **Add configuration options** in the config package

4. **Add comprehensive tests**

## 🎨 Coding Standards

### Go Code Style

- Follow [Effective Go](https://golang.org/doc/effective_go.html) guidelines
- Use `gofmt` to format your code
- Use meaningful variable and function names
- Add comments for exported functions and types
- Keep functions small and focused

### Code Organization

```
internal/
├── config/          # Configuration management
├── providers/       # Provider implementations
│   ├── openai/     # OpenAI provider
│   └── common/     # Shared provider utilities
├── guardrails/     # Guardrail implementations
├── handlers/       # HTTP request handlers
├── middleware/     # HTTP middleware
├── storage/        # Database and logging
└── router/         # Request routing
```

### Error Handling

- Use wrapped errors for better context: `fmt.Errorf("operation failed: %w", err)`
- Log errors at appropriate levels
- Return meaningful error messages to clients
- Handle all error cases explicitly

### Configuration

- Use environment variables for sensitive data
- Provide reasonable defaults
- Validate configuration on startup
- Document all configuration options

## 🔍 Code Review Process

All submissions require review before merging:

1. **Automated checks** must pass (tests, linting)
2. **At least one maintainer** must approve
3. **Address feedback** promptly and respectfully
4. **Squash commits** before merging if requested

### Review Criteria

- **Functionality**: Does the code work as intended?
- **Tests**: Are there adequate tests with good coverage?
- **Documentation**: Is the code well-documented?
- **Performance**: Are there any performance concerns?
- **Security**: Does the code introduce security vulnerabilities?
- **Consistency**: Does the code follow project conventions?

## 📋 Project Structure

Understanding the project structure helps with contributions:

- `cmd/server/main.go` - Application entry point
- `internal/` - Private application code
- `configs/` - Configuration files
- `migrations/` - Database migration files
- `docker-compose.yml` - Local development setup
- `Dockerfile` - Container build instructions

## 🆘 Getting Help

- **Documentation**: Check the README and docs/ directory
- **Issues**: Browse existing issues for similar problems
- **Discussions**: Use GitHub Discussions for questions
- **Community**: Join our community chat (if available)

## 📄 License

By contributing to Flash Gateway, you agree that your contributions will be licensed under the [MIT License](LICENSE).

## 🙏 Recognition

Contributors will be recognized in our [CHANGELOG.md](CHANGELOG.md) and we appreciate all forms of contribution, including:

- Code contributions
- Bug reports
- Feature requests
- Documentation improvements
- Community support
- Testing and feedback

Thank you for contributing to Flash Gateway! 🚀