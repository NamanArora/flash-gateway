package guardrails

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Guardrail is the main interface for all guardrails
type Guardrail interface {
	// Name returns the guardrail's unique identifier
	Name() string
	
	// Check performs the guardrail validation
	Check(ctx context.Context, content string) (*Result, error)
	
	// Priority returns execution priority (lower = higher priority)
	// Used for: 1) Startup order in parallel execution
	//          2) Result processing order 
	//          3) Future: Circuit breaking decisions
	Priority() int
}

// Result represents a guardrail check result
type Result struct {
	Passed          bool                   `json:"passed"`
	Score           *float64              `json:"score,omitempty"`
	Reason          string                `json:"reason,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	ModifiedContent *string               `json:"modified_content,omitempty"` // Optional modified content for next guardrails
}

// Metric captures performance data for a guardrail execution
type Metric struct {
	ID                 uuid.UUID             `json:"id" db:"id"`
	RequestID          uuid.UUID             `json:"request_id" db:"request_id"`
	GuardrailName      string                `json:"guardrail_name" db:"guardrail_name"`
	Layer              string                `json:"layer" db:"layer"` // "input" or "output"
	Priority           int                   `json:"priority" db:"priority"`
	StartTime          time.Time             `json:"start_time" db:"start_time"`
	EndTime            time.Time             `json:"end_time" db:"end_time"`
	DurationMs         int64                 `json:"duration_ms" db:"duration_ms"`
	Passed             bool                  `json:"passed" db:"passed"`
	Score              *float64              `json:"score" db:"score"`
	Error              *string               `json:"error" db:"error"`
	Metadata           map[string]interface{} `json:"metadata" db:"metadata"`
	OriginalResponse   *string               `json:"original_response" db:"original_response"`   // Original LLM response (output guardrails only)
	OverrideResponse   *string               `json:"override_response" db:"override_response"`   // Override response sent to client
	ResponseOverridden bool                  `json:"response_overridden" db:"response_overridden"` // Whether response was overridden
	CreatedAt          time.Time             `json:"created_at" db:"created_at"`
}

// ExecutionResult represents the result of executing a set of guardrails
type ExecutionResult struct {
	Passed          bool              `json:"passed"`
	FailedGuardrail string            `json:"failed_guardrail,omitempty"`
	FailureReason   string            `json:"failure_reason,omitempty"`
	Results         []*GuardrailResult `json:"results"`
}

// GuardrailResult represents the result of a single guardrail execution
type GuardrailResult struct {
	Name     string        `json:"name"`
	Priority int           `json:"priority"`
	Result   *Result       `json:"result"`
	Duration time.Duration `json:"duration"`
}

// GuardrailFailure tracks failure information with priority
type GuardrailFailure struct {
	Name     string `json:"name"`
	Priority int    `json:"priority"`
	Reason   string `json:"reason"`
}

// GuardrailFactory is a function type for creating guardrails
type GuardrailFactory func(name string, priority int, config map[string]interface{}) (Guardrail, error)