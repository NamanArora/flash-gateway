package guardrails

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
)

// Executor manages parallel guardrail execution with cancellation
type Executor struct {
	inputGuardrails  []Guardrail
	outputGuardrails []Guardrail
	metricsWriter    *MetricsWriter
	timeout          time.Duration
}

// ExecutorConfig holds configuration for the executor
type ExecutorConfig struct {
	InputGuardrails  []Guardrail
	OutputGuardrails []Guardrail
	MetricsWriter    *MetricsWriter
	Timeout          time.Duration
}

// NewExecutor creates a new guardrail executor
func NewExecutor(config ExecutorConfig) *Executor {
	if config.Timeout <= 0 {
		config.Timeout = 5 * time.Second // Default timeout
	}

	return &Executor{
		inputGuardrails:  config.InputGuardrails,
		outputGuardrails: config.OutputGuardrails,
		metricsWriter:    config.MetricsWriter,
		timeout:          config.Timeout,
	}
}

// ExecuteInput runs all input guardrails in parallel
func (e *Executor) ExecuteInput(ctx context.Context, requestID uuid.UUID, content string) (*ExecutionResult, error) {
	return e.executeParallel(ctx, requestID, content, e.inputGuardrails, "input", nil, nil)
}

// ExecuteOutput runs all output guardrails in parallel  
func (e *Executor) ExecuteOutput(ctx context.Context, requestID uuid.UUID, content string) (*ExecutionResult, error) {
	return e.executeParallel(ctx, requestID, content, e.outputGuardrails, "output", nil, nil)
}

// ExecuteOutputWithResponses runs all output guardrails in parallel and includes response data for metrics
func (e *Executor) ExecuteOutputWithResponses(ctx context.Context, requestID uuid.UUID, content string, originalResponse, overrideResponse []byte) (*ExecutionResult, error) {
	return e.executeParallel(ctx, requestID, content, e.outputGuardrails, "output", originalResponse, overrideResponse)
}

// executeParallel runs guardrails in priority groups - same priority runs in parallel, different priorities run sequentially
func (e *Executor) executeParallel(ctx context.Context, requestID uuid.UUID, content string, guardrails []Guardrail, layer string, originalResponse, overrideResponse []byte) (*ExecutionResult, error) {
	if len(guardrails) == 0 {
		return &ExecutionResult{Passed: true, Results: []*GuardrailResult{}}, nil
	}

	// Create timeout context
	ctx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()
	
	// Group guardrails by priority
	priorityGroups := make(map[int][]Guardrail)
	for _, g := range guardrails {
		priority := g.Priority()
		priorityGroups[priority] = append(priorityGroups[priority], g)
	}
	
	// Get sorted priority levels (ascending order - lower number = higher priority)
	var priorities []int
	for p := range priorityGroups {
		priorities = append(priorities, p)
	}
	sort.Ints(priorities)
	
	// Execute each priority group sequentially
	var allResults []*GuardrailResult
	currentContent := content // Track content modifications
	
	for _, priority := range priorities {
		groupGuardrails := priorityGroups[priority]
		
		// Execute this priority group in parallel
		groupResult, err := e.executeGroupParallel(ctx, requestID, currentContent, groupGuardrails, layer, originalResponse, overrideResponse)
		if err != nil {
			return &ExecutionResult{
				Passed:        false,
				FailureReason: fmt.Sprintf("Group execution failed: %v", err),
				Results:       allResults,
			}, nil
		}
		
		// If any guardrail in this group failed, stop execution immediately
		if !groupResult.Passed {
			// Append results from this group and return failure
			allResults = append(allResults, groupResult.Results...)
			return &ExecutionResult{
				Passed:          false,
				FailedGuardrail: groupResult.FailedGuardrail,
				FailureReason:   groupResult.FailureReason,
				Results:         allResults,
			}, nil
		}
		
		// All guardrails in this group passed - append results
		allResults = append(allResults, groupResult.Results...)
		
		// Check if any guardrail in this group modified the content
		for _, result := range groupResult.Results {
			if result != nil && result.Result != nil && result.Result.ModifiedContent != nil {
				currentContent = *result.Result.ModifiedContent // Use modified content for next priority group
				break // Use first modification found in this priority group
			}
		}
	}
	
	// All guardrails in all priority groups passed
	return &ExecutionResult{
		Passed:  true,
		Results: allResults,
	}, nil
}

