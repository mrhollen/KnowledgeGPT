// postgres.go
package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt" // Added for word count calculation if needed
	"time"

	"github.com/lib/pq" // Keep for session arrays if still needed
	"github.com/mrhollen/KnowledgeGPT/internal/models"
	"github.com/pgvector/pgvector-go"
)

type PostgresDB struct {
	db *sql.DB
}

// ChunkWithConcepts is a helper struct to group a chunk and its derived concepts
// for insertion, before database IDs are assigned.
type ChunkWithConcepts struct {
	Chunk    models.DocumentChunk     // SequenceNumber and ChunkText must be set
	Concepts []models.DocumentConcept // ConceptText and Vector must be set
}

func NewPostgresDB(connString string) (*PostgresDB, error) {
	db, err := sql.Open("postgres", connString)
	if err != nil {
		return nil, fmt.Errorf("unable to open database connection: %w", err)
	}

	// Consider tuning these based on expected load
	db.SetMaxOpenConns(15) // Slightly increased for potentially more complex transactions
	db.SetMaxIdleConns(5)  // Slightly increased
	db.SetConnMaxIdleTime(5 * time.Minute)
	db.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close() // Close the connection if ping fails
		return nil, fmt.Errorf("unable to connect to database: %w", err)
	}

	var vectorExists bool
	err = db.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM pg_extension WHERE extname = 'vector')").Scan(&vectorExists)
	if err != nil || !vectorExists {
		db.Close()
		return nil, fmt.Errorf("pgvector extension not found or check failed: %w", err)
	}

	return &PostgresDB{db: db}, nil
}

// Close closes the database connection pool.
func (pg *PostgresDB) Close() error {
	if pg.db != nil {
		return pg.db.Close()
	}
	return nil
}

// --- Document, Chunk, and Concept Management ---

// AddDocumentAndContent inserts document metadata, its chunks, and associated concepts
// within a single transaction.
func (pg *PostgresDB) AddDocumentAndContent(ctx context.Context, doc models.Document, content []ChunkWithConcepts) (int64, error) {
	tx, err := pg.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	// Ensure rollback happens if commit doesn't
	defer tx.Rollback() // Rollback is ignored if tx is committed

	// 1. Insert Document Metadata
	docQuery := `
        INSERT INTO documents (dataset_id, title, url, created_at, updated_at)
        VALUES ($1, $2, $3, NOW(), NOW())
        RETURNING id
    `
	var documentID int64
	err = tx.QueryRowContext(ctx, docQuery, doc.DatasetID, doc.Title, doc.URL).Scan(&documentID)
	if err != nil {
		return 0, fmt.Errorf("failed to insert document metadata: %w", err)
	}

	// Prepare statements for efficiency within the loop
	chunkStmt, err := tx.PrepareContext(ctx, `
        INSERT INTO document_chunks (document_id, sequence_number, chunk_text)
        VALUES ($1, $2, $3)
        RETURNING id
    `)
	if err != nil {
		return 0, fmt.Errorf("failed to prepare chunk insert statement: %w", err)
	}
	defer chunkStmt.Close()

	conceptStmt, err := tx.PrepareContext(ctx, `
        INSERT INTO document_concepts (document_chunk_id, concept_text, vector)
        VALUES ($1, $2, $3)
        RETURNING id -- Optional: return concept ID if needed elsewhere
    `)
	if err != nil {
		return 0, fmt.Errorf("failed to prepare concept insert statement: %w", err)
	}
	defer conceptStmt.Close()

	// 2. Insert Chunks and their Concepts
	for _, item := range content {
		if item.Chunk.ChunkText == "" {
			// Or handle as appropriate, maybe skip? Returning error ensures data integrity.
			return 0, fmt.Errorf("chunk text cannot be empty for sequence %d in document %d", item.Chunk.SequenceNumber, documentID)
		}

		// Insert Chunk
		var chunkID int64
		err = chunkStmt.QueryRowContext(ctx, documentID, item.Chunk.SequenceNumber, item.Chunk.ChunkText).Scan(&chunkID)
		if err != nil {
			return 0, fmt.Errorf("failed to insert chunk (sequence %d): %w", item.Chunk.SequenceNumber, err)
		}

		// Insert Concepts for this Chunk
		for _, concept := range item.Concepts {
			if concept.ConceptText == "" || len(concept.Vector) == 0 {
				// Or handle as appropriate. Skipping might be acceptable here?
				// Returning error ensures concepts are always valid if present.
				return 0, fmt.Errorf("concept text or vector cannot be empty for chunk %d", chunkID)
			}

			// Convert Go slice to pgvector type for insertion
			vec := pgvector.NewVector(concept.Vector)

			_, err = conceptStmt.ExecContext(ctx, chunkID, concept.ConceptText, vec)
			if err != nil {
				return 0, fmt.Errorf("failed to insert concept ('%s') for chunk %d: %w", concept.ConceptText, chunkID, err)
			}
		}
	}

	// 3. Commit Transaction
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return documentID, nil
}

