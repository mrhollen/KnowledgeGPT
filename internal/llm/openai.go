package llm

import (
	"bytes"
	"context" // Added import
	"encoding/json"
	"fmt"
	"io"
	"log" // Added import
	"net/http"
	"regexp" // Added import
	"strings"
	"time"
)

// Compile the regex for stripping <think> tags once for efficiency.
// The (?s) flag makes '.' match newlines as well.
var thinkTagRegex = regexp.MustCompile(`(?s)<think>.*?</think>`)

type OpenAIClient struct {
	Endpoint          string
	EmbeddingEndpoint string
	APIKey            string
	HTTPClient        *http.Client
	defaultModelName  string
}

type OpenAIEmbeddingRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type OpenAIRequest struct {
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float32         `json:"temperature"` // Changed to float32 (common for OpenAI)
	Seed        *int            `json:"seed,omitempty"`
	// Stop sequences could be added here if needed
	// Stop []string `json:"stop,omitempty"`
}

type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenAIEmbeddingResponseData struct {
	Embedding []float32 `json:"embedding"`
	Index     int       `json:"index"`  // Added index field
	Object    string    `json:"object"` // Added object field
}

// Adjusted according to common OpenAI API response structure
type OpenAIEmbeddingResponse struct {
	Data   []OpenAIEmbeddingResponseData `json:"data"`
	Model  string                        `json:"model"`
	Object string                        `json:"object"`
	Usage  map[string]int                `json:"usage"`
}

type OpenAIResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index        int           `json:"index"`
		Message      OpenAIMessage `json:"message"`
		FinishReason string        `json:"finish_reason"`
		// LogProbs could be here if requested
	} `json:"choices"`
	Usage map[string]int `json:"usage"` // Added usage field
	// SystemFingerprint string `json:"system_fingerprint"` // Optional
}

func NewOpenAIClient(endpoint string, embeddingEndpoint string, apiKey string, defaultModelName string) *OpenAIClient {
	// Default timeout
	timeout := 1000 * time.Second // Increased timeout for potentially long LLM responses

	return &OpenAIClient{
		Endpoint:          endpoint,
		EmbeddingEndpoint: embeddingEndpoint,
		APIKey:            apiKey,
		HTTPClient: &http.Client{
			Timeout: timeout,
		},
		defaultModelName: defaultModelName,
	}
}

// GetEmbedding retrieves the embedding vector for the given input text.
// Added context.Context for better control (cancellation, deadlines).
func (c *OpenAIClient) GetEmbedding(ctx context.Context, input string, modelName string) ([]float32, error) {
	if modelName == "" {
		modelName = c.defaultModelName // Use default if not specified
		// Consider checking if c.defaultModelName is actually an embedding model
	}
	// Basic input validation
	if strings.TrimSpace(input) == "" {
		return nil, fmt.Errorf("embedding input cannot be empty")
	}

	embeddingRequest := OpenAIEmbeddingRequest{
		Model: modelName, // Use the resolved model name
		Input: input,
	}

	data, err := json.Marshal(embeddingRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal embedding request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.EmbeddingEndpoint, bytes.NewBuffer(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create embedding HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send embedding request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Attempt to read body for more error info
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embedding endpoint returned status %s: %s", resp.Status, string(bodyBytes))
	}

	var embeddingResponse OpenAIEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embeddingResponse); err != nil {
		return nil, fmt.Errorf("failed to decode embedding response: %w", err)
	}

	if len(embeddingResponse.Data) == 0 {
		return nil, fmt.Errorf("no embedding data received from LLM server")
	}
	// Assuming we only care about the first embedding if multiple are returned (unlikely for single input)
	if len(embeddingResponse.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("received empty embedding vector")
	}

	return embeddingResponse.Data[0].Embedding, nil
}

// SendPrompt sends a prompt (including system prompt) to the LLM and returns the response.
// Added context.Context.
func (c *OpenAIClient) SendPrompt(ctx context.Context, prompt string, systemPrompt string, modelName string) (string, error) {
	messages := make([]OpenAIMessage, 0, 2)
	// Include system prompt only if it's not empty
	if systemPrompt != "" {
		messages = append(messages, OpenAIMessage{
			Role:    "system",
			Content: systemPrompt,
		})
	}
	messages = append(messages, OpenAIMessage{
		Role:    "user",
		Content: prompt,
	})

	if modelName == "" {
		modelName = c.defaultModelName
	}

	// Reasonable defaults, make these configurable if needed
	reqBody := OpenAIRequest{
		Model:    modelName,
		Messages: messages,
		// MaxTokens:   2048, // Set a reasonable max_tokens limit
		Temperature: 0.7, // Common default, adjust as needed
	}

	// Pass context down
	return c.getResponse(ctx, &reqBody)
}

