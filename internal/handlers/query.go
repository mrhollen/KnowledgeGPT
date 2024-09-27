package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	api "github.com/mrhollen/KnowledgeGPT/internal/api/query"
	"github.com/mrhollen/KnowledgeGPT/internal/db"
	"github.com/mrhollen/KnowledgeGPT/internal/llm"
)

type QueryHandler struct {
	DB    db.DB
	LLM   llm.Client
	Limit int
}

func (h *QueryHandler) Query(w http.ResponseWriter, r *http.Request) {
	var req api.QueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	if req.Query == "" {
		http.Error(w, "Query cannot be empty", http.StatusBadRequest)
		return
	}

	queryVector, err := h.LLM.GetEmbedding(req.Query, req.Model)
	if err != nil {
		http.Error(w, "Could not generate query embedding", http.StatusInternalServerError)
	}

	limit := h.Limit
	if req.Limit != nil {
		limit = *req.Limit
	}
	datasetName := req.Dataset
	if datasetName == "" {
		datasetName = "default"
	}

	docs, err := h.DB.SearchDocuments(queryVector, datasetName, limit)
	if err != nil {
		fmt.Println(err)
		http.Error(w, "Failed to search documents", http.StatusInternalServerError)
		return
	}

	prompt := "Search results: \n"
	if len(docs) < 1 {
		prompt += "No results \n\n"
	}

	for _, doc := range docs {
		prompt += fmt.Sprintf("```\nTitle: %s\nURL: %s\nContent: %s\n```\n\n", doc.Title, doc.URL, doc.Body)
	}
	prompt += req.Query

	response, err := h.LLM.SendPrompt(prompt, req.Model)
	if err != nil {
		http.Error(w, "Failed to get response from LLM", http.StatusInternalServerError)
		return
	}

	res := api.QueryResponse{
		Response: response,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}
