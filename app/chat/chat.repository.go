package chat

import (
	"tourism-village-chatbot/entity"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type ChatRepository interface {
	CreateSession(db *gorm.DB, session *entity.ChatSession) error
	FindSessionByID(db *gorm.DB, id string) (*entity.ChatSession, error)
	UpdateSession(db *gorm.DB, session *entity.ChatSession) error
	CreateMessage(db *gorm.DB, message *entity.ChatMessage) error
	FindMessagesBySessionID(db *gorm.DB, sessionID string, limit int) ([]entity.ChatMessage, error)
	FindChunkDocuments(db *gorm.DB) ([]entity.ChunkDocument, error)
}

type chatRepository struct {
	Log *logrus.Logger
}

func NewChatRepository(log *logrus.Logger) ChatRepository {
	return &chatRepository{
		Log: log,
	}
}

func (r *chatRepository) CreateSession(db *gorm.DB, session *entity.ChatSession) error {
	return db.Create(session).Error
}

func (r *chatRepository) FindSessionByID(db *gorm.DB, id string) (*entity.ChatSession, error) {
	session := new(entity.ChatSession)
	err := db.Where("id = ?", id).First(session).Error
	if err != nil {
		return nil, err
	}
	return session, nil
}

func (r *chatRepository) UpdateSession(db *gorm.DB, session *entity.ChatSession) error {
	return db.Save(session).Error
}

func (r *chatRepository) CreateMessage(db *gorm.DB, message *entity.ChatMessage) error {
	return db.Create(message).Error
}

func (r *chatRepository) FindMessagesBySessionID(db *gorm.DB, sessionID string, limit int) ([]entity.ChatMessage, error) {
	if limit <= 0 {
		limit = 10
	}

	var messages []entity.ChatMessage
	err := db.Where("session_id = ?", sessionID).Order("created_at DESC").Limit(limit).Find(&messages).Error
	if err != nil {
		return nil, err
	}

	// Reverse so the history is chronological.
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}

func (r *chatRepository) FindChunkDocuments(db *gorm.DB) ([]entity.ChunkDocument, error) {
	var chunks []entity.ChunkDocument
	err := db.Raw(`
		SELECT c.id, c.document_id, d.title, c.content, c.embedding
		FROM document_chunks c
		JOIN documents d ON d.id = c.document_id
		WHERE d.status = ?
		ORDER BY c.created_at ASC
	`, entity.DocumentStatusReady).Scan(&chunks).Error
	return chunks, err
}
