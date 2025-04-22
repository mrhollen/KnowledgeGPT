package llm

import "context"

type Client interface {
	GetEmbedding(ctx context.Context, input string, modelName string) ([]float32, error)
	GetSearchWords(ctx context.Context, queryString string, modelName string) (string, error)
	SendPrompt(ctx context.Context, prompt string, systemPrompt string, modelName string) (string, error)
	ExtractConcepts(ctx context.Context, text string, modelName string) ([]string, error)
}
