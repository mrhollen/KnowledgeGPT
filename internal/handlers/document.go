package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	api "github.com/mrhollen/KnowledgeGPT/internal/api/documents"
	"github.com/mrhollen/KnowledgeGPT/internal/db"
	"github.com/mrhollen/KnowledgeGPT/internal/llm"
	"github.com/mrhollen/KnowledgeGPT/internal/models"
)

type DocumentHandler struct {
	Client llm.Client
	DB     *db.PostgresDB
}

func (h *DocumentHandler) AddDocument(userId int64, w http.ResponseWriter, r *http.Request) {
	var req api.AddDocumentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	vec, err := h.Client.GetEmbedding(req.Body, "")
	if err != nil {
		fmt.Println(err)
		http.Error(w, "Could not get document embedding", http.StatusInternalServerError)
		return
	}

	datasetName := req.Dataset
	if datasetName == "" {
		datasetName = "default"
	}

	datasetId, err := h.DB.GetOrCreateDataset(datasetName, userId)
	if err != nil {
		fmt.Println(err)
		http.Error(w, "Error getting or creating dataset", http.StatusInternalServerError)
		return
	}

	doc := models.Document{
		Title:     req.Title,
		URL:       req.URL,
		Body:      req.Body,
		Vec:       vec,
		DatasetID: datasetId,
	}

	if err := h.DB.AddDocument(doc); err != nil {
		fmt.Println(err)
		http.Error(w, "Failed to add document", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}
