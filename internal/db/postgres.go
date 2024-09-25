// postgres.go
package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
	"github.com/mrhollen/KnowledgeGPT/internal/models"
	"github.com/pgvector/pgvector-go"
)

// PostgresDB is the PostgreSQL implementation of the DB interface.
type PostgresDB struct {
	db *sql.DB
}

// NewPostgresDB creates a new instance of PostgresDB.
// It takes a PostgreSQL connection string as input.
func NewPostgresDB(connString string) (*PostgresDB, error) {
	// Open a connection to the PostgreSQL database
	db, err := sql.Open("postgres", connString)
	if err != nil {
		return nil, fmt.Errorf("unable to open database connection: %w", err)
	}

	// Configure connection pool settings
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(2)
	db.SetConnMaxIdleTime(5 * time.Minute)
	db.SetConnMaxLifetime(30 * time.Minute)

	// Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("unable to connect to database: %w", err)
	}

	return &PostgresDB{db: db}, nil
}

// AddDocument inserts a new document along with its vector into the database.
func (pg *PostgresDB) AddDocument(doc models.Document) error {
	if len(doc.Vec) == 0 {
		return errors.New("vector cannot be empty")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use pgvector-go to create a Vector type
	vec := pgvector.NewVector(doc.Vec)

	query := `
		INSERT INTO documents (title, url, body, vector)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`

	var insertedID int64
	err := pg.db.QueryRowContext(ctx, query, doc.Title, doc.URL, doc.Body, vec).Scan(&insertedID)
	if err != nil {
		return fmt.Errorf("failed to insert document: %w", err)
	}

	doc.ID = insertedID
	return nil
}

// SearchDocuments performs a K-Nearest Neighbors (KNN) search using the pgvector extension.
func (pg *PostgresDB) SearchDocuments(queryVector []float32, limit int) ([]models.Document, error) {
	if len(queryVector) == 0 {
		return nil, errors.New("query vector cannot be empty")
	}
	if limit <= 0 {
		return nil, errors.New("limit must be greater than zero")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Use pgvector-go to create a Vector type
	vec := pgvector.NewVector(queryVector)

	query := `
		SELECT id, title, url, body
		FROM documents
		ORDER BY vector <-> $1
		LIMIT $2
	`

	rows, err := pg.db.QueryContext(ctx, query, vec, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search query: %w", err)
	}
	defer rows.Close()

	var documents []models.Document
	for rows.Next() {
		var doc models.Document
		err := rows.Scan(&doc.ID, &doc.Title, &doc.URL, &doc.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to scan document: %w", err)
		}
		documents = append(documents, doc)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("error iterating through documents: %w", rows.Err())
	}

	return documents, nil
}

// GetSession retrieves a chat session by its ID.
func (pg *PostgresDB) GetSession(id string) (*models.ChatSession, error) {
	if id == "" {
		return nil, errors.New("session ID cannot be empty")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		SELECT id, messages, model
		FROM sessions
		WHERE id = $1
	`

	var session models.ChatSession
	var messages []string

	err := pg.db.QueryRowContext(ctx, query, id).Scan(&session.ID, pq.Array(&messages), &session.Model)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("session with ID %s not found", id)
		}
		return nil, fmt.Errorf("failed to retrieve session: %w", err)
	}

	session.Messages = messages
	return &session, nil
}

// SaveSession saves or updates a chat session in the database.
func (pg *PostgresDB) SaveSession(session models.ChatSession) error {
	if session.ID == "" {
		return errors.New("session ID cannot be empty")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use pq.Array to handle the messages slice
	messages := pq.Array(session.Messages)

	query := `
		INSERT INTO sessions (id, messages, model)
		VALUES ($1, $2, $3)
		ON CONFLICT (id) DO UPDATE
		SET messages = EXCLUDED.messages,
		    model = EXCLUDED.model
	`

	_, err := pg.db.ExecContext(ctx, query, session.ID, messages, session.Model)
	if err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}

	return nil
}

// Close closes the database connection.
func (pg *PostgresDB) Close() error {
	return pg.db.Close()
}
