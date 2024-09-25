package llm

// Client defines the method to send prompts to the LLM server
type Client interface {
	SendPrompt(prompt string, modelName string) (string, error)
}
