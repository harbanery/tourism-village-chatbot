package helper

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"golang.org/x/oauth2/google"
)

const (
	vertexScope                 = "https://www.googleapis.com/auth/cloud-platform"
	defaultEmbeddingModel       = "gemini-embedding-001"
	defaultChatModel            = "gemini-2.5-flash"
	defaultRegion               = "asia-southeast2"
	defaultEmbeddingDimension   = 768
	defaultEmbeddingBatchSize   = 5
	defaultEmbeddingInstanceKey = "content"
	defaultImageDescribePrompt  = "Deskripsikan gambar ini secara detail dan faktual dalam bahasa Indonesia. Sertakan objek, tempat, aktivitas, dan suasana yang terlihat. Deskripsikan dalam bentuk paragraf ringkas."
)

// VertexAIClient is a lightweight REST client for Google Vertex AI
// (embeddings via :predict and generation via :generateContent).
// Authentication uses Application Default Credentials, or a service
// account JSON file when vertex.credentials_file is configured.
//
// Embeddings can be produced in two ways:
//   - a Google publisher model (default: gemini-embedding-001), or
//   - an open-weights model deployed to a Vertex AI endpoint, such as
//     EmbeddingGemma 2 from Model Garden (set vertex.embedding_endpoint).
type VertexAIClient struct {
	log        *logrus.Logger
	httpClient *http.Client

	projectID             string
	region                string
	embeddingModel        string
	embeddingEndpoint     string
	embeddingInstanceKey  string
	embeddingSendTaskType bool
	chatModel             string
	embeddingDim          int
	embeddingBatch        int
	imageDescPrompt       string
	credentials           *google.Credentials
	initErr               error
	requestTimeoutSec     int
}

func NewVertexAIClient(config *viper.Viper, log *logrus.Logger) *VertexAIClient {
	timeoutSec := config.GetInt("vertex.timeout_seconds")
	if timeoutSec <= 0 {
		timeoutSec = 120
	}

	embeddingBatch := config.GetInt("vertex.embedding_batch_size")
	if embeddingBatch <= 0 {
		embeddingBatch = defaultEmbeddingBatchSize
	}

	embeddingDim := config.GetInt("vertex.embedding_dimension")
	if embeddingDim <= 0 {
		embeddingDim = defaultEmbeddingDimension
	}

	client := &VertexAIClient{
		log:                  log,
		httpClient:           &http.Client{Timeout: time.Duration(timeoutSec) * time.Second},
		projectID:            config.GetString("vertex.project_id"),
		region:               config.GetString("vertex.region"),
		embeddingModel:       config.GetString("vertex.embedding_model"),
		embeddingEndpoint:    config.GetString("vertex.embedding_endpoint"),
		embeddingInstanceKey: config.GetString("vertex.embedding_instance_key"),
		chatModel:            config.GetString("vertex.chat_model"),
		embeddingDim:         embeddingDim,
		embeddingBatch:       embeddingBatch,
		imageDescPrompt:      config.GetString("vertex.image_prompt"),
		requestTimeoutSec:    timeoutSec,
	}

	if client.region == "" {
		client.region = defaultRegion
	}
	if client.embeddingModel == "" {
		client.embeddingModel = defaultEmbeddingModel
	}
	if client.embeddingInstanceKey == "" {
		client.embeddingInstanceKey = defaultEmbeddingInstanceKey
	}
	if client.chatModel == "" {
		client.chatModel = defaultChatModel
	}
	if client.imageDescPrompt == "" {
		client.imageDescPrompt = defaultImageDescribePrompt
	}

	// Publisher models accept a task_type hint to steer retrieval quality.
	// Deployed open-weights containers usually do not, so default to sending
	// it only for publisher models unless the config overrides it explicitly.
	if config.IsSet("vertex.embedding_send_task_type") {
		client.embeddingSendTaskType = config.GetBool("vertex.embedding_send_task_type")
	} else {
		client.embeddingSendTaskType = client.embeddingEndpoint == ""
	}

	ctx := context.Background()
	credentialsFile := config.GetString("vertex.credentials_file")
	if credentialsFile != "" {
		credentialsJSON, err := os.ReadFile(credentialsFile)
		if err != nil {
			client.initErr = fmt.Errorf("gagal membaca credentials file Vertex AI: %w", err)
		} else {
			creds, err := google.CredentialsFromJSON(ctx, credentialsJSON, vertexScope)
			if err != nil {
				client.initErr = fmt.Errorf("gagal memuat credentials file Vertex AI: %w", err)
			} else {
				client.credentials = creds
			}
		}
	} else {
		creds, err := google.FindDefaultCredentials(ctx, vertexScope)
		if err != nil {
			client.initErr = fmt.Errorf("Application Default Credentials tidak ditemukan (jalankan 'gcloud auth application-default login' atau isi vertex.credentials_file): %w", err)
		} else {
			client.credentials = creds
		}
	}

	return client
}

