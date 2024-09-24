package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/mrhollen/KnowledgeGPT/internal/db"
	"github.com/mrhollen/KnowledgeGPT/internal/llm"
)

type QueryHandler struct {
	DB    db.DB
	LLM   llm.Client
	Limit int
}

type QueryRequest struct {
	Query     string `json:"query"`
	SessionID string `json:"session_id"`
	Limit     *int   `json:"limit,omitempty"`
}

type QueryResponse struct {
	Response string `json:"response"`
}

func (h *QueryHandler) Query(w http.ResponseWriter, r *http.Request) {
	var req QueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	if req.Query == "" {
		http.Error(w, "Query cannot be empty", http.StatusBadRequest)
		return
	}

	limit := h.Limit
	if req.Limit != nil {
		limit = *req.Limit
	}

	// Search documents
	docs, err := h.DB.SearchDocuments(req.Query, limit)
	if err != nil {
		http.Error(w, "Failed to search documents", http.StatusInternalServerError)
		return
	}

	// Prepend documents to the prompt
	prompt := ""
	for _, doc := range docs {
		prompt += fmt.Sprintf("Title: %s\nURL: %s\nContent: %s\n\n", doc.Title, doc.URL, doc.Body)
	}
	prompt += req.Query

	// Send to LLM
	response, err := h.LLM.SendPrompt(prompt)
	if err != nil {
		http.Error(w, "Failed to get response from LLM", http.StatusInternalServerError)
		return
	}

	// Optionally, save the response to the session
	// This requires accessing the session, which can be handled separately

	res := QueryResponse{
		Response: response,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}