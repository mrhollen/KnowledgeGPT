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

func main() {
	// Load environment variables from .env file if it exists
	envPath := ".env"
	if _, err := os.Stat(envPath); err == nil {
		if err := utils.LoadDotenv(envPath); err != nil {
			log.Fatalf("Error loading .env file: %v", err)
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

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "knowledgegpt.db" // Default value
	}

	dbConnectionString := os.Getenv("DB_CONNECTION_STRING")

	// Initialize Database
	database, err := db.NewPostgresDB(dbConnectionString)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
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
		Limit: 5,
	}

	// Register Routes
	http.HandleFunc("/documents", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			isAuthorized, err := checkAccessToken(r, accessTokenAuthorizer)
			if !isAuthorized || err != nil {
				http.Error(w, "", http.StatusUnauthorized)
				return
			}

			docHandler.AddDocument(w, r)
			return
		}
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	})

	http.HandleFunc("/query", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			isAuthorized, err := checkAccessToken(r, accessTokenAuthorizer)
			if !isAuthorized || err != nil {
				if err != nil {
					fmt.Println(err)
				}

				http.Error(w, "", http.StatusUnauthorized)
				return
			}

			queryHandler.Query(w, r)
			return
		}
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	})

	addressAndPort := fmt.Sprintf("%s:%s", os.Getenv("IP_ADDRESS"), os.Getenv("PORT"))

	// Start Server
	log.Printf("KnowledgeGPT server is running on %s\n", addressAndPort)
	if err := http.ListenAndServe(addressAndPort, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func checkAccessToken(r *http.Request, accessTokenAuthorizer *auth.AccessTokenAuthorizer) (bool, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return false, fmt.Errorf("authorization header is missing")
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == authHeader {
		return false, fmt.Errorf("invalid Authorization header format")
	}

	return accessTokenAuthorizer.CheckToken(token)
}