// EmbeddingModel returns a human-readable identifier of the embedding target
// (deployed endpoint when configured, otherwise the publisher model name).
func (c *VertexAIClient) EmbeddingModel() string {
	if c.embeddingEndpoint != "" {
		return c.embeddingEndpoint
	}
	return c.embeddingModel
}

func (c *VertexAIClient) ChatModel() string       { return c.chatModel }
func (c *VertexAIClient) EmbeddingDimension() int { return c.embeddingDim }

func (c *VertexAIClient) token(ctx context.Context) (string, error) {
	if c.initErr != nil {
		return "", c.initErr
	}

	tok, err := c.credentials.TokenSource.Token()
	if err != nil {
		return "", fmt.Errorf("gagal mendapatkan access token Google: %w", err)
	}

	return tok.AccessToken, nil
}

func (c *VertexAIClient) doRequest(ctx context.Context, method, url string, payload any) ([]byte, int, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, fmt.Errorf("gagal encode payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return nil, 0, fmt.Errorf("gagal membuat request: %w", err)
	}

	accessToken, err := c.token(ctx)
	if err != nil {
		return nil, 0, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("gagal memanggil Vertex AI: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("gagal membaca response Vertex AI: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return respBody, resp.StatusCode, fmt.Errorf("vertex AI %s %s returned status %d: %s", method, url, resp.StatusCode, truncateForLog(string(respBody), 512))
	}

	return respBody, resp.StatusCode, nil
}

// --- Embeddings -----------------------------------------------------------------

// embeddingParameters mirrors the Vertex AI predict parameters block.
// Publisher models (gemini-embedding-001) accept outputDimensionality;
// deployed open-weights models such as EmbeddingGemma 2 ignore unknown keys.
type embeddingParameters struct {
	OutputDimensionality int `json:"outputDimensionality,omitempty"`
}

// embeddingPredictRequest is the shared :predict envelope. Instances are kept
// as generic maps so the instance key can follow the target model
// (vertex.embedding_instance_key, default "content"). Parameters is a pointer
// so it can be omitted entirely for deployed endpoints that do not accept it.
type embeddingPredictRequest struct {
	Instances  []map[string]any     `json:"instances"`
	Parameters *embeddingParameters `json:"parameters,omitempty"`
}

type embeddingPredictResponse struct {
	Predictions []json.RawMessage `json:"predictions"`
}

// parseEmbeddingValues extracts the vector from a single prediction. Vertex AI
// returns different shapes depending on the model:
//   - {"embeddings": {"values": [...]}}  (publisher models)
//   - {"embeddings": [...]}              (some Model Garden containers)
//   - {"embedding": [...]}
func parseEmbeddingValues(raw json.RawMessage) ([]float64, error) {
	var prediction map[string]json.RawMessage
	if err := json.Unmarshal(raw, &prediction); err != nil {
		return nil, fmt.Errorf("prediction tidak valid: %w", err)
	}

	for _, key := range []string{"embeddings", "embedding"} {
		value, ok := prediction[key]
		if !ok {
			continue
		}

		var nested struct {
			Values []float64 `json:"values"`
		}
		if err := json.Unmarshal(value, &nested); err == nil && len(nested.Values) > 0 {
			return nested.Values, nil
		}

		var flat []float64
		if err := json.Unmarshal(value, &flat); err == nil && len(flat) > 0 {
			return flat, nil
		}
	}

	return nil, fmt.Errorf("nilai embedding tidak ditemukan pada response")
}

// embeddingURL builds the :predict endpoint for the configured embedding
// model. When vertex.embedding_endpoint is set (for example an EmbeddingGemma 2
// deployment created in Vertex AI Model Garden) the request targets that
// endpoint; otherwise it targets the Google publisher model.
func (c *VertexAIClient) embeddingURL() string {
	if c.embeddingEndpoint != "" {
		if strings.HasPrefix(c.embeddingEndpoint, "projects/") {
			return fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/%s:predict", c.region, c.embeddingEndpoint)
		}
		return fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/endpoints/%s:predict",
			c.region, c.projectID, c.region, c.embeddingEndpoint)
	}

	return fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/%s:predict",
		c.region, c.projectID, c.region, c.embeddingModel)
}

