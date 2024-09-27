package api

type QueryRequest struct {
	Query     string `json:"query"`
	SessionID string `json:"session_id"`
	Limit     *int   `json:"limit,omitempty"`
	Model     string `json:"model"`
	Dataset   string `json:"dataset"`
}
