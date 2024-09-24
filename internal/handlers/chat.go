package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/mrhollen/KnowledgeGPT/internal/db"
	"github.com/mrhollen/KnowledgeGPT/internal/llm"
	"github.com/mrhollen/KnowledgeGPT/pkg/utils"
)

type ChatHandler struct {
	DB  db.DB
	LLM llm.Client
}

type ChatRequest struct {
	Query     string `json:"query"`
	SessionID string `json:"session_id,omitempty"`
}

type ChatResponse struct {
	SessionID string `json:"session_id"`
	Response  string `json:"response"`
}

func (h *ChatHandler) Chat(w http.ResponseWriter, r *http.Request) {
	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	if req.Query == "" {
		http.Error(w, "Query cannot be empty", http.StatusBadRequest)
		return
	}

	sessionID := req.SessionID
	if sessionID == "" {
		var err error

		sessionID, err = utils.GenerateUUID()
		if err != nil {
			http.Error(w, "Failed to generate session ID", http.StatusInternalServerError)
			return
		}
	}

	// Retrieve or create session
	session, err := h.DB.GetSession(sessionID)
	if err != nil {
		http.Error(w, "Failed to retrieve session", http.StatusInternalServerError)
		return
	}

	// Append user query to session
	session.Messages = append(session.Messages, "User: "+req.Query)

	// Prepend session messages to the prompt
	prompt := ""
	for _, msg := range session.Messages {
		prompt += msg + "\n"
	}
	prompt += "AI:"

	// Send to LLM
	response, err := h.LLM.SendPrompt(prompt)
	if err != nil {
		http.Error(w, "Failed to get response from LLM", http.StatusInternalServerError)
		return
	}

	// Append AI response to session
	session.Messages = append(session.Messages, "AI: "+response)

	// Save session
	if err := h.DB.SaveSession(*session); err != nil {
		http.Error(w, "Failed to save session", http.StatusInternalServerError)
		return
	}

	res := ChatResponse{
		SessionID: sessionID,
		Response:  response,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}
