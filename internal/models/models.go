package models

import "time"

type Document struct {
	ID        int64     `json:"id"`
	DatasetID int64     `json:"dataset_id"`
	Title     string    `json:"title"`
	URL       string    `json:"url,omitempty"`
	Body      string    `json:"body"`
	Vec       []float32 `json:"vector"`
}

type ChatSession struct {
	ID       string   `json:"id"`
	Messages []string `json:"messages"`
	Model    string   `json:"model"`
}

type User struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

type AccessToken struct {
	UserID     int64     `json:"id"`
	Token      string    `json:"token"`
	Expiration time.Time `json:"expiration"`
}
