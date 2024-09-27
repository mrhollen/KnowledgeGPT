package api

type AddDocumentRequest struct {
	Title   string `json:"title"`
	URL     string `json:"url,omitempty"`
	Body    string `json:"body"`
	Dataset string `json:"dataset"`
}
