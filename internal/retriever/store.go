package retriever

import (
	"github.com/codementor/codementor/internal/embedding"
	"github.com/codementor/codementor/internal/indexer"
)

// SearchResult represents a search result with similarity score
type SearchResult struct {
	Chunk    *indexer.CodeChunk `json:"chunk"`
	Score    float32            `json:"score"`
	Distance float32            `json:"distance"`
}

// VectorStore is the interface for vector storage backends
type VectorStore interface {
	// Insert adds embedded chunks to the store
	Insert(chunks []*embedding.EmbeddedChunk) error

	// Search finds similar chunks to the query embedding
	Search(queryEmbedding []float32, topK int) ([]*SearchResult, error)

	// Delete removes chunks by IDs
	Delete(ids []string) error

	// Clear removes all data
	Clear() error

	// Count returns the number of stored chunks
	Count() int

	// Close closes the store connection
	Close() error
}

