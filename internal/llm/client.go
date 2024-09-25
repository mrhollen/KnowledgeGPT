package llm

type Client interface {
	GetEmbedding(input string, modelName string) ([]float32, error)
	GetSearchWords(queryString string, modelName string) (string, error)
	SendPrompt(prompt string, modelName string) (string, error)
}
