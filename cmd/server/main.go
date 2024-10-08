// cmd/server/main.go
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/mrhollen/KnowledgeGPT/internal/auth"
	"github.com/mrhollen/KnowledgeGPT/internal/db"
	"github.com/mrhollen/KnowledgeGPT/internal/handlers"
	"github.com/mrhollen/KnowledgeGPT/internal/llm"
	"github.com/mrhollen/KnowledgeGPT/pkg/utils"
)

// Server encapsulates the dependencies for the HTTP server
type Server struct {
	Database              *db.PostgresDB
	LLMClient             *llm.OpenAIClient
	AccessTokenAuthorizer *auth.AccessTokenAuthorizer
	DocumentHandler       *handlers.DocumentHandler
	QueryHandler          *handlers.QueryHandler
	UploadHandler         *handlers.UploadHandler
}

func main() {
	// Initialize the server with all dependencies
	server, err := initializeServer()
	if err != nil {
		log.Fatalf("Failed to initialize server: %v", err)
	}

	// Register all HTTP routes
	server.registerRoutes()

	// Construct the server address from environment variables
	addressAndPort := fmt.Sprintf("%s:%s", os.Getenv("IP_ADDRESS"), os.Getenv("PORT"))

	// Start the HTTP server
	log.Printf("KnowledgeGPT server is running on %s\n", addressAndPort)
	if err := http.ListenAndServe(addressAndPort, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

// initializeServer sets up the server with all necessary dependencies
func initializeServer() (*Server, error) {
	// Load environment variables from .env file if it exists
	envPath := ".env"
	if _, err := os.Stat(envPath); err == nil {
		if err := utils.LoadDotenv(envPath); err != nil {
			return nil, fmt.Errorf("error loading .env file: %w", err)
		}
		log.Println(".env file loaded successfully")
	} else {
		log.Println(".env file not found, proceeding with existing environment variables")
	}

	// Retrieve configuration from environment variables
	llmEndpoint := os.Getenv("LLM_ENDPOINT")
	llmEmbeddingEndpoint := os.Getenv("LLM_EMBEDDING_ENDPOINT")
	llmAPIKey := os.Getenv("LLM_API_KEY")
	llmDefaultModel := os.Getenv("LLM_DEFAULT_MODEL")
	if llmDefaultModel == "" {
		llmDefaultModel = "text-davinci-003"
	}

	dbConnectionString := os.Getenv("DB_CONNECTION_STRING")

	// Initialize Database
	database, err := db.NewPostgresDB(dbConnectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Initialize LLM Client
	llmClient := llm.NewOpenAIClient(llmEndpoint, llmEmbeddingEndpoint, llmAPIKey, llmDefaultModel)

	// Initialize Authorization
	accessTokenAuthorizer := auth.NewAccessTokenAuthorizer(database)

	// Initialize Handlers
	docHandler := &handlers.DocumentHandler{
		Client: llmClient,
		DB:     database,
	}
	queryHandler := &handlers.QueryHandler{
		DB:    database,
		LLM:   llmClient,
		Limit: 512,
	}
	uploadHandler := &handlers.UploadHandler{}

	// Create and return the Server instance
	return &Server{
		Database:              database,
		LLMClient:             llmClient,
		AccessTokenAuthorizer: accessTokenAuthorizer,
		DocumentHandler:       docHandler,
		QueryHandler:          queryHandler,
		UploadHandler:         uploadHandler,
	}, nil
}

// registerRoutes sets up all the HTTP routes with their respective handlers
func (s *Server) registerRoutes() {
	http.HandleFunc("/documents", s.enableCORS(s.handleDocuments))
	http.HandleFunc("/bulk/documents", s.enableCORS(s.handleBulkDocuments))
	http.HandleFunc("/query", s.enableCORS(s.handleQuery))
	http.HandleFunc("/upload", s.enableCORS(s.handleUpload))
}

// enableCORS is a middleware that adds CORS headers to the response
func (s *Server) enableCORS(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		// Handle preflight requests
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Proceed to the next handler
		next.ServeHTTP(w, r)
	}
}

// handleDocuments handles requests to the /documents endpoint
func (s *Server) handleDocuments(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		isAuthorized, userId, err := s.checkAccessToken(r)
		if !isAuthorized || err != nil {
			if err != nil {
				log.Println(err)
			}
			http.Error(w, "", http.StatusUnauthorized)
			return
		}

		s.DocumentHandler.AddDocument(userId, w, r)
		return
	}
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// handleBulkDocuments handles requests to the /bulk/documents endpoint
func (s *Server) handleBulkDocuments(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		isAuthorized, userId, err := s.checkAccessToken(r)
		if !isAuthorized || err != nil {
			if err != nil {
				log.Println(err)
			}
			http.Error(w, "", http.StatusUnauthorized)
			return
		}

		s.DocumentHandler.AddDocuments(userId, w, r)
		return
	}
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// handleQuery handles requests to the /query endpoint
func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		isAuthorized, userId, err := s.checkAccessToken(r)
		if !isAuthorized || err != nil {
			if err != nil {
				log.Println(err)
			}
			http.Error(w, "", http.StatusUnauthorized)
			return
		}
		s.QueryHandler.SimpleQuery(userId, w, r)
	case http.MethodPost:
		isAuthorized, userId, err := s.checkAccessToken(r)
		if !isAuthorized || err != nil {
			if err != nil {
				log.Println(err)
			}
			http.Error(w, "", http.StatusUnauthorized)
			return
		}
		s.QueryHandler.QueryWithLLM(userId, w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleUpload handles requests to the /upload endpoint
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		isAuthorized, userId, err := s.checkAccessToken(r)
		if !isAuthorized || err != nil {
			if err != nil {
				log.Println(err)
			}
			http.Error(w, "", http.StatusUnauthorized)
			return
		}

		s.UploadHandler.UploadFile(userId, w, r)
		return
	}
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// checkAccessToken verifies the Authorization header and validates the token
func (s *Server) checkAccessToken(r *http.Request) (bool, int64, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return false, 0, fmt.Errorf("authorization header is missing")
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == authHeader {
		return false, 0, fmt.Errorf("invalid Authorization header format")
	}

	return s.AccessTokenAuthorizer.CheckToken(token)
}