// EmbedTexts converts texts into embedding vectors. By default it uses the
// Google publisher model (gemini-embedding-001); set vertex.embedding_endpoint
// to use a deployed model such as EmbeddingGemma 2. Requests are batched to
// respect Vertex AI per-request instance limits.
func (c *VertexAIClient) EmbedTexts(ctx context.Context, taskType string, texts []string) ([][]float64, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	if c.initErr != nil {
		return nil, c.initErr
	}

	url := c.embeddingURL()

	vectors := make([][]float64, 0, len(texts))
	for start := 0; start < len(texts); start += c.embeddingBatch {
		end := start + c.embeddingBatch
		if end > len(texts) {
			end = len(texts)
		}

		batch := texts[start:end]
		instances := make([]map[string]any, len(batch))
		for i, text := range batch {
			instance := map[string]any{c.embeddingInstanceKey: text}
			if c.embeddingSendTaskType && taskType != "" {
				instance["task_type"] = taskType
			}
			instances[i] = instance
		}

		payload := embeddingPredictRequest{Instances: instances}
		// outputDimensionality is a Google publisher-model parameter; deployed
		// open-weights endpoints (e.g. EmbeddingGemma 2, fixed 768-d) do not
		// always accept it, so only send it for publisher models.
		if c.embeddingEndpoint == "" {
			payload.Parameters = &embeddingParameters{OutputDimensionality: c.embeddingDim}
		}

		respBody, _, err := c.doRequest(ctx, http.MethodPost, url, payload)
		if err != nil {
			return nil, err
		}

		var resp embeddingPredictResponse
		if err := json.Unmarshal(respBody, &resp); err != nil {
			return nil, fmt.Errorf("gagal decode response embedding: %w", err)
		}

		if len(resp.Predictions) != len(batch) {
			return nil, fmt.Errorf("jumlah prediction tidak sesuai: mengirim %d teks, menerima %d embedding", len(batch), len(resp.Predictions))
		}

		for _, prediction := range resp.Predictions {
			values, err := parseEmbeddingValues(prediction)
			if err != nil {
				return nil, err
			}
			vectors = append(vectors, values)
		}
	}

	return vectors, nil
}

// --- Generation -----------------------------------------------------------------

type ContentPart struct {
	Text       string      `json:"text,omitempty"`
	InlineData *InlineData `json:"inline_data,omitempty"`
}

type InlineData struct {
	MimeType string `json:"mime_type"`
	Data     string `json:"data"`
}

type GenerativeContent struct {
	Role  string        `json:"role,omitempty"`
	Parts []ContentPart `json:"parts"`
}

type SystemInstruction struct {
	Parts []ContentPart `json:"parts"`
}

type GenerationConfig struct {
	Temperature     float64 `json:"temperature,omitempty"`
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
}

type GenerateContentRequest struct {
	Contents          []GenerativeContent `json:"contents"`
	SystemInstruction *SystemInstruction  `json:"system_instruction,omitempty"`
	GenerationConfig  *GenerationConfig   `json:"generation_config,omitempty"`
}

type generateContentResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	PromptFeedback struct {
		BlockReason string `json:"blockReason"`
	} `json:"promptFeedback"`
}

// GenerateContent calls the configured Gemini model on Vertex AI and
// returns the concatenated text of the first candidate.
func (c *VertexAIClient) GenerateContent(ctx context.Context, req *GenerateContentRequest) (string, error) {
	if c.initErr != nil {
		return "", c.initErr
	}

	url := fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/%s:generateContent",
		c.region, c.projectID, c.region, c.chatModel)

	respBody, _, err := c.doRequest(ctx, http.MethodPost, url, req)
	if err != nil {
		return "", err
	}

	var resp generateContentResponse
	if err := json.Unmarshal(respBody, &resp); err != nil {
		return "", fmt.Errorf("gagal decode response generateContent: %w", err)
	}

	if len(resp.Candidates) == 0 {
		if resp.PromptFeedback.BlockReason != "" {
			return "", fmt.Errorf("permintaan diblokir oleh safety settings: %s", resp.PromptFeedback.BlockReason)
		}
		return "", fmt.Errorf("tidak ada kandidat jawaban dari model %s", c.chatModel)
	}

	var answer strings.Builder
	for _, part := range resp.Candidates[0].Content.Parts {
		answer.WriteString(part.Text)
	}

	result := strings.TrimSpace(answer.String())
	if result == "" {
		return "", fmt.Errorf("jawaban kosong dari model %s", c.chatModel)
	}

	return result, nil
}

// DescribeImage uses Gemini multimodal capability to produce a textual
// description of an image. The description is what gets embedded so the
// image becomes searchable in the RAG pipeline.
func (c *VertexAIClient) DescribeImage(ctx context.Context, mimeType string, imageBase64 string) (string, error) {
	if c.initErr != nil {
		return "", c.initErr
	}

	req := &GenerateContentRequest{
		Contents: []GenerativeContent{
			{
				Role: "user",
				Parts: []ContentPart{
					{Text: c.imageDescPrompt},
					{InlineData: &InlineData{MimeType: mimeType, Data: imageBase64}},
				},
			},
		},
		GenerationConfig: &GenerationConfig{
			Temperature:     0.2,
			MaxOutputTokens: 512,
		},
	}

	return c.GenerateContent(ctx, req)
}

func truncateForLog(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}