// --- Search Functionality ---

// SearchResult combines information from concepts, chunks, and documents for search results.
type SearchResult struct {
	DocumentID      int64   `json:"document_id"`
	DocumentTitle   string  `json:"document_title"`
	DocumentURL     *string `json:"document_url,omitempty"`
	ChunkID         int64   `json:"chunk_id"`
	ChunkSequence   int     `json:"chunk_sequence"`
	ChunkText       string  `json:"chunk_text"` // Include chunk text for context
	ConceptID       int64   `json:"concept_id"`
	ConceptText     string  `json:"concept_text"`
	SimilarityScore float64 `json:"similarity_score"` // Cosine similarity or distance
	// ConceptVector   []float32 `json:"-"` // Usually not needed in results, exclude from JSON
}

// SearchConcepts performs a vector similarity search on document concepts.
// It returns a ranked list of SearchResult, joining concepts with their chunks and documents.
func (pg *PostgresDB) SearchConcepts(ctx context.Context, queryVector []float32, datasetName string, userId int64, maxResults int) ([]SearchResult, error) {
	if len(queryVector) == 0 {
		return nil, errors.New("query vector cannot be empty")
	}
	if maxResults <= 0 {
		return nil, errors.New("maxResults must be greater than zero")
	}

	// Use pgvector-go type for the query parameter
	vecParam := pgvector.NewVector(queryVector)

	// NOTE: Adjust the distance operator (<-> for L2, <#> for inner product, <=> for cosine)
	// and the score calculation based on your indexing choice and preference.
	// Cosine distance (<=>) is often preferred. Distance is 0 for identical, higher for different.
	// We can calculate similarity = 1 - distance for cosine similarity.
	query := `
        SELECT
            d.id AS document_id,
            d.title AS document_title,
            d.url AS document_url,
            dc.id AS chunk_id,
            dc.sequence_number AS chunk_sequence,
            dc.chunk_text,
            dcon.id AS concept_id,
            dcon.concept_text,
            dcon.vector <=> $3 AS distance -- Cosine Distance operator
        FROM document_concepts dcon
        JOIN document_chunks dc ON dcon.document_chunk_id = dc.id
        JOIN documents d ON dc.document_id = d.id
        JOIN datasets ds ON d.dataset_id = ds.id
        WHERE ds.name = $1 AND ds.user_id = $2
        ORDER BY distance ASC -- Lower distance is better
        LIMIT $4
    `

	rows, err := pg.db.QueryContext(ctx, query, datasetName, userId, vecParam, maxResults)
	if err != nil {
		return nil, fmt.Errorf("failed to execute concept search query: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var res SearchResult
		var distance float64 // Scan distance separately
		err := rows.Scan(
			&res.DocumentID,
			&res.DocumentTitle,
			&res.DocumentURL, // Scan directly into *string
			&res.ChunkID,
			&res.ChunkSequence,
			&res.ChunkText,
			&res.ConceptID,
			&res.ConceptText,
			&distance, // Scan the calculated distance
		)
		if err != nil {
			// Log the error but try to continue if possible, or return immediately
			// log.Printf("Warning: failed to scan search result row: %v", err)
			// continue
			return nil, fmt.Errorf("failed to scan search result: %w", err)
		}
		// Calculate similarity score (assuming cosine distance <=> was used)
		res.SimilarityScore = 1.0 - distance
		results = append(results, res)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating through search results: %w", err)
	}

	return results, nil
}

// SearchConceptsWithWordCount performs a vector search and limits results
// based on the cumulative word count of the associated chunks.
func (pg *PostgresDB) SearchConceptsWithWordCount(ctx context.Context, queryVector []float32, datasetName string, userId int64, maxTotalWordCount int) ([]SearchResult, error) {
	if len(queryVector) == 0 {
		return nil, errors.New("query vector cannot be empty")
	}
	if maxTotalWordCount <= 0 {
		return nil, errors.New("maxTotalWordCount must be greater than zero")
	}

	vecParam := pgvector.NewVector(queryVector)

	// This query is more complex:
	// 1. Rank concepts by distance.
	// 2. Join with chunks to get text and calculate word count per chunk.
	// 3. Calculate cumulative word count based on concept rank.
	// 4. Filter based on cumulative word count.
	query := `
        WITH ranked_concepts AS (
            SELECT
                d.id AS document_id,
                d.title AS document_title,
                d.url AS document_url,
                dc.id AS chunk_id,
                dc.sequence_number AS chunk_sequence,
                dc.chunk_text,
                dcon.id AS concept_id,
                dcon.concept_text,
                dcon.vector <=> $3 AS distance, -- Cosine Distance operator
                -- Calculate word count for the chunk associated with the concept
                array_length(regexp_split_to_array(dc.chunk_text, '\s+'), 1) AS chunk_word_count
            FROM document_concepts dcon
            JOIN document_chunks dc ON dcon.document_chunk_id = dc.id
            JOIN documents d ON dc.document_id = d.id
            JOIN datasets ds ON d.dataset_id = ds.id
            WHERE ds.name = $1 AND ds.user_id = $2
            -- ORDER BY distance ASC -- Order applied in the window function
        ), cumulative_ranked AS (
            SELECT
                *,
                -- Calculate cumulative word count based on concept distance ranking
                SUM(chunk_word_count) OVER (ORDER BY distance ASC ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW) AS cumulative_word_count
            FROM ranked_concepts
        )
        SELECT
            document_id,
            document_title,
            document_url,
            chunk_id,
            chunk_sequence,
            chunk_text,
            concept_id,
            concept_text,
            distance
        FROM cumulative_ranked
        WHERE cumulative_word_count <= $4
        ORDER BY distance ASC -- Final sort for the limited results
    `

	rows, err := pg.db.QueryContext(ctx, query, datasetName, userId, vecParam, maxTotalWordCount)
	if err != nil {
		return nil, fmt.Errorf("failed to execute concept search query with word count: %w", err)
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var res SearchResult
		var distance float64
		err := rows.Scan(
			&res.DocumentID,
			&res.DocumentTitle,
			&res.DocumentURL,
			&res.ChunkID,
			&res.ChunkSequence,
			&res.ChunkText,
			&res.ConceptID,
			&res.ConceptText,
			&distance,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan search result with word count: %w", err)
		}
		res.SimilarityScore = 1.0 - distance
		results = append(results, res)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating through search results with word count: %w", err)
	}

	return results, nil
}

// --- Helper Functions (Dataset, Session, Token - Assuming unchanged) ---

func (pg *PostgresDB) GetOrCreateDataset(ctx context.Context, datasetName string, userId int64) (int64, error) {
	if datasetName == "" {
		return 0, errors.New("dataset name cannot be empty")
	}
	// Removed timeout context from here - prefer passing it down
	// ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	// defer cancel()

	// Use a slightly simpler query for get-or-create (Postgres 9.5+)
	query := `
        INSERT INTO datasets (name, user_id)
        VALUES ($1, $2)
        ON CONFLICT (name, user_id) DO UPDATE SET name = EXCLUDED.name -- Dummy update to get ID
        RETURNING id;
    `
	// Note: ON CONFLICT DO NOTHING doesn't return ID if row exists in older PG versions.
	// DO UPDATE guarantees RETURNING id works correctly.

	var id int64
	err := pg.db.QueryRowContext(ctx, query, datasetName, userId).Scan(&id)
	if err != nil {
		// If you really need to distinguish between insert and conflict-select,
		// you might need the original CTE approach or separate SELECT after INSERT...ON CONFLICT DO NOTHING.
		// This combined approach is usually sufficient.
		return 0, fmt.Errorf("failed to get or create dataset id: %w", err)
	}

	return id, nil
}

// GetSession, SaveSession, GetAccessTokens remain the same as your original code
// (assuming models.ChatSession, models.AccessToken, and sessions/access_tokens tables are unchanged)
// ... (Include the original GetSession, SaveSession, GetAccessTokens functions here) ...
// GetSession retrieves a chat session by its ID
func (pg *PostgresDB) GetSession(ctx context.Context, id string) (*models.ChatSession, error) {
	if id == "" {
		return nil, errors.New("session ID cannot be empty")
	}

	query := `
        SELECT id, messages, model
        FROM sessions
        WHERE id = $1
    `

	var session models.ChatSession
	var messages pq.StringArray // Use pq.StringArray directly

	err := pg.db.QueryRowContext(ctx, query, id).Scan(&session.ID, &messages, &session.Model)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Return nil, nil to indicate not found, common pattern
			return nil, nil
			// return nil, fmt.Errorf("session with ID %s not found", id) // Or return specific error
		}
		return nil, fmt.Errorf("failed to retrieve session: %w", err)
	}

	session.Messages = messages // Assign the scanned slice
	return &session, nil
}

