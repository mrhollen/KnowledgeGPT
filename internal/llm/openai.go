package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

type OpenAIClient struct {
	Endpoint          string
	EmbeddingEndpoint string
	APIKey            string
	HTTPClient        *http.Client
	systemPrompt      string
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
	Temperature int             `json:"tempurature"`
}

type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenAIEmbeddingResponseData struct {
	Embedding []float32 `json:"embedding"`
}

type OpenAIEmbeddingResponse struct {
	Data []OpenAIEmbeddingResponseData `json:"data"`
}

type OpenAIResponse struct {
	Choices []struct {
		Message OpenAIMessage `json:"message"`
	} `json:"choices"`
}

func NewOpenAIClient(endpoint string, embeddingEndpoint string, apiKey string, defaultModelName string) *OpenAIClient {
	systemPrompt, err := os.ReadFile("./system_prompt.txt")

	if err != nil {
		panic("No system prompt found! Please make sure there is a file named system_prompt.txt in your project root.")
	}

	return &OpenAIClient{
		Endpoint:          endpoint,
		EmbeddingEndpoint: embeddingEndpoint,
		APIKey:            apiKey,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		defaultModelName: defaultModelName,
		systemPrompt:     string(systemPrompt),
	}
}

func (c *OpenAIClient) GetEmbedding(input string, modelName string) ([]float32, error) {
	embeddingRequest := OpenAIEmbeddingRequest{
		Model: c.defaultModelName,
		Input: input,
	}

	data, err := json.Marshal(embeddingRequest)
	if err != nil {
		return []float32{}, err
	}

	req, err := http.NewRequest("POST", c.EmbeddingEndpoint, bytes.NewBuffer(data))
	if err != nil {
		return []float32{}, err
	}

	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return []float32{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return []float32{}, fmt.Errorf("LLM server returned status: %s", resp.Status)
	}

	var embeddingResponse OpenAIEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embeddingResponse); err != nil {
		return []float32{}, err
	}

	if len(embeddingResponse.Data) == 0 {
		return []float32{}, fmt.Errorf("no response from LLM server")
	}

	return embeddingResponse.Data[0].Embedding, nil
}

func (c *OpenAIClient) GetSearchWords(queryString string, modelName string) (string, error) {
	message := OpenAIMessage{
		Role:    "user",
		Content: "Please create a search query for this. ONLY give me the search string. Do not use quotes: \n\n" + queryString,
	}

	if modelName == "" {
		modelName = c.defaultModelName
	}

	reqBody := OpenAIRequest{
		Model:       modelName,
		Messages:    []OpenAIMessage{message},
		MaxTokens:   -1,
		Temperature: 0,
	}

	return c.getResponse(&reqBody)
}

func (c *OpenAIClient) SendPrompt(prompt string, modelName string) (string, error) {
	systemMessage := OpenAIMessage{
		Role:    "system",
		Content: c.systemPrompt,
	}

	message := OpenAIMessage{
		Role:    "user",
		Content: prompt,
	}

	if modelName == "" {
		modelName = c.defaultModelName
	}

	reqBody := OpenAIRequest{
		Model:       modelName,
		Messages:    []OpenAIMessage{systemMessage, message},
		MaxTokens:   -1,
		Temperature: 0,
	}

	return c.getResponse(&reqBody)
}

func (c *OpenAIClient) getResponse(reqBody *OpenAIRequest) (string, error) {
	fmt.Println(reqBody)

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", c.Endpoint, bytes.NewBuffer(data))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.APIKey))
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("LLM server returned status: %s", resp.Status)
	}

	var llmResp OpenAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&llmResp); err != nil {
		return "", err
	}

	if len(llmResp.Choices) == 0 {
		return "", fmt.Errorf("no response from LLM server")
	}

	return llmResp.Choices[0].Message.Content, nil
}
