package entity

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// EmbeddingVector stores a dense embedding vector as PostgreSQL jsonb
// so the service runs on any vanilla PostgreSQL instance (no pgvector
// extension required). Similarity is computed in the application layer.
type EmbeddingVector []float64

func (v EmbeddingVector) Value() (driver.Value, error) {
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}

func (v *EmbeddingVector) Scan(value interface{}) error {
	if value == nil {
		*v = nil
		return nil
	}

	switch data := value.(type) {
	case []byte:
		return json.Unmarshal(data, v)
	case string:
		return json.Unmarshal([]byte(data), v)
	default:
		return fmt.Errorf("unsupported type for EmbeddingVector: %T", value)
	}
}

const (
	SourceTypeText  = "text"
	SourceTypeFile  = "file"
	SourceTypeImage = "image"

	DocumentStatusReady  = "ready"
	DocumentStatusFailed = "failed"
)

type Document struct {
	ID          string    `gorm:"column:id;type:uuid;primaryKey" json:"id"`
	Title       string    `gorm:"column:title" json:"title"`
	SourceType  string    `gorm:"column:source_type" json:"source_type"`
	FileName    string    `gorm:"column:file_name" json:"file_name"`
	MimeType    string    `gorm:"column:mime_type" json:"mime_type"`
	Description string    `gorm:"column:description" json:"description"`
	Status      string    `gorm:"column:status" json:"status"`
	ChunkCount  int       `gorm:"column:chunk_count" json:"chunk_count"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (Document) TableName() string { return "documents" }

type DocumentChunk struct {
	ID         string          `gorm:"column:id;type:uuid;primaryKey" json:"id"`
	DocumentID string          `gorm:"column:document_id;type:uuid;index" json:"document_id"`
	ChunkIndex int             `gorm:"column:chunk_index" json:"chunk_index"`
	Content    string          `gorm:"column:content" json:"content"`
	Embedding  EmbeddingVector `gorm:"column:embedding;type:jsonb" json:"-"`
	CreatedAt  time.Time       `gorm:"column:created_at;autoCreateTime" json:"created_at"`
}

func (DocumentChunk) TableName() string { return "document_chunks" }

// ChunkDocument is a read model that joins a chunk with its parent
// document, used during RAG retrieval.
type ChunkDocument struct {
	ID         string          `gorm:"column:id"`
	DocumentID string          `gorm:"column:document_id"`
	Title      string          `gorm:"column:title"`
	Content    string          `gorm:"column:content"`
	Embedding  EmbeddingVector `gorm:"column:embedding"`
}
