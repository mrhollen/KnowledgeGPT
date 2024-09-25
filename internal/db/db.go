package db

import "github.com/mrhollen/KnowledgeGPT/internal/models"

type DB interface {
	AddDocument(doc models.Document) error
	SearchDocuments(queryVector []float32, limit int) ([]models.Document, error)
	GetSession(id string) (*models.ChatSession, error)
	SaveSession(session models.ChatSession) error
}
