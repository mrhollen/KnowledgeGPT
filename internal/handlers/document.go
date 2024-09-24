package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/mrhollen/KnowledgeGPT/internal/db"
	"github.com/mrhollen/KnowledgeGPT/internal/models"
)

type DocumentHandler struct {
	DB db.DB
}

type AddDocumentRequest struct {
	Title string `json:"title"`
	URL   string `json:"url,omitempty"`
	Body  string `json:"body"`
}

func (h *DocumentHandler) AddDocument(w http.ResponseWriter, r *http.Request) {
	var req AddDocumentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	doc := models.Document{
		Title: req.Title,
		URL:   req.URL,
		Body:  req.Body,
	}

	if err := h.DB.AddDocument(doc); err != nil {
		http.Error(w, "Failed to add document", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}
