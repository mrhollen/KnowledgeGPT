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

type PostgresDB struct {
	db *sql.DB
}

func NewPostgresDB(connString string) (*PostgresDB, error) {
	db, err := sql.Open("postgres", connString)
	if err != nil {
		return nil, fmt.Errorf("unable to open database connection: %w", err)
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(2)
	db.SetConnMaxIdleTime(5 * time.Minute)
	db.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("unable to connect to database: %w", err)
	}

	return &PostgresDB{db: db}, nil
}

func (pg *PostgresDB) AddDocument(doc models.Document) error {
	if len(doc.Vec) == 0 {
		return errors.New("vector cannot be empty")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use pgvector-go to create a Vector type
	vec := pgvector.NewVector(doc.Vec)

	query := `
		INSERT INTO documents (dataset_id, title, url, body, vector)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`

	var insertedID int64
	err := pg.db.QueryRowContext(ctx, query, doc.DatasetID, doc.Title, doc.URL, doc.Body, vec).Scan(&insertedID)
	if err != nil {
		return fmt.Errorf("failed to insert document: %w", err)
	}

	return nil
}

func (pg *PostgresDB) SearchDocuments(queryVector []float32, datasetName string, limit int) ([]models.Document, error) {
	if len(queryVector) == 0 {
		return nil, errors.New("query vector cannot be empty")
	}
	if limit <= 0 {
		return nil, errors.New("limit must be greater than zero")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	vec := pgvector.NewVector(queryVector)

	query := `
		SELECT documents.id, documents.title, documents.url, documents.body, datasets.id
		FROM documents
		JOIN datasets ON datasets.id = documents.dataset_id
		WHERE datasets.name = $1
		ORDER BY documents.vector <-> $2
		LIMIT $3
	`

	rows, err := pg.db.QueryContext(ctx, query, datasetName, vec, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search query: %w", err)
	}
	defer rows.Close()

	var documents []models.Document
	for rows.Next() {
		var doc models.Document
		err := rows.Scan(&doc.ID, &doc.Title, &doc.URL, &doc.Body, &doc.DatasetID)
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

func (pg *PostgresDB) SaveSession(session models.ChatSession) error {
	if session.ID == "" {
		return errors.New("session ID cannot be empty")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

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

func (pg *PostgresDB) GetOrCreateDataset(datasetName string) (int64, error) {
	if datasetName == "" {
		return 0, errors.New("dataset name cannot be empty")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		WITH data as (
			INSERT INTO datasets (name)
			VALUES ($1)
			ON CONFLICT (name) DO NOTHING
			RETURNING id
		)
		SELECT id FROM data
			UNION ALL
		SELECT id FROM datasets WHERE name=$1
		LIMIT 1;
	`

	var id int64
	err := pg.db.QueryRowContext(ctx, query, datasetName).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to get dataset id: %w", err)
	}

	return id, err
}

func (pg *PostgresDB) Close() error {
	return pg.db.Close()
}