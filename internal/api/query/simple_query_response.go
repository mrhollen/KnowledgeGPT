package api

type SimpleQueryResponseContentAPI struct {
	ConceptText   string  `json:"concept_text"`
	ChunkText     string  `json:"chunk_text"` // Include chunk for context
	DocumentTitle string  `json:"document_title"`
	DocumentURL   *string `json:"document_url,omitempty"`
	Score         float64 `json:"score"` // Similarity score
	ChunkID       int64   `json:"chunk_id"`
	DocumentID    int64   `json:"document_id"`
}
type SimpleQueryResponseAPI struct {
	Results []SimpleQueryResponseContentAPI `json:"results"`
}
