package api

type SimpleQueryResponseContent struct {
	Title string `json:"title"`
	URL   string `json:"url"`
	Text  string `json:"text"`
}

type SimpleQueryResponse struct {
	Responses []SimpleQueryResponseContent `json:"responses"`
}
