package knowledge

import (
	"time"

	"support-agent-golang/entity"
)

type IngestTextRequest struct {
	Title   string `json:"title" validate:"required"`
	Content string `json:"content" validate:"required"`
}

type SearchRequest struct {
	Query string `json:"query" validate:"required"`
	TopK  int    `json:"top_k"`
}

type DocumentResponse struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	SourceType  string    `json:"source_type"`
	FileName    string    `json:"file_name"`
	MimeType    string    `json:"mime_type"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	ChunkCount  int       `json:"chunk_count"`
	CreatedAt   time.Time `json:"created_at"`
}

type SearchHit struct {
	ChunkID    string  `json:"chunk_id"`
	DocumentID string  `json:"document_id"`
	Title      string  `json:"title"`
	Content    string  `json:"content"`
	Score      float64 `json:"score"`
}

func ToDocumentResponse(document entity.Document) DocumentResponse {
	return DocumentResponse{
		ID:          document.ID,
		Title:       document.Title,
		SourceType:  document.SourceType,
		FileName:    document.FileName,
		MimeType:    document.MimeType,
		Description: document.Description,
		Status:      document.Status,
		ChunkCount:  document.ChunkCount,
		CreatedAt:   document.CreatedAt,
	}
}
