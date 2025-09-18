package handlers

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// GuardrailResponseBuilder creates API-compatible responses for blocked content
type GuardrailResponseBuilder struct{}

// NewGuardrailResponseBuilder creates a new response builder
func NewGuardrailResponseBuilder() *GuardrailResponseBuilder {
	return &GuardrailResponseBuilder{}
}

// BuildResponse creates an appropriate API response based on the endpoint
func (b *GuardrailResponseBuilder) BuildResponse(endpoint string) ([]byte, error) {
	switch endpoint {
	case "/v1/chat/completions":
		return b.buildChatCompletionResponse()
	case "/v1/completions":
		return b.buildLegacyCompletionResponse()
	case "/v1/responses":
		// Assume responses endpoint uses chat completion format
		return b.buildChatCompletionResponse()
	default:
		// Default to chat completion format for unknown endpoints
		return b.buildChatCompletionResponse()
	}
}

// buildChatCompletionResponse creates a chat completion response
func (b *GuardrailResponseBuilder) buildChatCompletionResponse() ([]byte, error) {
	response := map[string]interface{}{
		"id":      fmt.Sprintf("chatcmpl-blocked-%s", uuid.New().String()[:8]),
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   "gpt-3.5-turbo",
		"choices": []map[string]interface{}{
			{
				"index": 0,
				"message": map[string]interface{}{
					"role":    "assistant",
					"content": "I cannot service this request",
					"refusal": nil,
				},
				"logprobs":      nil,
				"finish_reason": "stop",
			},
		},
		"usage": map[string]interface{}{
			"prompt_tokens":     0,
			"completion_tokens": 6, // "I cannot service this request" is roughly 6 tokens
			"total_tokens":      6,
		},
		"system_fingerprint": "fp_guardrail_blocked",
	}

	return json.Marshal(response)
}

// buildLegacyCompletionResponse creates a legacy text completion response
func (b *GuardrailResponseBuilder) buildLegacyCompletionResponse() ([]byte, error) {
	response := map[string]interface{}{
		"id":      fmt.Sprintf("cmpl-blocked-%s", uuid.New().String()[:8]),
		"object":  "text_completion",
		"created": time.Now().Unix(),
		"model":   "gpt-3.5-turbo",
		"choices": []map[string]interface{}{
			{
				"text":          "I cannot service this request",
				"index":         0,
				"logprobs":      nil,
				"finish_reason": "stop",
			},
		},
		"usage": map[string]interface{}{
			"prompt_tokens":     0,
			"completion_tokens": 6, // "I cannot service this request" is roughly 6 tokens
			"total_tokens":      6,
		},
	}

	return json.Marshal(response)
}

// GetBlockedMessage returns the standard blocked message
func (b *GuardrailResponseBuilder) GetBlockedMessage() string {
	return "I cannot service this request"
}

// GuardrailBlockContext holds information about blocked requests
type GuardrailBlockContext struct {
	Blocked          bool
	Layer            string // "input" or "output"
	GuardrailName    string
	GuardrailReason  string
	OriginalResponse []byte // Only for output guardrails
	OverrideResponse []byte // The fake response we generate
}