// ExtractConcepts is an example implementation using the LLM client interface.
// Added context.Context.
func (c *OpenAIClient) ExtractConcepts(ctx context.Context, text string, modelName string) ([]string, error) {
	// This implementation uses SendPrompt, adjust if a dedicated endpoint or logic is better
	prompt := fmt.Sprintf("Identify the main 3-5 core concepts or keywords from the following text. List each concept on a new line, without any preamble or explanation:\n\n```\n%s\n```", text)

	// Use a potentially different model or settings optimized for extraction
	if modelName == "" {
		modelName = c.defaultModelName // Or a specific extraction model
	}

	response, err := c.SendPrompt(ctx, prompt, "", modelName) // Pass context
	if err != nil {
		return nil, fmt.Errorf("failed to extract concepts using LLM: %w", err)
	}

	concepts := strings.Split(strings.TrimSpace(response), "\n")
	var cleanedConcepts []string
	for _, concept := range concepts {
		trimmed := strings.TrimSpace(concept)
		if trimmed != "" {
			trimmed = strings.TrimPrefix(trimmed, "- ")
			trimmed = strings.TrimPrefix(trimmed, "* ")
			cleanedConcepts = append(cleanedConcepts, trimmed)
		}
	}

	if len(cleanedConcepts) == 0 {
		log.Printf("Warning: No concepts extracted via LLM for text starting with: %s...", text[:min(50, len(text))])
		// Return empty slice, not an error, if LLM genuinely found none or response was empty
	}
	return cleanedConcepts, nil
}

// getResponse sends the request to the LLM API and processes the response.
// It now accepts context.Context and strips <think> tags.
func (c *OpenAIClient) getResponse(ctx context.Context, reqBody *OpenAIRequest) (string, error) {
	// Log the request *before* marshaling if needed for debugging sensitive data carefully
	// log.Printf("Sending request to LLM: %+v", reqBody) // Be mindful of logging prompts/data

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal LLM request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.Endpoint, bytes.NewBuffer(data))
	if err != nil {
		return "", fmt.Errorf("failed to create LLM HTTP request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json") // Be explicit about accepted response type
	if c.APIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		// Check context error type if possible
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("LLM request cancelled or timed out: %w", ctx.Err())
		default:
			return "", fmt.Errorf("failed to send LLM request: %w", err)
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Attempt to read body for more error info
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("LLM server returned status %s: %s", resp.Status, string(bodyBytes))
	}

	var llmResp OpenAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&llmResp); err != nil {
		return "", fmt.Errorf("failed to decode LLM response: %w", err)
	}

	if len(llmResp.Choices) == 0 || llmResp.Choices[0].Message.Content == "" {
		// Log the full response maybe?
		log.Printf("Received empty or unexpected response from LLM: %+v", llmResp)
		return "", fmt.Errorf("no valid response content from LLM server")
	}

	// --- Strip <think> tags ---
	originalContent := llmResp.Choices[0].Message.Content
	// Use the pre-compiled regex to remove all occurrences of <think>...</think> blocks
	cleanedContent := thinkTagRegex.ReplaceAllString(originalContent, "")
	// Optionally, trim leading/trailing whitespace that might result from removal
	cleanedContent = strings.TrimSpace(cleanedContent)

	// Log if content was changed (optional, for debugging)
	if cleanedContent != originalContent {
		log.Printf("Stripped <think> tags from LLM response.")
		// log.Printf("Original: %s", originalContent) // Be careful logging full content
		// log.Printf("Cleaned: %s", cleanedContent)
	}
	// --- End strip ---

	// Return the cleaned content
	return cleanedContent, nil
}

// --- Deprecated / To Be Refactored ---

// GetSearchWords - Consider if this specific transformation is needed
// or if a general prompt refinement approach is better. Added context.
func (c *OpenAIClient) GetSearchWords(ctx context.Context, queryString string, modelName string) (string, error) {
	// The prompt is very specific, ensure the model understands "ONLY give me the search string"
	message := OpenAIMessage{
		Role:    "user",
		Content: "Please create a concise search query string suitable for a vector database based on the following user request. Output ONLY the search string, without quotes or any explanation:\n\nUser Request: " + queryString,
	}

	if modelName == "" {
		modelName = c.defaultModelName
	}

	// Using a fixed seed might make the output deterministic, which could be good or bad.
	// Setting seed might not be supported by all OpenAI compatible endpoints.
	seedValue := 1234
	seedPtr := &seedValue

	reqBody := OpenAIRequest{
		Model:    modelName,
		Messages: []OpenAIMessage{message},
		// MaxTokens:   50, // Limit tokens for a short search query
		Temperature: 0.1, // Low temperature for more focused output
		Seed:        seedPtr,
	}

	// Pass context down
	return c.getResponse(ctx, &reqBody)
}

// Helper for min calculation (if needed elsewhere)
// func min(a, b int) int {
// 	if a < b {
// 		return a
// 	}
// 	return b
// }

// Ensure OpenAIClient implements the Client interface
var _ Client = (*OpenAIClient)(nil)
