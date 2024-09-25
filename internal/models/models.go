package models

type Document struct {
	ID    int64  `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url,omitempty"`
	Body  string `json:"body"`
}

type ChatSession struct {
	ID       string   `json:"id"`
	Messages []string `json:"messages"`
	Model    string   `json:"model"`
}
