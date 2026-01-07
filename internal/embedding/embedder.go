package embedding

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/codementor/codementor/internal/indexer"
)

// Embedder generates embeddings for code chunks
type Embedder struct {
	provider    Provider
	batchSize   int
	concurrency int
	maxRetries  int
}

// NewEmbedder creates a new embedder with a provider
func NewEmbedder(provider Provider) *Embedder {
	return &Embedder{
		provider:    provider,
		batchSize:   10,
		concurrency: 2, // Lower concurrency to avoid overloading
		maxRetries:  3,
	}
}

// EmbeddedChunk represents a code chunk with its embedding
type EmbeddedChunk struct {
	Chunk     *indexer.CodeChunk `json:"chunk"`
	Embedding []float32          `json:"embedding"`
}

// EmbedChunks generates embeddings for multiple chunks
func (e *Embedder) EmbedChunks(ctx context.Context, chunks []*indexer.CodeChunk, progressFn func(current, total int)) ([]*EmbeddedChunk, error) {
	var results []*EmbeddedChunk
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Create a semaphore for concurrency control
	sem := make(chan struct{}, e.concurrency)

	// Track errors
	var failedCount int
	var errMu sync.Mutex

	total := len(chunks)

	for i, chunk := range chunks {
		wg.Add(1)
		go func(idx int, c *indexer.CodeChunk) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			// Create embedding text from chunk
			text := e.createEmbeddingText(c)

			// Generate embedding with retry
			var embedding []float32
			var err error

			for retry := 0; retry < e.maxRetries; retry++ {
				embedding, err = e.provider.Embed(ctx, text)
				if err == nil {
					break
				}
				// Wait before retry
				if retry < e.maxRetries-1 {
					time.Sleep(time.Duration(retry+1) * 500 * time.Millisecond)
				}
			}

			if err != nil {
				errMu.Lock()
				failedCount++
				errMu.Unlock()
				return
			}

			mu.Lock()
			results = append(results, &EmbeddedChunk{
				Chunk:     c,
				Embedding: embedding,
			})

			// Report progress
			if progressFn != nil {
				progressFn(len(results), total)
			}
			mu.Unlock()
		}(i, chunk)
	}

	wg.Wait()

	// Allow partial failures if success rate > 50%
	successRate := float64(len(results)) / float64(total)
	if successRate < 0.5 {
		return results, fmt.Errorf("too many failures: only %d/%d chunks embedded (%.0f%%)", len(results), total, successRate*100)
	}

	if failedCount > 0 {
		fmt.Printf("\n⚠️  Warning: %d chunks failed to embed, continuing with %d chunks\n", failedCount, len(results))
	}

	return results, nil
}

// createEmbeddingText creates the text to embed for a chunk
// For CodeBERT, we use a code-focused format
func (e *Embedder) createEmbeddingText(chunk *indexer.CodeChunk) string {
	var text string

	// For CodeBERT, focus more on the code itself with minimal metadata
	// CodeBERT understands code structure natively
	switch chunk.ChunkType {
	case indexer.ChunkTypeFunction, indexer.ChunkTypeMethod:
		// Include signature and doc comment as they provide semantic context
		if chunk.DocComment != "" {
			text = fmt.Sprintf("// %s\n", chunk.DocComment)
		}
		if chunk.Signature != "" {
			text += chunk.Signature + "\n"
		}
		text += chunk.Content

	case indexer.ChunkTypeStruct, indexer.ChunkTypeInterface:
		if chunk.DocComment != "" {
			text = fmt.Sprintf("// %s\n", chunk.DocComment)
		}
		text += chunk.Content

	default:
		// For other types, include file path for context
		text = fmt.Sprintf("// File: %s\n%s", chunk.FilePath, chunk.Content)
	}

	// Limit text length to avoid exceeding model limits
	if len(text) > 8000 {
		text = text[:8000]
	}

	return text
}

// GetProvider returns the underlying provider
func (e *Embedder) GetProvider() Provider {
	return e.provider
}

// GetDimension returns the embedding dimension
func (e *Embedder) GetDimension() int {
	return e.provider.GetDimension()
}
