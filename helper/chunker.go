package helper

import (
	"math"
	"strings"
	"unicode/utf8"
)

const (
	defaultChunkSize  = 1200
	defaultOverlap    = 150
	hardSplitStepSize = 64
)

// SplitTextIntoChunks splits raw text into overlapping chunks suitable
// for embedding. It prefers paragraph boundaries, then sentences, and
// finally hard-slices anything longer than chunkSize.
func SplitTextIntoChunks(text string, chunkSize int, overlap int) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	if chunkSize <= 0 {
		chunkSize = defaultChunkSize
	}
	if overlap < 0 {
		overlap = defaultOverlap
	}
	if overlap >= chunkSize {
		overlap = chunkSize / 4
	}

	paragraphs := splitParagraphs(text)

	var pieces []string
	for _, paragraph := range paragraphs {
		if paragraph == "" {
			continue
		}

		if utf8.RuneCountInString(paragraph) <= chunkSize {
			pieces = append(pieces, paragraph)
			continue
		}

		for _, sentence := range splitSentences(paragraph) {
			if utf8.RuneCountInString(sentence) <= chunkSize {
				pieces = append(pieces, sentence)
				continue
			}

			// Hard split very long "sentences" (tables, CSV rows, minified text).
			runes := []rune(sentence)
			for start := 0; start < len(runes); start += chunkSize - overlap {
				end := start + chunkSize
				if end > len(runes) {
					end = len(runes)
				}
				pieces = append(pieces, string(runes[start:end]))
				if end == len(runes) {
					break
				}
			}
		}
	}

	// Greedily merge small pieces up to chunkSize.
	var merged []string
	var current string
	flush := func() {
		if strings.TrimSpace(current) != "" {
			merged = append(merged, strings.TrimSpace(current))
		}
		current = ""
	}

	for _, piece := range pieces {
		candidate := current
		if candidate != "" {
			candidate += "\n\n"
		}
		candidate += piece

		if utf8.RuneCountInString(candidate) > chunkSize && current != "" {
			flush()
			current = piece
			continue
		}
		current = candidate
	}
	flush()

	// Add overlap between consecutive chunks so context is preserved.
	if overlap <= 0 || len(merged) <= 1 {
		return merged
	}

	chunks := make([]string, 0, len(merged))
	chunks = append(chunks, merged[0])
	for i := 1; i < len(merged); i++ {
		tail := trailingRunes(chunks[len(chunks)-1], overlap)
		chunk := merged[i]
		if tail != "" {
			chunk = tail + "\n" + chunk
		}
		chunks = append(chunks, chunk)
	}

	return chunks
}

func splitParagraphs(text string) []string {
	return strings.Split(text, "\n\n")
}

func splitSentences(paragraph string) []string {
	lines := strings.Split(paragraph, "\n")
	var sentences []string
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}

		var current strings.Builder
		for _, r := range line {
			current.WriteRune(r)
			if r == '.' || r == '!' || r == '?' || r == ';' {
				sentences = append(sentences, strings.TrimSpace(current.String()))
				current.Reset()
			}
		}
		if s := strings.TrimSpace(current.String()); s != "" {
			sentences = append(sentences, s)
		}
	}
	return sentences
}

func trailingRunes(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[len(runes)-n:])
}

// CosineSimilarity computes cosine similarity between two vectors of
// the same dimension.
func CosineSimilarity(a, b []float64) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}

	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
