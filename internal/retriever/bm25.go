package retriever

import (
	"math"
	"regexp"
	"sort"
	"strings"

	"github.com/codementor/codementor/internal/indexer"
)

// BM25 implements the BM25 ranking algorithm for keyword search
type BM25 struct {
	k1          float64
	b           float64
	chunks      []*indexer.CodeChunk
	docLengths  []int
	avgDocLen   float64
	termFreqs   []map[string]int
	docFreqs    map[string]int
	totalDocs   int
	tokenRegex  *regexp.Regexp
}

// NewBM25 creates a new BM25 index
func NewBM25() *BM25 {
	return &BM25{
		k1:         1.5,
		b:          0.75,
		docFreqs:   make(map[string]int),
		tokenRegex: regexp.MustCompile(`[a-zA-Z_][a-zA-Z0-9_]*`),
	}
}

// Index builds the BM25 index from chunks
func (b *BM25) Index(chunks []*indexer.CodeChunk) {
	b.chunks = chunks
	b.totalDocs = len(chunks)
	b.termFreqs = make([]map[string]int, len(chunks))
	b.docLengths = make([]int, len(chunks))
	b.docFreqs = make(map[string]int)

	totalLen := 0

	for i, chunk := range chunks {
		tokens := b.tokenize(chunk.Content)
		b.docLengths[i] = len(tokens)
		totalLen += len(tokens)

		// Count term frequencies
		tf := make(map[string]int)
		seenTerms := make(map[string]bool)

		for _, token := range tokens {
			tf[token]++
			if !seenTerms[token] {
				b.docFreqs[token]++
				seenTerms[token] = true
			}
		}
		b.termFreqs[i] = tf
	}

	if len(chunks) > 0 {
		b.avgDocLen = float64(totalLen) / float64(len(chunks))
	}
}

// Search performs BM25 search
func (b *BM25) Search(query string, topK int) []*SearchResult {
	if b.totalDocs == 0 {
		return nil
	}

	queryTokens := b.tokenize(query)
	if len(queryTokens) == 0 {
		return nil
	}

	// Calculate BM25 scores
	type scored struct {
		idx   int
		score float64
	}

	var scores []scored
	for i := range b.chunks {
		score := b.score(queryTokens, i)
		if score > 0 {
			scores = append(scores, scored{idx: i, score: score})
		}
	}

	// Sort by score descending
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})

	// Take top K
	if topK > len(scores) {
		topK = len(scores)
	}

	results := make([]*SearchResult, topK)
	for i := 0; i < topK; i++ {
		results[i] = &SearchResult{
			Chunk: b.chunks[scores[i].idx],
			Score: float32(scores[i].score),
		}
	}

	return results
}

// score calculates BM25 score for a document
func (bm *BM25) score(queryTokens []string, docIdx int) float64 {
	var score float64

	docLen := float64(bm.docLengths[docIdx])
	tf := bm.termFreqs[docIdx]

	for _, term := range queryTokens {
		termFreq, exists := tf[term]
		if !exists {
			continue
		}

		docFreq := bm.docFreqs[term]
		if docFreq == 0 {
			continue
		}

		// IDF component
		idf := math.Log((float64(bm.totalDocs)-float64(docFreq)+0.5)/(float64(docFreq)+0.5) + 1)

		// TF component with length normalization
		tfNorm := (float64(termFreq) * (bm.k1 + 1)) /
			(float64(termFreq) + bm.k1*(1-bm.b+bm.b*docLen/bm.avgDocLen))

		score += idf * tfNorm
	}

	return score
}

// tokenize splits text into tokens (identifiers)
func (b *BM25) tokenize(text string) []string {
	text = strings.ToLower(text)

	// Extract identifiers
	matches := b.tokenRegex.FindAllString(text, -1)

	// Also split camelCase and snake_case
	var tokens []string
	for _, match := range matches {
		// Split camelCase
		camelTokens := splitCamelCase(match)
		for _, t := range camelTokens {
			t = strings.ToLower(t)
			if len(t) > 1 { // Skip single characters
				tokens = append(tokens, t)
			}
		}
	}

	return tokens
}

// splitCamelCase splits a camelCase string into words
func splitCamelCase(s string) []string {
	var result []string
	var current strings.Builder

	for i, r := range s {
		if i > 0 && (isUpper(byte(r)) || r == '_') {
			if current.Len() > 0 {
				result = append(result, current.String())
				current.Reset()
			}
			if r == '_' {
				continue
			}
		}
		current.WriteRune(r)
	}

	if current.Len() > 0 {
		result = append(result, current.String())
	}

	return result
}

func isUpper(b byte) bool {
	return b >= 'A' && b <= 'Z'
}

