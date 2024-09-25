package db

import (
	"database/sql"
	"strings"

	"github.com/mrhollen/KnowledgeGPT/internal/models"
	_ "modernc.org/sqlite"
)

type SQLiteDB struct {
	conn *sql.DB
}

func NewSQLiteDB(dataSourceName string) (*SQLiteDB, error) {
	db, err := sql.Open("sqlite", dataSourceName)
	if err != nil {
		return nil, err
	}

	// Initialize tables
	queries := []string{
		`CREATE TABLE IF NOT EXISTS documents (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            title TEXT NOT NULL,
            url TEXT,
            body TEXT NOT NULL
        );`,
		`CREATE TABLE IF NOT EXISTS sessions (
            id TEXT PRIMARY KEY,
            messages TEXT
        );`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS fts_documents USING 
			fts5(id UNINDEXED, 
			title, 
			url UNINDEXED, 
			body
		);`,
	}

	for _, q := range queries {
		if _, err := db.Exec(q); err != nil {
			return nil, err
		}
	}

	return &SQLiteDB{conn: db}, nil
}

func (s *SQLiteDB) AddDocument(doc models.Document) error {
	query := `INSERT INTO documents (title, url, body) VALUES (?, ?, ?)`
	_, err := s.conn.Exec(query, doc.Title, doc.URL, doc.Body)
	return err
}

func (s *SQLiteDB) SearchDocuments(queryStr string, limit int) ([]models.Document, error) {
	query := `SELECT id, title, url, body FROM fts_documents WHERE body MATCH ? ORDER BY rank LIMIT ?`
	escapedQueryString := strings.ReplaceAll(queryStr, "?", "")

	rows, err := s.conn.Query(query, escapedQueryString, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var docs []models.Document
	for rows.Next() {
		var doc models.Document
		if err := rows.Scan(&doc.ID, &doc.Title, &doc.URL, &doc.Body); err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}
	return docs, nil
}

func (s *SQLiteDB) GetSession(id string) (*models.ChatSession, error) {
	query := `SELECT messages FROM sessions WHERE id = ?`
	var messages string
	err := s.conn.QueryRow(query, id).Scan(&messages)
	if err != nil {
		if err == sql.ErrNoRows {
			return &models.ChatSession{
				ID:       id,
				Messages: []string{},
			}, nil
		}
		return nil, err
	}

	// Assuming messages are stored as newline-separated strings
	msgList := []string{}
	if messages != "" {
		msgList = append(msgList, messages)
	}

	return &models.ChatSession{
		ID:       id,
		Messages: msgList,
	}, nil
}

func (s *SQLiteDB) SaveSession(session models.ChatSession) error {
	// Serialize messages
	messages := ""
	for _, msg := range session.Messages {
		messages += msg + "\n"
	}

	// Upsert
	query := `
    INSERT INTO sessions (id, messages) VALUES (?, ?)
    ON CONFLICT(id) DO UPDATE SET messages=excluded.messages
    `
	_, err := s.conn.Exec(query, session.ID, messages)
	return err
}
