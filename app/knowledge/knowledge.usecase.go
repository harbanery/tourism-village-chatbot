package knowledge

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"support-agent-golang/entity"
	"support-agent-golang/helper"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/ledongthuc/pdf"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

const (
	taskTypeDocument = "RETRIEVAL_DOCUMENT"
	taskTypeQuery    = "RETRIEVAL_QUERY"

	textFileType = "text/plain"
	pdfFileType  = "application/pdf"

	maxTitleLength = 200
)

type KnowledgeUseCase interface {
	IngestText(ctx *fiber.Ctx) *helper.WebResponse[interface{}]
	UploadDocument(ctx *fiber.Ctx) *helper.WebResponse[interface{}]
	ListDocuments(ctx *fiber.Ctx) *helper.WebResponse[interface{}]
	DeleteDocument(ctx *fiber.Ctx) *helper.WebResponse[interface{}]
	Search(ctx *fiber.Ctx) *helper.WebResponse[interface{}]
}

type knowledgeUseCase struct {
	DB         *gorm.DB
	Log        *logrus.Logger
	Validate   *validator.Validate
	Repository KnowledgeRepository
	Config     *viper.Viper
	Vertex     *helper.VertexAIClient
}

func NewKnowledgeUseCase(db *gorm.DB, logger *logrus.Logger, validate *validator.Validate, repository KnowledgeRepository, viper *viper.Viper, vertex *helper.VertexAIClient) KnowledgeUseCase {
	return &knowledgeUseCase{
		DB:         db,
		Log:        logger,
		Validate:   validate,
		Repository: repository,
		Config:     viper,
		Vertex:     vertex,
	}
}

// IngestText menambahkan pengetahuan berupa teks langsung (mis. deskripsi
// paket wisata, FAQ, kebijakan desa) lalu meng-embed-nya ke knowledge base.
func (c *knowledgeUseCase) IngestText(ctx *fiber.Ctx) *helper.WebResponse[interface{}] {
	sessionID := helper.GenerateSessionID()
	module := "KNOWLEDGE INGEST TEXT"

	request := new(IngestTextRequest)
	if err := helper.ValidateRequest(ctx, c.Validate, request); err != nil {
		return helper.Response(ctx, c.Config, 400, err.Error(), helper.EmptyObject())
	}
	c.Log.WithFields(logrus.Fields{"session_id": sessionID, "module": module, "action": "request"}).Info(request.Title)

	document, err := c.ingestContent(ctx.Context(), request.Title, request.Content, entity.SourceTypeText, "", textFileType, "")
	if err != nil {
		c.Log.WithFields(logrus.Fields{"session_id": sessionID, "module": module, "action": "response"}).Error(helper.MessageLog(500, err))
		return helper.Response(ctx, c.Config, 500, err.Error(), helper.EmptyObject())
	}

	response := helper.Response(ctx, c.Config, 0, "", ToDocumentResponse(*document))
	c.Log.WithFields(logrus.Fields{"session_id": sessionID, "module": module, "action": "response"}).Info("document_id: " + document.ID)
	return response
}

// UploadDocument menambahkan pengetahuan dari file: dokumen teks
// (.txt, .md, .csv, .json, .html), PDF, atau gambar (jpg, png, webp).
// Gambar dideskripsikan oleh Gemini multimodal terlebih dahulu, lalu
// deskripsinya yang di-embed.
func (c *knowledgeUseCase) UploadDocument(ctx *fiber.Ctx) *helper.WebResponse[interface{}] {
	sessionID := helper.GenerateSessionID()
	module := "KNOWLEDGE UPLOAD DOCUMENT"

	fileHeader, err := ctx.FormFile("file")
	if err != nil {
		return helper.Response(ctx, c.Config, 400, "file is required", helper.EmptyObject())
	}

	title := strings.TrimSpace(ctx.FormValue("title"))

	maxFileSizeMB := c.Config.GetInt("knowledge.max_file_size_mb")
	if maxFileSizeMB <= 0 {
		maxFileSizeMB = 20
	}
	if fileHeader.Size > int64(maxFileSizeMB)*1024*1024 {
		return helper.Response(ctx, c.Config, 413, "", helper.EmptyObject())
	}

	file, err := fileHeader.Open()
	if err != nil {
		return helper.Response(ctx, c.Config, 400, "file cannot be opened", helper.EmptyObject())
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return helper.Response(ctx, c.Config, 400, "file cannot be read", helper.EmptyObject())
	}

	extension := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(fileHeader.Filename), filepath.Ext(fileHeader.Filename))
	}

	var (
		content     string
		mimeType    string
		description string
	)

	switch extension {
	case ".txt", ".md", ".csv", ".json", ".html", ".htm":
		if !utf8.Valid(data) {
			return helper.Response(ctx, c.Config, 415, "file must be UTF-8 encoded text", helper.EmptyObject())
		}
		content = string(data)
		mimeType = textFileType
		if extension == ".json" {
			mimeType = "application/json"
		} else if extension == ".csv" {
			mimeType = "text/csv"
		} else if extension == ".html" || extension == ".htm" {
			mimeType = "text/html"
		}

	case ".pdf":
		content, err = extractPDFText(data)
		if err != nil {
			c.Log.WithFields(logrus.Fields{"session_id": sessionID, "module": module, "action": "extract_pdf"}).Error(err.Error())
			return helper.Response(ctx, c.Config, 422, "", helper.EmptyObject())
		}
		mimeType = pdfFileType

	case ".png", ".jpg", ".jpeg", ".webp":
		mimeType = imageMimeType(extension)
		c.Log.WithFields(logrus.Fields{"session_id": sessionID, "module": module, "action": "describe_image"}).Info(fileHeader.Filename)
		description, err = c.Vertex.DescribeImage(ctx.Context(), mimeType, base64.StdEncoding.EncodeToString(data))
		if err != nil {
			c.Log.WithFields(logrus.Fields{"session_id": sessionID, "module": module, "action": "describe_image"}).Error(helper.MessageLog(502, err))
			return helper.Response(ctx, c.Config, 502, "", helper.EmptyObject())
		}
		content = description

	default:
		return helper.Response(ctx, c.Config, 415, "", helper.EmptyObject())
	}

	if strings.TrimSpace(content) == "" {
		return helper.Response(ctx, c.Config, 422, "", helper.EmptyObject())
	}

	document, err := c.ingestContent(ctx.Context(), title, content, sourceTypeForExtension(extension), fileHeader.Filename, mimeType, description)
	if err != nil {
		c.Log.WithFields(logrus.Fields{"session_id": sessionID, "module": module, "action": "response"}).Error(helper.MessageLog(500, err))
		return helper.Response(ctx, c.Config, 500, err.Error(), helper.EmptyObject())
	}

	response := helper.Response(ctx, c.Config, 0, "", ToDocumentResponse(*document))
	c.Log.WithFields(logrus.Fields{"session_id": sessionID, "module": module, "action": "response"}).Info("document_id: " + document.ID)
	return response
}

