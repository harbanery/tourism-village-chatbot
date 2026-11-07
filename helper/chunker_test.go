package helper

import (
	"math"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSplitTextIntoChunks_EmptyInput(t *testing.T) {
	if got := SplitTextIntoChunks("   \n  ", 100, 20); got != nil {
		t.Errorf("expected nil for empty input, got %v", got)
	}
}

func TestSplitTextIntoChunks_ShortTextSingleChunk(t *testing.T) {
	text := "Desa wisata DesakuWisataku menawarkan paket outbound."
	chunks := SplitTextIntoChunks(text, 200, 20)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if !strings.Contains(chunks[0], "DesakuWisataku") {
		t.Errorf("chunk should contain the original text, got %q", chunks[0])
	}
}

func TestSplitTextIntoChunks_RespectsChunkSize(t *testing.T) {
	paragraph := strings.Repeat("kalimat penting. ", 200) // ~3400 chars
	chunks := SplitTextIntoChunks(paragraph, 300, 50)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	for i, chunk := range chunks {
		// Overlap may push a chunk slightly above chunkSize, allow a margin.
		if utf8.RuneCountInString(chunk) > 300+50+1 {
			t.Errorf("chunk %d too large: %d runes", i, utf8.RuneCountInString(chunk))
		}
	}
}

func TestSplitTextIntoChunks_Overlap(t *testing.T) {
	var paragraphs []string
	for i := 0; i < 5; i++ {
		paragraphs = append(paragraphs, strings.Repeat("paragraf", 60)+string(rune('A'+i)))
	}
	text := strings.Join(paragraphs, "\n\n")
	chunks := SplitTextIntoChunks(text, 400, 100)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	// With overlap, chunk N+1 should begin with the tail of chunk N.
	tail := []rune(chunks[0])
	if len(tail) > 100 {
		tail = tail[len(tail)-100:]
	}
	if !strings.HasPrefix(chunks[1], string(tail)) {
		t.Errorf("expected chunk 2 to start with overlap tail %q, got prefix %q", string(tail), string([]rune(chunks[1])[:min(100, len([]rune(chunks[1])))]))
	}
}

func TestSplitTextIntoChunks_HardSplitLongRun(t *testing.T) {
	text := strings.Repeat("x", 5000) // no separators at all
	chunks := SplitTextIntoChunks(text, 500, 50)
	if len(chunks) < 2 {
		t.Fatalf("expected hard split into multiple chunks, got %d", len(chunks))
	}
	for i, chunk := range chunks {
		// Overlap may push a chunk slightly above chunkSize, allow a margin.
		if utf8.RuneCountInString(chunk) > 500+50+1 {
			t.Errorf("hard split chunk %d too large: %d runes", i, utf8.RuneCountInString(chunk))
		}
	}
}

func TestCosineSimilarity(t *testing.T) {
	a := []float64{1, 0, 0}
	b := []float64{1, 0, 0}
	if got := CosineSimilarity(a, b); math.Abs(got-1) > 1e-9 {
		t.Errorf("identical vectors should have similarity 1, got %f", got)
	}

	c := []float64{0, 1, 0}
	if got := CosineSimilarity(a, c); math.Abs(got) > 1e-9 {
		t.Errorf("orthogonal vectors should have similarity 0, got %f", got)
	}

	if got := CosineSimilarity(a, []float64{1}); got != 0 {
		t.Errorf("dimension mismatch should return 0, got %f", got)
	}

	if got := CosineSimilarity([]float64{0, 0}, []float64{0, 0}); got != 0 {
		t.Errorf("zero vectors should return 0, got %f", got)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
