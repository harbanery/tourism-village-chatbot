package chat

import (
	"fmt"
	"strings"
	"time"

	"support-agent-golang/entity"
	"support-agent-golang/helper"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

const (
	taskTypeQuery        = "RETRIEVAL_QUERY"
	maxSessionTitleChars = 60
	emptyKnowledgeAnswer = "Maaf, saat ini saya belum memiliki informasi yang cukup untuk menjawab pertanyaan Anda. Silakan hubungi admin desa wisata melalui kontak yang tersedia di website, atau coba pertanyaan lain ya."
)

const defaultSystemPrompt = `Anda adalah "Asisten DesakuWisataku", AI support agent untuk website desa wisata DesakuWisataku (paket wisata, tempat, galeri, vlog, artikel, dan pemesanan).

Aturan menjawab:
1. Jawab HANYA berdasarkan KONTEKS yang diberikan. Jangan mengarang informasi seperti harga, jadwal, atau fasilitas yang tidak ada di konteks.
2. Jika KONTEKS tidak cukup untuk menjawab, sampaikan dengan sopan bahwa informasi belum tersedia dan arahkan pengguna menghubungi admin desa wisata.
3. Gunakan bahasa Indonesia yang ramah, sopan, dan mudah dipahami.
4. Jawab ringkas dan terstruktur. Gunakan poin jika relevan.
5. Anda boleh membantu topik seputar desa wisata secara umum (tips berkunjung, cara memesan, dll) tanpa mengarang data spesifik desa.`

type ChatUseCase interface {
	CreateSession(ctx *fiber.Ctx) *helper.WebResponse[interface{}]
	SendMessage(ctx *fiber.Ctx) *helper.WebResponse[interface{}]
	GetHistory(ctx *fiber.Ctx) *helper.WebResponse[interface{}]
}

type chatUseCase struct {
	DB         *gorm.DB
	Log        *logrus.Logger
	Validate   *validator.Validate
	Repository ChatRepository
	Config     *viper.Viper
	Vertex     *helper.VertexAIClient
}

func NewChatUseCase(db *gorm.DB, logger *logrus.Logger, validate *validator.Validate, repository ChatRepository, viper *viper.Viper, vertex *helper.VertexAIClient) ChatUseCase {
	return &chatUseCase{
		DB:         db,
		Log:        logger,
		Validate:   validate,
		Repository: repository,
		Config:     viper,
		Vertex:     vertex,
	}
}

func (c *chatUseCase) CreateSession(ctx *fiber.Ctx) *helper.WebResponse[interface{}] {
	sessionID := helper.GenerateSessionID()
	module := "CHAT CREATE SESSION"

	request := new(CreateSessionRequest)
	if err := ctx.BodyParser(request); err != nil {
		return helper.Response(ctx, c.Config, 400, err.Error(), helper.EmptyObject())
	}

	session := &entity.ChatSession{
		ID:     uuid.NewString(),
		UserID: strings.TrimSpace(request.UserID),
		Title:  strings.TrimSpace(request.Title),
	}

	if err := c.Repository.CreateSession(c.DB.WithContext(ctx.Context()), session); err != nil {
		c.Log.WithFields(logrus.Fields{"session_id": sessionID, "module": module, "action": "response"}).Error(helper.MessageLog(500, err))
		return helper.Response(ctx, c.Config, 500, err.Error(), helper.EmptyObject())
	}

	response := helper.Response(ctx, c.Config, 0, "", ToSessionResponse(*session))
	c.Log.WithFields(logrus.Fields{"session_id": sessionID, "module": module, "action": "response"}).Info("chat_session_id: " + session.ID)
	return response
}

// SendMessage menjalankan pipeline RAG lengkap:
//  1. embed pertanyaan dengan Vertex AI (EmbeddingGemma / gemini-embedding)
//  2. ambil chunk knowledge base paling mirip (cosine similarity)
//  3. susun prompt dengan konteks + riwayat percakapan
//  4. generate jawaban dengan Gemini di Vertex AI
//  5. simpan jawaban beserta sumber kutipannya
func (c *chatUseCase) SendMessage(ctx *fiber.Ctx) *helper.WebResponse[interface{}] {
	sessionID := helper.GenerateSessionID()
	module := "CHAT SEND MESSAGE"

	request := new(SendMessageRequest)
	if err := helper.ValidateRequest(ctx, c.Validate, request); err != nil {
		return helper.Response(ctx, c.Config, 400, err.Error(), helper.EmptyObject())
	}
	c.Log.WithFields(logrus.Fields{"session_id": sessionID, "module": module, "action": "request"}).Info(request.Message)

	session, err := c.Repository.FindSessionByID(c.DB.WithContext(ctx.Context()), request.SessionID)
	if err != nil {
		return helper.Response(ctx, c.Config, 404, "", helper.EmptyObject())
	}

	// 1. Embed pertanyaan.
	vectors, err := c.Vertex.EmbedTexts(ctx.Context(), taskTypeQuery, []string{request.Message})
	if err != nil {
		c.Log.WithFields(logrus.Fields{"session_id": sessionID, "module": module, "action": "embed_query"}).Error(helper.MessageLog(502, err))
		return helper.Response(ctx, c.Config, 502, "", helper.EmptyObject())
	}
	if len(vectors) != 1 {
		return helper.Response(ctx, c.Config, 502, "", helper.EmptyObject())
	}

	// 2. Retrieval: chunk paling relevan dari knowledge base.
	chunks, err := c.Repository.FindChunkDocuments(c.DB.WithContext(ctx.Context()))
	if err != nil {
		c.Log.WithFields(logrus.Fields{"session_id": sessionID, "module": module, "action": "retrieve"}).Error(helper.MessageLog(500, err))
		return helper.Response(ctx, c.Config, 500, err.Error(), helper.EmptyObject())
	}

	sources, contextBlock := c.rankChunks(vectors[0], chunks)

	// 3. Ambil riwayat percakapan sebelum pesan baru disimpan.
	maxHistory := c.Config.GetInt("chat.max_history")
	if maxHistory <= 0 {
		maxHistory = 10
	}
	history, err := c.Repository.FindMessagesBySessionID(c.DB.WithContext(ctx.Context()), session.ID, maxHistory)
	if err != nil {
		c.Log.WithFields(logrus.Fields{"session_id": sessionID, "module": module, "action": "load_history"}).Error(helper.MessageLog(500, err))
		return helper.Response(ctx, c.Config, 500, err.Error(), helper.EmptyObject())
	}

	// 4. Simpan pesan user.
	userMessage := &entity.ChatMessage{
		ID:        uuid.NewString(),
		SessionID: session.ID,
		Role:      entity.MessageRoleUser,
		Content:   request.Message,
	}
	if err := c.Repository.CreateMessage(c.DB.WithContext(ctx.Context()), userMessage); err != nil {
		c.Log.WithFields(logrus.Fields{"session_id": sessionID, "module": module, "action": "save_user_message"}).Error(helper.MessageLog(500, err))
		return helper.Response(ctx, c.Config, 500, err.Error(), helper.EmptyObject())
	}

	// 5. Generate jawaban.
	answer := emptyKnowledgeAnswer
	var latencyMs int64

	if len(sources) > 0 {
		answer, latencyMs, err = c.generateAnswer(ctx, request.Message, history, sources, contextBlock)
		if err != nil {
			c.Log.WithFields(logrus.Fields{"session_id": sessionID, "module": module, "action": "generate"}).Error(helper.MessageLog(502, err))
			return helper.Response(ctx, c.Config, 502, "", helper.EmptyObject())
		}
	}

	// 6. Simpan jawaban + sumber.
	assistantMessage := &entity.ChatMessage{
		ID:        uuid.NewString(),
		SessionID: session.ID,
		Role:      entity.MessageRoleAssistant,
		Content:   answer,
		Sources:   sources,
		LatencyMs: latencyMs,
	}
	if err := c.Repository.CreateMessage(c.DB.WithContext(ctx.Context()), assistantMessage); err != nil {
		c.Log.WithFields(logrus.Fields{"session_id": sessionID, "module": module, "action": "save_assistant_message"}).Error(helper.MessageLog(500, err))
		return helper.Response(ctx, c.Config, 500, err.Error(), helper.EmptyObject())
	}

	if session.Title == "" {
		session.Title = truncateSessionTitle(request.Message)
	}
	if err := c.Repository.UpdateSession(c.DB.WithContext(ctx.Context()), session); err != nil {
		c.Log.WithFields(logrus.Fields{"session_id": sessionID, "module": module, "action": "update_session"}).Error(helper.MessageLog(500, err))
		return helper.Response(ctx, c.Config, 500, err.Error(), helper.EmptyObject())
	}

	data := SendMessageData{
		SessionID: session.ID,
		Answer:    answer,
		Sources:   sources,
		LatencyMs: latencyMs,
	}

	response := helper.Response(ctx, c.Config, 0, "", data)
	c.Log.WithFields(logrus.Fields{"session_id": sessionID, "module": module, "action": "response"}).Info("answer delivered")
	return response
}

func (c *chatUseCase) GetHistory(ctx *fiber.Ctx) *helper.WebResponse[interface{}] {
	sessionID := helper.GenerateSessionID()
	module := "CHAT GET HISTORY"

	chatSessionID := ctx.Params("id")
	if chatSessionID == "" {
		return helper.Response(ctx, c.Config, 400, "id is required", helper.EmptyArray())
	}

	if _, err := c.Repository.FindSessionByID(c.DB.WithContext(ctx.Context()), chatSessionID); err != nil {
		return helper.Response(ctx, c.Config, 404, "", helper.EmptyArray())
	}

	messages, err := c.Repository.FindMessagesBySessionID(c.DB.WithContext(ctx.Context()), chatSessionID, 0)
	if err != nil {
		c.Log.WithFields(logrus.Fields{"session_id": sessionID, "module": module, "action": "response"}).Error(helper.MessageLog(500, err))
		return helper.Response(ctx, c.Config, 500, err.Error(), helper.EmptyArray())
	}

	responses := make([]MessageResponse, len(messages))
	for i, message := range messages {
		responses[i] = ToMessageResponse(message)
	}

	return helper.Response(ctx, c.Config, 0, "", responses)
}

// rankChunks computes cosine similarity between the query vector and every
// chunk, keeps the top K above the configured minimum score, and returns
// them as citation sources together with a numbered context block for the
// generation prompt.
func (c *chatUseCase) rankChunks(queryVector []float64, chunks []entity.ChunkDocument) (entity.SourceList, string) {
	topK := c.Config.GetInt("chat.top_k")
	if topK <= 0 {
		topK = 5
	}
	minScore := c.Config.GetFloat64("chat.min_score")

	type scored struct {
		chunk entity.ChunkDocument
		score float64
	}

	scoredChunks := make([]scored, 0, len(chunks))
	for _, chunk := range chunks {
		scoredChunks = append(scoredChunks, scored{chunk: chunk, score: helper.CosineSimilarity(queryVector, chunk.Embedding)})
	}

	for i := 0; i < len(scoredChunks); i++ {
		for j := i + 1; j < len(scoredChunks); j++ {
			if scoredChunks[j].score > scoredChunks[i].score {
				scoredChunks[i], scoredChunks[j] = scoredChunks[j], scoredChunks[i]
			}
		}
	}

	if len(scoredChunks) > topK {
		scoredChunks = scoredChunks[:topK]
	}

	sources := make(entity.SourceList, 0, len(scoredChunks))
	var contextBuilder strings.Builder
	index := 0
	for _, s := range scoredChunks {
		if s.score < minScore {
			continue
		}
		index++
		sources = append(sources, entity.MessageSource{
			ChunkID:    s.chunk.ID,
			DocumentID: s.chunk.DocumentID,
			Title:      s.chunk.Title,
			Score:      s.score,
		})
		contextBuilder.WriteString(fmt.Sprintf("[%d] (%s)\n%s\n\n", index, s.chunk.Title, s.chunk.Content))
	}

	return sources, strings.TrimSpace(contextBuilder.String())
}

// generateAnswer builds the RAG prompt (system prompt + recent history +
// retrieved context) and calls Gemini on Vertex AI.
func (c *chatUseCase) generateAnswer(ctx *fiber.Ctx, userMessage string, history []entity.ChatMessage, sources entity.SourceList, contextBlock string) (string, int64, error) {
	systemPrompt := c.Config.GetString("chat.system_prompt")
	if strings.TrimSpace(systemPrompt) == "" {
		systemPrompt = defaultSystemPrompt
	}
	systemPrompt += "\n\nKONTEKS diambil dari knowledge base dokumen desa wisata. Kutip nomor sumber [n] pada jawaban jika relevan."

	contents := make([]helper.GenerativeContent, 0, len(history)+1)
	for _, message := range history {
		role := "user"
		if message.Role == entity.MessageRoleAssistant {
			role = "model"
		}
		contents = append(contents, helper.GenerativeContent{
			Role:  role,
			Parts: []helper.ContentPart{{Text: message.Content}},
		})
	}

	contents = append(contents, helper.GenerativeContent{
		Role:  "user",
		Parts: []helper.ContentPart{{Text: fmt.Sprintf("KONTEKS:\n\n%s\nPERTANYAAN:\n%s", contextBlock, userMessage)}},
	})

	temperature := c.Config.GetFloat64("chat.temperature")
	if temperature <= 0 {
		temperature = 0.2
	}
	maxOutputTokens := c.Config.GetInt("chat.max_output_tokens")
	if maxOutputTokens <= 0 {
		maxOutputTokens = 1024
	}

	generateRequest := &helper.GenerateContentRequest{
		Contents: contents,
		SystemInstruction: &helper.SystemInstruction{
			Parts: []helper.ContentPart{{Text: systemPrompt}},
		},
		GenerationConfig: &helper.GenerationConfig{
			Temperature:     temperature,
			MaxOutputTokens: maxOutputTokens,
		},
	}

	startedAt := time.Now()
	answer, err := c.Vertex.GenerateContent(ctx.Context(), generateRequest)
	if err != nil {
		return "", 0, err
	}

	return answer, time.Since(startedAt).Milliseconds(), nil
}

func truncateSessionTitle(message string) string {
	title := strings.TrimSpace(message)
	runes := []rune(title)
	if len(runes) > maxSessionTitleChars {
		return string(runes[:maxSessionTitleChars]) + "..."
	}
	return title
}
