package helper

import (
	"encoding/json"
	"testing"
)

func TestParseEmbeddingValues_NestedObject(t *testing.T) {
	raw := json.RawMessage(`{"embeddings": {"values": [0.1, 0.2, 0.3]}}`)
	values, err := parseEmbeddingValues(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(values) != 3 || values[0] != 0.1 || values[2] != 0.3 {
		t.Fatalf("unexpected values: %v", values)
	}
}

func TestParseEmbeddingValues_FlatArray(t *testing.T) {
	raw := json.RawMessage(`{"embeddings": [0.4, 0.5]}`)
	values, err := parseEmbeddingValues(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(values) != 2 || values[1] != 0.5 {
		t.Fatalf("unexpected values: %v", values)
	}
}

func TestParseEmbeddingValues_SingularEmbeddingKey(t *testing.T) {
	raw := json.RawMessage(`{"embedding": [1, 2, 3, 4]}`)
	values, err := parseEmbeddingValues(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(values) != 4 {
		t.Fatalf("unexpected values: %v", values)
	}
}

func TestParseEmbeddingValues_Missing(t *testing.T) {
	if _, err := parseEmbeddingValues(json.RawMessage(`{"foo": 1}`)); err == nil {
		t.Fatal("expected error for prediction without embedding values")
	}
}

func TestEmbeddingURL_PublisherModel(t *testing.T) {
	client := &VertexAIClient{
		projectID:      "my-project",
		region:         "asia-southeast2",
		embeddingModel: "gemini-embedding-001",
	}

	want := "https://asia-southeast2-aiplatform.googleapis.com/v1/projects/my-project/locations/asia-southeast2/publishers/google/models/gemini-embedding-001:predict"
	if got := client.embeddingURL(); got != want {
		t.Errorf("unexpected publisher URL:\n got: %s\nwant: %s", got, want)
	}
}

func TestEmbeddingURL_DeployedEndpointNumeric(t *testing.T) {
	client := &VertexAIClient{
		projectID:         "my-project",
		region:            "asia-southeast2",
		embeddingEndpoint: "1234567890",
	}

	want := "https://asia-southeast2-aiplatform.googleapis.com/v1/projects/my-project/locations/asia-southeast2/endpoints/1234567890:predict"
	if got := client.embeddingURL(); got != want {
		t.Errorf("unexpected endpoint URL:\n got: %s\nwant: %s", got, want)
	}
}

func TestEmbeddingURL_DeployedEndpointFullResource(t *testing.T) {
	client := &VertexAIClient{
		projectID:         "my-project",
		region:            "asia-southeast2",
		embeddingEndpoint: "projects/my-project/locations/asia-southeast2/endpoints/1234567890",
	}

	want := "https://asia-southeast2-aiplatform.googleapis.com/v1/projects/my-project/locations/asia-southeast2/endpoints/1234567890:predict"
	if got := client.embeddingURL(); got != want {
		t.Errorf("unexpected full-resource URL:\n got: %s\nwant: %s", got, want)
	}
}

func TestEmbeddingModel_ReportsEndpointWhenDeployed(t *testing.T) {
	publisher := &VertexAIClient{embeddingModel: "gemini-embedding-001"}
	if got := publisher.EmbeddingModel(); got != "gemini-embedding-001" {
		t.Errorf("expected publisher model name, got %s", got)
	}

	deployed := &VertexAIClient{embeddingModel: "gemini-embedding-001", embeddingEndpoint: "1234567890"}
	if got := deployed.EmbeddingModel(); got != "1234567890" {
		t.Errorf("expected endpoint id, got %s", got)
	}
}