func (c *knowledgeUseCase) ListDocuments(ctx *fiber.Ctx) *helper.WebResponse[interface{}] {
	sessionID := helper.GenerateSessionID()
	module := "KNOWLEDGE LIST DOCUMENTS"

	documents, err := c.Repository.FindAllDocuments(c.DB.WithContext(ctx.Context()))
	if err != nil {
		c.Log.WithFields(logrus.Fields{"session_id": sessionID, "module": module, "action": "response"}).Error(helper.MessageLog(500, err))
		return helper.Response(ctx, c.Config, 500, err.Error(), helper.EmptyArray())
	}

	responses := make([]DocumentResponse, len(documents))
	for i, document := range documents {
		responses[i] = ToDocumentResponse(document)
	}

	return helper.Response(ctx, c.Config, 0, "", responses)
}

func (c *knowledgeUseCase) DeleteDocument(ctx *fiber.Ctx) *helper.WebResponse[interface{}] {
	sessionID := helper.GenerateSessionID()
	module := "KNOWLEDGE DELETE DOCUMENT"

	documentID := ctx.Params("id")
	if documentID == "" {
		return helper.Response(ctx, c.Config, 400, "id is required", helper.EmptyObject())
	}

	document, err := c.Repository.FindDocumentByID(c.DB.WithContext(ctx.Context()), documentID)
	if err != nil {
		return helper.Response(ctx, c.Config, 404, "", helper.EmptyObject())
	}

	err = c.DB.WithContext(ctx.Context()).Transaction(func(tx *gorm.DB) error {
		return c.Repository.DeleteDocument(tx, document.ID)
	})
	if err != nil {
		c.Log.WithFields(logrus.Fields{"session_id": sessionID, "module": module, "action": "response"}).Error(helper.MessageLog(500, err))
		return helper.Response(ctx, c.Config, 500, err.Error(), helper.EmptyObject())
	}

	return helper.Response(ctx, c.Config, 0, "", helper.EmptyObject())
}

// Search mengembalikan chunk knowledge base paling relevan terhadap
// sebuah query — berguna untuk memverifikasi kualitas retrieval RAG.
func (c *knowledgeUseCase) Search(ctx *fiber.Ctx) *helper.WebResponse[interface{}] {
	sessionID := helper.GenerateSessionID()
	module := "KNOWLEDGE SEARCH"

	request := new(SearchRequest)
	if err := helper.ValidateRequest(ctx, c.Validate, request); err != nil {
		return helper.Response(ctx, c.Config, 400, err.Error(), helper.EmptyArray())
	}
	c.Log.WithFields(logrus.Fields{"session_id": sessionID, "module": module, "action": "request"}).Info(request.Query)

	topK := request.TopK
	if topK <= 0 {
		topK = c.Config.GetInt("chat.top_k")
		if topK <= 0 {
			topK = 5
		}
	}

	hits, err := c.RetrieveTopK(ctx.Context(), request.Query, topK)
	if err != nil {
		c.Log.WithFields(logrus.Fields{"session_id": sessionID, "module": module, "action": "response"}).Error(helper.MessageLog(500, err))
		return helper.Response(ctx, c.Config, 500, err.Error(), helper.EmptyArray())
	}

	return helper.Response(ctx, c.Config, 0, "", hits)
}

