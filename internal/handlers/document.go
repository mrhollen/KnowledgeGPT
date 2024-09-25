package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/mrhollen/KnowledgeGPT/internal/db"
	"github.com/mrhollen/KnowledgeGPT/internal/llm"
	"github.com/mrhollen/KnowledgeGPT/internal/models"
)

type DocumentHandler struct {
	Client llm.Client
	DB     db.DB
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

	vec, err := h.Client.GetEmbedding(req.Body, "")
	if err != nil {
		fmt.Println(err)
		http.Error(w, "Could not get document embedding", http.StatusInternalServerError)
		return
	}

	doc := models.Document{
		Title: req.Title,
		URL:   req.URL,
		Body:  req.Body,
		Vec:   vec,
	}

	if err := h.DB.AddDocument(doc); err != nil {
		http.Error(w, "Failed to add document", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}
