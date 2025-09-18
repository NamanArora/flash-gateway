package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/NamanArora/flash-gateway/internal/guardrails"
)

// ModerationGuardrail implements content moderation using OpenAI's moderation API
type ModerationGuardrail struct {
	name        string
	priority    int
	apiKey      string
	blockOnFlag bool
	categories  []string
	httpClient  *http.Client
}

// Config structure for moderation guardrail
type ModerationConfig struct {
	APIKey      string   `json:"api_key"`
	BlockOnFlag bool     `json:"block_on_flag"`
	Categories  []string `json:"categories,omitempty"`
}

// Request structures for different OpenAI endpoints
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatCompletionRequest struct {
	Messages []ChatMessage `json:"messages"`
}

type ResponsesRequest struct {
	Input string `json:"input"`
}

type CompletionRequest struct {
	Prompt string `json:"prompt"`
}

// OpenAI Moderation API structures
type ModerationRequest struct {
	Input string `json:"input"`
}

type ModerationResponse struct {
	ID      string             `json:"id"`
	Model   string             `json:"model"`
	Results []ModerationResult `json:"results"`
}

type ModerationResult struct {
	Flagged         bool                       `json:"flagged"`
	Categories      map[string]bool           `json:"categories"`
	CategoryScores  map[string]float64        `json:"category_scores"`
}

// NewModerationGuardrail creates a new moderation guardrail
func NewModerationGuardrail(name string, priority int, config map[string]interface{}) *ModerationGuardrail {
	// Parse configuration
	var modConfig ModerationConfig
	if configBytes, err := json.Marshal(config); err == nil {
		json.Unmarshal(configBytes, &modConfig)
	}

	// Get API key from config or environment
	apiKey := modConfig.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}

	// Default to blocking on flag if not specified
	blockOnFlag := modConfig.BlockOnFlag
	if config["block_on_flag"] == nil {
		blockOnFlag = true
	}

	return &ModerationGuardrail{
		name:        name,
		priority:    priority,
		apiKey:      apiKey,
		blockOnFlag: blockOnFlag,
		categories:  modConfig.Categories,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Name returns the guardrail's unique identifier
func (m *ModerationGuardrail) Name() string {
	return m.name
}

// Priority returns execution priority (lower = higher priority)
func (m *ModerationGuardrail) Priority() int {
	return m.priority
}

// Check performs the moderation validation
func (m *ModerationGuardrail) Check(ctx context.Context, content string) (*guardrails.Result, error) {
	// Extract user message from request
	userMessage, err := m.extractUserMessage(content)
	if err != nil {
		return &guardrails.Result{
			Passed: true, // Don't block on parsing errors
			Reason: fmt.Sprintf("Failed to extract message: %v", err),
			Metadata: map[string]interface{}{
				"error":      err.Error(),
				"extraction": "failed",
			},
		}, nil
	}

	// Skip if no user message found
	if userMessage == "" {
		return &guardrails.Result{
			Passed: true,
			Reason: "No user message found to moderate",
			Metadata: map[string]interface{}{
				"extraction": "empty",
			},
		}, nil
	}

	// Call OpenAI moderation API
	moderationResult, err := m.callModerationAPI(ctx, userMessage)
	if err != nil {
		// Don't block requests on API failures
		return &guardrails.Result{
			Passed: true,
			Reason: fmt.Sprintf("Moderation API error: %v", err),
			Metadata: map[string]interface{}{
				"error":        err.Error(),
				"api_call":     "failed",
				"user_message": userMessage,
			},
		}, nil
	}

	// Check if content is flagged
	flagged := moderationResult.Flagged
	
	// If specific categories are configured, only block if those categories are violated
	if len(m.categories) > 0 {
		flagged = false
		for _, category := range m.categories {
			if moderationResult.Categories[category] {
				flagged = true
				break
			}
		}
	}

	// Determine if request should be blocked
	passed := !flagged || !m.blockOnFlag

	// Build metadata with detailed results
	metadata := map[string]interface{}{
		"user_message":    userMessage,
		"flagged":         moderationResult.Flagged,
		"categories":      moderationResult.Categories,
		"category_scores": moderationResult.CategoryScores,
		"api_call":        "success",
	}

	if len(m.categories) > 0 {
		metadata["configured_categories"] = m.categories
		metadata["configured_flagged"] = flagged
	}

	reason := "Content passed moderation"
	if flagged {
		violatedCategories := []string{}
		for category, violated := range moderationResult.Categories {
			if violated && (len(m.categories) == 0 || m.containsCategory(category)) {
				violatedCategories = append(violatedCategories, category)
			}
		}
		reason = fmt.Sprintf("Content flagged for: %s", strings.Join(violatedCategories, ", "))
	}

	return &guardrails.Result{
		Passed:   passed,
		Reason:   reason,
		Metadata: metadata,
	}, nil
}

// extractUserMessage extracts the user message from different request formats
func (m *ModerationGuardrail) extractUserMessage(content string) (string, error) {
	// Try to parse as different request types
	
	// 1. Try Chat Completion format
	var chatReq ChatCompletionRequest
	if err := json.Unmarshal([]byte(content), &chatReq); err == nil && len(chatReq.Messages) > 0 {
		// Find the last user message
		for i := len(chatReq.Messages) - 1; i >= 0; i-- {
			if chatReq.Messages[i].Role == "user" {
				return chatReq.Messages[i].Content, nil
			}
		}
	}

	// 2. Try Responses format
	var respReq ResponsesRequest
	if err := json.Unmarshal([]byte(content), &respReq); err == nil && respReq.Input != "" {
		return respReq.Input, nil
	}

	// 3. Try Completion format
	var compReq CompletionRequest
	if err := json.Unmarshal([]byte(content), &compReq); err == nil && compReq.Prompt != "" {
		return compReq.Prompt, nil
	}

	// If none of the above worked, try to extract any "content" field
	var generic map[string]interface{}
	if err := json.Unmarshal([]byte(content), &generic); err == nil {
		if content, ok := generic["content"].(string); ok {
			return content, nil
		}
		if input, ok := generic["input"].(string); ok {
			return input, nil
		}
		if prompt, ok := generic["prompt"].(string); ok {
			return prompt, nil
		}
	}

	return "", fmt.Errorf("unable to extract user message from request")
}

// callModerationAPI calls OpenAI's moderation API
func (m *ModerationGuardrail) callModerationAPI(ctx context.Context, text string) (*ModerationResult, error) {
	if m.apiKey == "" {
		return nil, fmt.Errorf("OpenAI API key not configured")
	}

	// Prepare request
	modReq := ModerationRequest{
		Input: text,
	}

	requestBody, err := json.Marshal(modReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.openai.com/v1/moderations", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.apiKey)

	// Make request
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	// Parse response
	var modResp ModerationResponse
	if err := json.NewDecoder(resp.Body).Decode(&modResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Return first result (there should only be one for single input)
	if len(modResp.Results) == 0 {
		return nil, fmt.Errorf("no results in moderation response")
	}

	return &modResp.Results[0], nil
}

// containsCategory checks if a category is in the configured categories list
func (m *ModerationGuardrail) containsCategory(category string) bool {
	for _, c := range m.categories {
		if c == category {
			return true
		}
	}
	return false
}