package examples

import (
	"context"

	"github.com/NamanArora/flash-gateway/internal/guardrails"
)

// OutputExampleGuardrail demonstrates how to implement an output guardrail
type OutputExampleGuardrail struct {
	name     string
	priority int
	config   map[string]interface{}
}

// NewOutputExampleGuardrail creates a new output example guardrail
func NewOutputExampleGuardrail(name string, priority int, config map[string]interface{}) *OutputExampleGuardrail {
	return &OutputExampleGuardrail{
		name:     name,
		priority: priority,
		config:   config,
	}
}

// Name returns the guardrail's name
func (g *OutputExampleGuardrail) Name() string {
	return g.name
}

// Priority returns the guardrail's priority (lower = higher priority)
func (g *OutputExampleGuardrail) Priority() int {
	return g.priority
}

// Check validates the output content from LLM
// This is an example implementation that always passes
// Real implementations would validate LLM output against specific rules
func (g *OutputExampleGuardrail) Check(ctx context.Context, content string) (*guardrails.Result, error) {
	// Check if context is cancelled
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	
	// Example logic - in real implementation you would:
	// 1. Parse the LLM response (JSON, text, etc.)
	// 2. Check for harmful content, inappropriate responses
	// 3. Validate response format/structure
	// 4. Calculate confidence scores
	// 5. Return appropriate result
	
	return &guardrails.Result{
		Passed: true,
		Reason: "Example output guardrail check passed",
		Metadata: map[string]interface{}{
			"response_length":  len(content),
			"guardrail_type":   "output_example",
			"config_loaded":    len(g.config) > 0,
		},
	}, nil
}