// SaveSession saves or updates a chat session.
func (pg *PostgresDB) SaveSession(ctx context.Context, session models.ChatSession) error {
	if session.ID == "" {
		return errors.New("session ID cannot be empty")
	}

	messages := pq.StringArray(session.Messages)

	query := `
        INSERT INTO sessions (id, messages, model)
        VALUES ($1, $2, $3)
        ON CONFLICT (id) DO UPDATE
        SET messages = EXCLUDED.messages,
            model = EXCLUDED.model,
            updated_at = NOW() -- Also update timestamp on update
    ` // Added updated_at assuming you have it in sessions table

	_, err := pg.db.ExecContext(ctx, query, session.ID, messages, session.Model)
	if err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}

	return nil
}

func (pg *PostgresDB) GetAccessTokens(ctx context.Context) ([]models.AccessToken, error) {
	query := `
        SELECT user_id, token, expiration
        FROM access_tokens
        WHERE expiration > NOW();
    `

	rows, err := pg.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query for access tokens: %w", err)
	}
	defer rows.Close()

	var accessTokens []models.AccessToken
	for rows.Next() {
		var accessToken models.AccessToken
		err := rows.Scan(&accessToken.UserID, &accessToken.Token, &accessToken.Expiration)
		if err != nil {
			// Return accumulated tokens and the error? Or fail completely?
			// Failing completely is often safer.
			return nil, fmt.Errorf("failed to scan access token: %w", err)
		}
		accessTokens = append(accessTokens, accessToken)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating through access tokens: %w", err)
	}

	return accessTokens, nil
}

// --- Deprecated Functions ---

/*
// AddDocument is DEPRECATED. Use AddDocumentAndContent instead.
func (pg *PostgresDB) AddDocument(doc models.Document) error {
    return errors.New("AddDocument is deprecated; use AddDocumentAndContent")
}

// SimpleSearchDocuments is DEPRECATED. Use SearchConcepts instead.
func (pg *PostgresDB) SimpleSearchDocuments(queryVector []float32, datasetName string, userId int64, maxResults int) ([]models.Document, error) {
    return nil, errors.New("SimpleSearchDocuments is deprecated; use SearchConcepts")
}

// SearchDocuments is DEPRECATED. Use SearchConceptsWithWordCount instead.
func (pg *PostgresDB) SearchDocuments(queryVector []float32, datasetName string, userId int64, maxTotalWordCount int) ([]models.Document, error) {
    return nil, errors.New("SearchDocuments is deprecated; use SearchConceptsWithWordCount")
}
*/