// RetrieveTopK embeds a query and returns the most similar knowledge
// chunks. Shared with the chat module for the RAG pipeline.
func (c *knowledgeUseCase) RetrieveTopK(ctxContext context.Context, query string, topK int) ([]SearchHit, error) {
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("query tidak boleh kosong")
	}

	vectors, err := c.Vertex.EmbedTexts(ctxContext, taskTypeQuery, []string{query})
	if err != nil {
		return nil, err
	}
	if len(vectors) != 1 {
		return nil, fmt.Errorf("embedding query tidak valid")
	}
	queryVector := vectors[0]

	chunks, err := c.Repository.FindChunkDocuments(c.DB.WithContext(ctxContext))
	if err != nil {
		return nil, err
	}

	hits := make([]SearchHit, 0, len(chunks))
	for _, chunk := range chunks {
		score := helper.CosineSimilarity(queryVector, chunk.Embedding)
		hits = append(hits, SearchHit{
			ChunkID:    chunk.ID,
			DocumentID: chunk.DocumentID,
			Title:      chunk.Title,
			Content:    chunk.Content,
			Score:      score,
		})
	}

	// Sort by score descending.
	for i := 0; i < len(hits); i++ {
		for j := i + 1; j < len(hits); j++ {
			if hits[j].Score > hits[i].Score {
				hits[i], hits[j] = hits[j], hits[i]
			}
		}
	}

	minScore := c.Config.GetFloat64("chat.min_score")
	if len(hits) > topK {
		hits = hits[:topK]
	}

	filtered := hits[:0]
	for _, hit := range hits {
		if hit.Score >= minScore {
			filtered = append(filtered, hit)
		}
	}

	return filtered, nil
}

// ingestContent is the shared RAG ingestion pipeline: chunk the content,
// embed every chunk with Vertex AI, then persist the document and its
// chunks atomically.
func (c *knowledgeUseCase) ingestContent(ctxContext context.Context, title string, content string, sourceType string, fileName string, mimeType string, description string) (*entity.Document, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "Untitled"
	}
	if len(title) > maxTitleLength {
		title = title[:maxTitleLength]
	}

	chunkSize := c.Config.GetInt("knowledge.chunk_size")
	if chunkSize <= 0 {
		chunkSize = 1200
	}
	chunkOverlap := c.Config.GetInt("knowledge.chunk_overlap")
	if chunkOverlap < 0 {
		chunkOverlap = 150
	}

	chunkTexts := helper.SplitTextIntoChunks(content, chunkSize, chunkOverlap)
	if len(chunkTexts) == 0 {
		return nil, fmt.Errorf("tidak ada konten yang bisa di-embed")
	}

	vectors, err := c.Vertex.EmbedTexts(ctxContext, taskTypeDocument, chunkTexts)
	if err != nil {
		return nil, err
	}

	document := &entity.Document{
		ID:          uuid.NewString(),
		Title:       title,
		SourceType:  sourceType,
		FileName:    fileName,
		MimeType:    mimeType,
		Description: description,
		Status:      entity.DocumentStatusReady,
		ChunkCount:  len(chunkTexts),
	}

	chunks := make([]entity.DocumentChunk, len(chunkTexts))
	for i, chunkText := range chunkTexts {
		chunks[i] = entity.DocumentChunk{
			ID:         uuid.NewString(),
			DocumentID: document.ID,
			ChunkIndex: i,
			Content:    chunkText,
			Embedding:  vectors[i],
		}
	}

	err = c.DB.WithContext(ctxContext).Transaction(func(tx *gorm.DB) error {
		if err := c.Repository.CreateDocument(tx, document); err != nil {
			return err
		}
		return c.Repository.CreateChunks(tx, chunks)
	})
	if err != nil {
		return nil, err
	}

	return document, nil
}

func extractPDFText(data []byte) (string, error) {
	reader, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("gagal membuka PDF: %w", err)
	}

	textReader, err := reader.GetPlainText()
	if err != nil {
		return "", fmt.Errorf("gagal mengekstrak teks PDF: %w", err)
	}

	textBytes, err := io.ReadAll(textReader)
	if err != nil {
		return "", fmt.Errorf("gagal membaca teks PDF: %w", err)
	}

	text := strings.TrimSpace(string(textBytes))
	if text == "" {
		return "", fmt.Errorf("PDF tidak memiliki teks yang bisa diekstrak")
	}

	return text, nil
}

func imageMimeType(extension string) string {
	switch extension {
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	default:
		return "image/jpeg"
	}
}

func sourceTypeForExtension(extension string) string {
	switch extension {
	case ".png", ".jpg", ".jpeg", ".webp":
		return entity.SourceTypeImage
	case ".pdf", ".txt", ".md", ".csv", ".json", ".html", ".htm":
		return entity.SourceTypeFile
	default:
		return entity.SourceTypeFile
	}
}
