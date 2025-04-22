package models

import "time"

// Document represents the metadata for a document stored in the database.
// The actual content is stored in associated DocumentChunk records.
type Document struct {
	ID        int64           `json:"id" db:"id"`
	DatasetID int64           `json:"dataset_id" db:"dataset_id"`
	Title     string          `json:"title" db:"title"`
	URL       *string         `json:"url,omitempty" db:"url"` // Use pointer for NULLable fields
	CreatedAt time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt time.Time       `json:"updated_at" db:"updated_at"`
	Chunks    []DocumentChunk `json:"chunks,omitempty"`
}

// DocumentChunk represents a sequential chunk of text from a parent Document.
type DocumentChunk struct {
	ID             int64             `json:"id" db:"id"`
	DocumentID     int64             `json:"document_id" db:"document_id"`
	SequenceNumber int               `json:"sequence_number" db:"sequence_number"` // Order of the chunk
	ChunkText      string            `json:"chunk_text" db:"chunk_text"`           // Text content of the chunk
	Concepts       []DocumentConcept `json:"concepts,omitempty"`
}

// DocumentConcept represents a core concept extracted from a DocumentChunk,
// along with its specific embedding vector.
type DocumentConcept struct {
	ID              int64     `json:"id" db:"id"`
	DocumentChunkID int64     `json:"document_chunk_id" db:"document_chunk_id"` // Link to the source chunk
	ConceptText     string    `json:"concept_text" db:"concept_text"`           // The extracted concept
	Vector          []float32 `json:"vector" db:"vector"`                       // Embedding vector for this concept
}

type ChatSession struct {
	ID       string   `json:"id"`
	Messages []string `json:"messages"`
	Model    string   `json:"model"`
}

type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

type AccessToken struct {
	UserID     int64     `json:"id"`
	Token      string    `json:"token"`
	Expiration time.Time `json:"expiration"`
}