// executeGroupParallel executes a group of guardrails (same priority) in parallel
func (e *Executor) executeGroupParallel(ctx context.Context, requestID uuid.UUID, content string, guardrails []Guardrail, layer string, originalResponse, overrideResponse []byte) (*ExecutionResult, error) {
	if len(guardrails) == 0 {
		return &ExecutionResult{Passed: true, Results: []*GuardrailResult{}}, nil
	}
	
	// Use errgroup for parallel execution with context cancellation
	g, ctx := errgroup.WithContext(ctx)
	
	// Thread-safe storage for results
	results := make([]*GuardrailResult, len(guardrails))
	resultsMu := sync.Mutex{}
	
	// Track first failure with highest priority
	var firstFailure *GuardrailFailure
	var failureMu sync.Mutex

	// Execute each guardrail in parallel
	for i, guardrail := range guardrails {
		i, guardrail := i, guardrail // capture loop variables
		
		g.Go(func() error {
			startTime := time.Now()
			
			// Check if context already cancelled
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			
			// Execute guardrail with instrumentation
			result, err := guardrail.Check(ctx, content)
			
			duration := time.Since(startTime)
			
			// Create metric for this execution
			metric := &Metric{
				ID:            uuid.New(),
				RequestID:     requestID,
				GuardrailName: guardrail.Name(),
				Layer:         layer,
				Priority:      guardrail.Priority(),
				StartTime:     startTime,
				EndTime:       time.Now(),
				DurationMs:    duration.Milliseconds(),
			}
			
			// Handle execution error
			if err != nil {
				errStr := err.Error()
				metric.Error = &errStr
				metric.Passed = false
				
				// Write metric asynchronously
				if e.metricsWriter != nil {
					e.metricsWriter.Write(metric)
				}
				
				// Track failure if it's the highest priority so far
				failureMu.Lock()
				if firstFailure == nil || guardrail.Priority() < firstFailure.Priority {
					firstFailure = &GuardrailFailure{
						Name:     guardrail.Name(),
						Priority: guardrail.Priority(),
						Reason:   err.Error(),
					}
				}
				failureMu.Unlock()
				
				return fmt.Errorf("%s failed: %w", guardrail.Name(), err)
			}
			
			// Update metric with result data
			metric.Passed = result.Passed
			metric.Score = result.Score
			metric.Metadata = result.Metadata
			
			// Add response override data if this is a failed output guardrail
			if !result.Passed && layer == "output" && originalResponse != nil && overrideResponse != nil {
				originalStr := string(originalResponse)
				overrideStr := string(overrideResponse)
				metric.OriginalResponse = &originalStr
				metric.OverrideResponse = &overrideStr
				metric.ResponseOverridden = true
			}
			
			// Write metric asynchronously
			if e.metricsWriter != nil {
				e.metricsWriter.Write(metric)
			}
			
			// Check if guardrail passed
			if !result.Passed {
				// Track failure if it's the highest priority so far
				failureMu.Lock()
				if firstFailure == nil || guardrail.Priority() < firstFailure.Priority {
					firstFailure = &GuardrailFailure{
						Name:     guardrail.Name(),
						Priority: guardrail.Priority(),
						Reason:   result.Reason,
					}
				}
				failureMu.Unlock()
				
				return fmt.Errorf("%s rejected: %s", guardrail.Name(), result.Reason)
			}
			
			// Store successful result
			resultsMu.Lock()
			results[i] = &GuardrailResult{
				Name:     guardrail.Name(),
				Priority: guardrail.Priority(),
				Result:   result,
				Duration: duration,
			}
			resultsMu.Unlock()
			
			return nil
		})
	}
	
	// Wait for all guardrails in this group or first failure
	err := g.Wait()
	
	// If there was an error and we tracked a failure, return the highest priority failure
	if err != nil && firstFailure != nil {
		return &ExecutionResult{
			Passed:          false,
			FailedGuardrail: firstFailure.Name,
			FailureReason:   firstFailure.Reason,
			Results:         results,
		}, nil
	}
	
	// If there was an error but no tracked failure (e.g., timeout), return generic error
	if err != nil {
		return &ExecutionResult{
			Passed:        false,
			FailureReason: fmt.Sprintf("Guardrail execution failed: %v", err),
			Results:       results,
		}, nil
	}
	
	// All guardrails in this group passed
	return &ExecutionResult{
		Passed:  true,
		Results: results,
	}, nil
}

// AddInputGuardrail adds an input guardrail to the executor
func (e *Executor) AddInputGuardrail(guardrail Guardrail) {
	e.inputGuardrails = append(e.inputGuardrails, guardrail)
	
	// Keep sorted by priority
	sort.Slice(e.inputGuardrails, func(i, j int) bool {
		return e.inputGuardrails[i].Priority() < e.inputGuardrails[j].Priority()
	})
}

// AddOutputGuardrail adds an output guardrail to the executor
func (e *Executor) AddOutputGuardrail(guardrail Guardrail) {
	e.outputGuardrails = append(e.outputGuardrails, guardrail)
	
	// Keep sorted by priority
	sort.Slice(e.outputGuardrails, func(i, j int) bool {
		return e.outputGuardrails[i].Priority() < e.outputGuardrails[j].Priority()
	})
}

// GetInputGuardrails returns all input guardrails
func (e *Executor) GetInputGuardrails() []Guardrail {
	return e.inputGuardrails
}

// GetOutputGuardrails returns all output guardrails
func (e *Executor) GetOutputGuardrails() []Guardrail {
	return e.outputGuardrails
}

// Close gracefully shuts down the executor
func (e *Executor) Close() error {
	if e.metricsWriter != nil {
		return e.metricsWriter.Close()
	}
	return nil
}