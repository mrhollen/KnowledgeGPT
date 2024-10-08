package api

type SimpleQueryRequest struct {
	Query   string `json:"query"`
	Limit   int    `json:"limit"`
	Dataset string `json:"dataset"`
}
