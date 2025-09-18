package examples

import (
	"context"

	"github.com/NamanArora/flash-gateway/internal/guardrails"
)

// InputExampleGuardrail demonstrates how to implement an input guardrail
type InputExampleGuardrail struct {
	name     string
	priority int
	config   map[string]interface{}
}

// NewInputExampleGuardrail creates a new input example guardrail
func NewInputExampleGuardrail(name string, priority int, config map[string]interface{}) *InputExampleGuardrail {
	return &InputExampleGuardrail{
		name:     name,
		priority: priority,
		config:   config,
	}
}

// Name returns the guardrail's name
func (g *InputExampleGuardrail) Name() string {
	return g.name
}

// Priority returns the guardrail's priority (lower = higher priority)
func (g *InputExampleGuardrail) Priority() int {
	return g.priority
}

// Check validates the input content
// This is an example implementation that always passes
// Real implementations would check content against specific rules
func (g *InputExampleGuardrail) Check(ctx context.Context, content string) (*guardrails.Result, error) {
	// Check if context is cancelled
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	
	// Example logic - in real implementation you would:
	// 1. Parse the content (JSON, text, etc.)
	// 2. Apply your validation rules
	// 3. Calculate scores if applicable
	// 4. Return appropriate result
	
	score := 1.0 // Perfect score for example
	
	return &guardrails.Result{
		Passed: true,
		Score:  &score,
		Reason: "Example input guardrail check passed",
		Metadata: map[string]interface{}{
			"content_length": len(content),
			"guardrail_type": "input_example",
			"config_loaded":  len(g.config) > 0,
		},
	}, nil
}