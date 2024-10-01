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

func (pg *PostgresDB) SearchDocuments(queryVector []float32, datasetName string, userId int64, maxTotalWordCount int) ([]models.Document, error) {
	if len(queryVector) == 0 {
		return nil, errors.New("query vector cannot be empty")
	}
	if maxTotalWordCount <= 0 {
		return nil, errors.New("maxTotalWordCount must be greater than zero")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	vec := pgvector.NewVector(queryVector)

	query := `
        WITH ranked_docs AS (
            SELECT 
                documents.id, 
                documents.title, 
                documents.url, 
                documents.body, 
                datasets.id AS dataset_id,
                documents.vector <-> $3 AS distance,
                array_length(regexp_split_to_array(documents.body, '\s+'), 1) AS word_count,
                SUM(array_length(regexp_split_to_array(documents.body, '\s+'), 1)) OVER (ORDER BY documents.vector <-> $3) AS cumulative_word_count
            FROM documents
            JOIN datasets ON datasets.id = documents.dataset_id
            WHERE datasets.name = $1 AND datasets.user_id = $2
        )
        SELECT id, title, url, body, dataset_id
        FROM ranked_docs
        WHERE cumulative_word_count <= $4
        ORDER BY distance
    `

	rows, err := pg.db.QueryContext(ctx, query, datasetName, userId, vec, maxTotalWordCount)
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

func (pg *PostgresDB) GetOrCreateDataset(datasetName string, userId int64) (int64, error) {
	if datasetName == "" {
		return 0, errors.New("dataset name cannot be empty")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		WITH data as (
			INSERT INTO datasets (name, user_id)
			VALUES ($1, $2)
			ON CONFLICT (name, user_id) DO NOTHING
			RETURNING id
		)
		SELECT id FROM data
			UNION ALL
		SELECT id FROM datasets WHERE name=$1 AND user_id = $2
		LIMIT 1;
	`

	var id int64
	err := pg.db.QueryRowContext(ctx, query, datasetName, userId).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("failed to get dataset id: %w", err)
	}

	return id, err
}

func (pg *PostgresDB) GetAccessTokens() (*[]models.AccessToken, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		SELECT user_id, token, expiration
		FROM access_tokens
		WHERE expiration > NOW();
	`

	rows, err := pg.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	var accessTokens []models.AccessToken
	for rows.Next() {
		var accessToken models.AccessToken
		err := rows.Scan(&accessToken.UserID, &accessToken.Token, &accessToken.Expiration)
		if err != nil {
			return &[]models.AccessToken{}, fmt.Errorf("failed to scan document %w", err)
		}

		accessTokens = append(accessTokens, accessToken)
	}

	return &accessTokens, nil
}

func (pg *PostgresDB) Close() error {
	return pg.db.Close()
}
