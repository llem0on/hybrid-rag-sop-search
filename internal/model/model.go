package model

import "time"

type Document struct {
	ID         int64     `json:"id"`
	Number     string    `json:"number"`
	Title      string    `json:"title"`
	Department string    `json:"department"`
	FileName   string    `json:"file_name"`
	Chunks     int       `json:"chunks"`
	Figures    int       `json:"figures"`
	CreatedAt  time.Time `json:"created_at"`
}

type Chunk struct {
	ID         int64     `json:"id"`
	DocumentID int64     `json:"document_id"`
	Index      int       `json:"index"`
	Section    string    `json:"section"`
	PageStart  int       `json:"page_start"`
	PageEnd    int       `json:"page_end"`
	Content    string    `json:"content"`
	Tokens     int       `json:"tokens"`
	FigurePath string    `json:"figure_path,omitempty"`
	Vector     []float32 `json:"vector,omitempty"`
}
