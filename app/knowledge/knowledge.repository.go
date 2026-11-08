package knowledge

import (
	"tourism-village-chatbot/entity"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type KnowledgeRepository interface {
	CreateDocument(db *gorm.DB, document *entity.Document) error
	CreateChunks(db *gorm.DB, chunks []entity.DocumentChunk) error
	FindAllDocuments(db *gorm.DB) ([]entity.Document, error)
	FindDocumentByID(db *gorm.DB, id string) (*entity.Document, error)
	DeleteDocument(db *gorm.DB, documentID string) error
	FindChunkDocuments(db *gorm.DB) ([]entity.ChunkDocument, error)
}

type knowledgeRepository struct {
	Log *logrus.Logger
}

func NewKnowledgeRepository(log *logrus.Logger) KnowledgeRepository {
	return &knowledgeRepository{
		Log: log,
	}
}

func (r *knowledgeRepository) CreateDocument(db *gorm.DB, document *entity.Document) error {
	return db.Create(document).Error
}

func (r *knowledgeRepository) CreateChunks(db *gorm.DB, chunks []entity.DocumentChunk) error {
	if len(chunks) == 0 {
		return nil
	}
	return db.CreateInBatches(chunks, 100).Error
}

func (r *knowledgeRepository) FindAllDocuments(db *gorm.DB) ([]entity.Document, error) {
	var documents []entity.Document
	err := db.Order("created_at DESC").Find(&documents).Error
	return documents, err
}

func (r *knowledgeRepository) FindDocumentByID(db *gorm.DB, id string) (*entity.Document, error) {
	document := new(entity.Document)
	err := db.Where("id = ?", id).First(document).Error
	if err != nil {
		return nil, err
	}
	return document, nil
}

func (r *knowledgeRepository) DeleteDocument(db *gorm.DB, documentID string) error {
	if err := db.Where("document_id = ?", documentID).Delete(&entity.DocumentChunk{}).Error; err != nil {
		return err
	}
	return db.Where("id = ?", documentID).Delete(&entity.Document{}).Error
}

func (r *knowledgeRepository) FindChunkDocuments(db *gorm.DB) ([]entity.ChunkDocument, error) {
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
