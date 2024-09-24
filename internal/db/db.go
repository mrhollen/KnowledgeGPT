package db

import "github.com/mrhollen/KnowledgeGPT/internal/models"

// DB defines the methods for interacting with the database
type DB interface {
	AddDocument(doc models.Document) error
	SearchDocuments(query string, limit int) ([]models.Document, error)
	GetSession(id string) (*models.ChatSession, error)
	SaveSession(session models.ChatSession) error
}
