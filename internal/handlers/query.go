package handlers

import (
	"context" // Added
	"encoding/json"
	"fmt"
	"log" // Added
	"net/http"
	"os"
	"strconv"
	"strings"
	"time" // Added for context timeouts

	api "github.com/mrhollen/KnowledgeGPT/internal/api/query"
	"github.com/mrhollen/KnowledgeGPT/internal/db"
	"github.com/mrhollen/KnowledgeGPT/internal/llm"
)

type QueryHandler struct {
	DB    *db.PostgresDB
	LLM   llm.Client
	Limit int // Default word count limit for QueryWithLLM
}

func (h *QueryHandler) SimpleQuery(userId int64, w http.ResponseWriter, r *http.Request) {
	ctx := r.Context() // Get context from request

	query := r.URL.Query()
	queryString := query.Get("query")
	limitStr := query.Get("limit")
	dataset := query.Get("dataset")

	if queryString == "" {
		http.Error(w, `{"error": "Query parameter 'query' is required"}`, http.StatusBadRequest)
		return
	}
	if dataset == "" {
		dataset = "default" // Default dataset name
	}

	limitNum := 10 // Default number of concepts/results
	if limitStr != "" {
		var err error
		limitNum, err = strconv.Atoi(limitStr)
		if err != nil || limitNum <= 0 {
			http.Error(w, fmt.Sprintf(`{"error": "Invalid limit parameter: %s"}`, limitStr), http.StatusBadRequest)
			return
		}
	}

	// Get query embedding
	embedCtx, cancel := context.WithTimeout(ctx, 30*time.Second)      // Timeout for embedding
	queryVector, err := h.LLM.GetEmbedding(embedCtx, queryString, "") // Pass context

	cancel() // Release context

	if err != nil {
		log.Printf("SimpleQuery: Could not generate query embedding: %v", err)
		http.Error(w, `{"error": "Failed to process query embedding"}`, http.StatusInternalServerError)
		return
	}

	// Search using the new DB method
	// Use context for DB call
	searchCtx, cancelSearch := context.WithTimeout(ctx, 15*time.Second) // Timeout for search
	results, err := h.DB.SearchConcepts(searchCtx, queryVector, dataset, userId, limitNum)
	cancelSearch() // Release context
	if err != nil {
		log.Printf("SimpleQuery: Failed to search concepts for dataset '%s': %v", dataset, err)
		http.Error(w, `{"error": "Failed to execute search"}`, http.StatusInternalServerError)
		return
	}

	response := api.SimpleQueryResponseAPI{
		Results: make([]api.SimpleQueryResponseContentAPI, 0, len(results)),
	}

	for _, res := range results {
		response.Results = append(response.Results, api.SimpleQueryResponseContentAPI{
			ConceptText:   res.ConceptText,
			ChunkText:     res.ChunkText,
			DocumentTitle: res.DocumentTitle,
			DocumentURL:   res.DocumentURL,
			Score:         res.SimilarityScore,
			ChunkID:       res.ChunkID,
			DocumentID:    res.DocumentID,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		// Log encoding error, but headers might be sent already
		log.Printf("SimpleQuery: Failed to encode response: %v", err)
	}
}

func (h *QueryHandler) QueryWithLLM(userId int64, w http.ResponseWriter, r *http.Request) {
	ctx := r.Context() // Get context from request

	var req api.QueryRequest // Assuming api.QueryRequest has Query, Dataset, Model, Limit (optional *int for word count)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid request payload"}`, http.StatusBadRequest)
		return
	}
	defer r.Body.Close() // Ensure body is closed

	if req.Query == "" {
		http.Error(w, `{"error": "Query cannot be empty"}`, http.StatusBadRequest)
		return
	}

	// Get query embedding
	embedCtx, cancel := context.WithTimeout(ctx, 30*time.Second)           // Timeout for embedding
	queryVector, err := h.LLM.GetEmbedding(embedCtx, req.Query, req.Model) // Use model from request if provided

	cancel()

	if err != nil {
		log.Printf("QueryWithLLM: Could not generate query embedding: %v", err)
		http.Error(w, `{"error": "Failed to process query embedding"}`, http.StatusInternalServerError)
		return
	}

	// Determine word count limit
	wordCountLimit := h.Limit // Use handler's default limit
	if req.Limit != nil && *req.Limit > 0 {
		wordCountLimit = *req.Limit // Override with request limit if provided and valid
	}
	if wordCountLimit <= 0 {
		wordCountLimit = 2000 // Fallback to a reasonable default if needed
	}

	datasetName := req.Dataset
	if datasetName == "" {
		datasetName = "default"
	}

	// Search using the new DB method with word count limit
	// Use context for DB call
	searchCtx, cancelSearch := context.WithTimeout(ctx, 20*time.Second) // Longer timeout for potentially complex search
	results, err := h.DB.SearchConceptsWithWordCount(searchCtx, queryVector, datasetName, userId, wordCountLimit)
	cancelSearch()

	if err != nil {
		log.Printf("QueryWithLLM: Failed to search concepts (word count) for dataset '%s': %v", datasetName, err)
		http.Error(w, `{"error": "Failed to execute search"}`, http.StatusInternalServerError)
		return
	}

	prompt := "Answer the following query based *only* on the provided search results. Do not use information not present in the search results.\n\nSearch results:\n"
	if len(results) == 0 {
		prompt += "No relevant information found in the knowledge base.\n\n"
	} else {
		for _, res := range results {
			prompt += fmt.Sprintf("```\nChunkID: %d\nDocument Title: %s\nDocument URL: %s\nText: %s\n```\n\n", res.ChunkID, res.DocumentTitle, *res.DocumentURL, res.ChunkText)
		}
	}
	prompt += "Query: " + req.Query

	systemPromptBytes, err := os.ReadFile("./system_prompt.txt")
	if err != nil {
		log.Printf("Warning: Could not read system_prompt.txt: %v. Using empty system prompt.", err)
	}
	systemPrompt := string(systemPromptBytes) // Will be empty if read failed

	llmCtx, cancelLLM := context.WithTimeout(ctx, 1000*time.Second) // Timeout for LLM generation
	llmResponse, err := h.LLM.SendPrompt(llmCtx, prompt, systemPrompt, req.Model)

	cancelLLM()

	if err != nil {
		log.Printf("QueryWithLLM: Failed to get response from LLM: %v", err)
		http.Error(w, `{"error": "Failed to get response from LLM"}`, http.StatusInternalServerError)
		return
	}

	res := api.QueryResponse{
		Response: strings.TrimSpace(llmResponse), // Clean up whitespace
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(res); err != nil {
		log.Printf("QueryWithLLM: Failed to encode response: %v", err)
	}
}
