package entity

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

const (
	MessageRoleUser      = "user"
	MessageRoleAssistant = "assistant"
)

type ChatSession struct {
	ID        string    `gorm:"column:id;type:uuid;primaryKey" json:"id"`
	UserID    string    `gorm:"column:user_id" json:"user_id"`
	Title     string    `gorm:"column:title" json:"title"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (ChatSession) TableName() string { return "chat_sessions" }

// MessageSource records which knowledge chunks were used to produce an
// answer, so the client can show citations.
type MessageSource struct {
	ChunkID    string  `json:"chunk_id"`
	DocumentID string  `json:"document_id"`
	Title      string  `json:"title"`
	Score      float64 `json:"score"`
}

type SourceList []MessageSource

func (s SourceList) Value() (driver.Value, error) {
	if s == nil {
		return nil, nil
	}
	return json.Marshal(s)
}

func (s *SourceList) Scan(value interface{}) error {
	if value == nil {
		*s = nil
		return nil
	}

	switch data := value.(type) {
	case []byte:
		return json.Unmarshal(data, s)
	case string:
		return json.Unmarshal([]byte(data), s)
	default:
		return fmt.Errorf("unsupported type for SourceList: %T", value)
	}
}

type ChatMessage struct {
	ID        string     `gorm:"column:id;type:uuid;primaryKey" json:"id"`
	SessionID string     `gorm:"column:session_id;type:uuid;index" json:"session_id"`
	Role      string     `gorm:"column:role" json:"role"`
	Content   string     `gorm:"column:content" json:"content"`
	Sources   SourceList `gorm:"column:sources;type:jsonb" json:"sources"`
	LatencyMs int64      `gorm:"column:latency_ms" json:"latency_ms"`
	CreatedAt time.Time  `gorm:"column:created_at;autoCreateTime" json:"created_at"`
}

func (ChatMessage) TableName() string { return "chat_messages" }
