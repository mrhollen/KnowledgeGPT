package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	api "github.com/mrhollen/KnowledgeGPT/internal/api/query"
	"github.com/mrhollen/KnowledgeGPT/internal/db"
	"github.com/mrhollen/KnowledgeGPT/internal/llm"
)

type QueryHandler struct {
	DB    *db.PostgresDB
	LLM   llm.Client
	Limit int
}

func (h *QueryHandler) SimpleQuery(userId int64, w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	queryString := query.Get("query")
	limit := query.Get("limit")
	dataset := query.Get("dataset")

	limitNum := 5

	if queryString == "" {
		http.Error(w, "No query", http.StatusBadRequest)
		return
	}
	if dataset == "" {
		dataset = "default"
	}
	if limit != "" {
		var err error
		limitNum, err = strconv.Atoi(limit)
		if err != nil {
			limitNum = 5
		}
	}

	request := api.SimpleQueryRequest{
		Query:   queryString,
		Limit:   limitNum,
		Dataset: dataset,
	}

	queryVector, err := h.LLM.GetEmbedding(request.Query, "")
	if err != nil {
		http.Error(w, "Could not generate query embedding", http.StatusInternalServerError)
		return
	}

	docs, err := h.DB.SimpleSearchDocuments(queryVector, request.Dataset, userId, request.Limit)
	if err != nil {
		fmt.Println(err)
		http.Error(w, "Failed to search documents", http.StatusInternalServerError)
		return
	}

	response := api.SimpleQueryResponse{
		Responses: []api.SimpleQueryResponseContent{},
	}

	for _, doc := range docs {
		response.Responses = append(
			response.Responses,
			api.SimpleQueryResponseContent{
				Title: doc.Title,
				URL:   doc.URL,
				Text:  doc.Body,
			},
		)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *QueryHandler) QueryWithLLM(userId int64, w http.ResponseWriter, r *http.Request) {
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

	docs, err := h.DB.SearchDocuments(queryVector, datasetName, userId, limit)
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
		docJson, err := json.Marshal(doc)
		if err != nil {
			fmt.Println("Error marshaling to JSON:", err)
			return
		}

		prompt += fmt.Sprintf("```json\n%s\n```\n\n", docJson)
	}
	prompt += req.Query

	response, err := h.LLM.SendPrompt(prompt, req.Model)
	if err != nil {
		http.Error(w, "Failed to get response from LLM", http.StatusInternalServerError)
		return
	}

	re := regexp.MustCompile(`\[citation\](\d+)\[/citation\]`)
	replacedText := re.ReplaceAllStringFunc(response, func(match string) string {
		submatches := re.FindStringSubmatch(match)
		if len(submatches) != 2 {
			return match
		}

		var id int64
		fmt.Sscanf(submatches[1], "%d", &id)

		for _, doc := range docs {
			if doc.ID == id {
				return fmt.Sprintf("[%s](%s)", doc.Title, doc.URL)
			}
		}

		return match
	})

	res := api.QueryResponse{
		Response: strings.ReplaceAll(replacedText, "\\n", "\n"),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}
