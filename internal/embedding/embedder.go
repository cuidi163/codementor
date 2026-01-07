package embedding

import (
	"context"
	"fmt"
	"sync"

	"github.com/codementor/codementor/internal/indexer"
	"github.com/codementor/codementor/internal/llm"
)

// Embedder generates embeddings for code chunks
type Embedder struct {
	client      *llm.Client
	batchSize   int
	concurrency int
}

// NewEmbedder creates a new embedder
func NewEmbedder(client *llm.Client) *Embedder {
	return &Embedder{
		client:      client,
		batchSize:   10,
		concurrency: 5,
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
	var embedErrors []error
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

			// Generate embedding
			embedding, err := e.client.Embed(ctx, text)
			if err != nil {
				errMu.Lock()
				embedErrors = append(embedErrors, fmt.Errorf("chunk %s: %w", c.ID, err))
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

	if len(embedErrors) > 0 {
		// Return partial results with error info
		return results, fmt.Errorf("failed to embed %d chunks", len(embedErrors))
	}

	return results, nil
}

// createEmbeddingText creates the text to embed for a chunk
// This includes relevant metadata to improve retrieval quality
func (e *Embedder) createEmbeddingText(chunk *indexer.CodeChunk) string {
	var text string

	// Add context based on chunk type
	switch chunk.ChunkType {
	case indexer.ChunkTypeFunction, indexer.ChunkTypeMethod:
		text = fmt.Sprintf("File: %s\nType: %s\n", chunk.FilePath, chunk.ChunkType)
		if chunk.Signature != "" {
			text += fmt.Sprintf("Signature: %s\n", chunk.Signature)
		}
		if chunk.DocComment != "" {
			text += fmt.Sprintf("Documentation: %s\n", chunk.DocComment)
		}
		if chunk.ParentName != "" {
			text += fmt.Sprintf("Belongs to: %s\n", chunk.ParentName)
		}
		text += fmt.Sprintf("Code:\n%s", chunk.Content)

	case indexer.ChunkTypeStruct, indexer.ChunkTypeInterface:
		text = fmt.Sprintf("File: %s\nType: %s\nName: %s\n", chunk.FilePath, chunk.ChunkType, chunk.Name)
		if chunk.DocComment != "" {
			text += fmt.Sprintf("Documentation: %s\n", chunk.DocComment)
		}
		text += fmt.Sprintf("Definition:\n%s", chunk.Content)

	default:
		text = fmt.Sprintf("File: %s\n%s", chunk.FilePath, chunk.Content)
	}

	return text
}

// GetEmbeddingDimension returns the dimension of embeddings
func (e *Embedder) GetEmbeddingDimension(ctx context.Context) (int, error) {
	// Generate a test embedding to get dimension
	testEmbed, err := e.client.Embed(ctx, "test")
	if err != nil {
		return 0, err
	}
	return len(testEmbed), nil
}

