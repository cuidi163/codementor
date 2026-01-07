package retriever

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/codementor/codementor/internal/embedding"
	"github.com/codementor/codementor/internal/indexer"
)

// MemoryStore is an in-memory vector store with persistence
type MemoryStore struct {
	mu       sync.RWMutex
	chunks   map[string]*embedding.EmbeddedChunk
	dataPath string
}

// NewMemoryStore creates a new in-memory vector store
func NewMemoryStore(dataPath string) *MemoryStore {
	store := &MemoryStore{
		chunks:   make(map[string]*embedding.EmbeddedChunk),
		dataPath: dataPath,
	}

	// Try to load existing data
	if dataPath != "" {
		_ = store.load()
	}

	return store
}

// Insert adds embedded chunks to the store
func (m *MemoryStore) Insert(chunks []*embedding.EmbeddedChunk) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, chunk := range chunks {
		m.chunks[chunk.Chunk.ID] = chunk
	}

	// Persist if dataPath is set
	if m.dataPath != "" {
		return m.save()
	}

	return nil
}

// Search finds similar chunks using cosine similarity
func (m *MemoryStore) Search(queryEmbedding []float32, topK int) ([]*SearchResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.chunks) == 0 {
		return nil, nil
	}

	// Calculate similarity for all chunks
	type scoredChunk struct {
		chunk *indexer.CodeChunk
		score float32
	}

	var scored []scoredChunk
	for _, embeddedChunk := range m.chunks {
		score := cosineSimilarity(queryEmbedding, embeddedChunk.Embedding)
		scored = append(scored, scoredChunk{
			chunk: embeddedChunk.Chunk,
			score: score,
		})
	}

	// Sort by score descending
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Take top K
	if topK > len(scored) {
		topK = len(scored)
	}

	results := make([]*SearchResult, topK)
	for i := 0; i < topK; i++ {
		results[i] = &SearchResult{
			Chunk:    scored[i].chunk,
			Score:    scored[i].score,
			Distance: 1 - scored[i].score, // Convert similarity to distance
		}
	}

	return results, nil
}

// Delete removes chunks by IDs
func (m *MemoryStore) Delete(ids []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, id := range ids {
		delete(m.chunks, id)
	}

	if m.dataPath != "" {
		return m.save()
	}

	return nil
}

// Clear removes all data
func (m *MemoryStore) Clear() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.chunks = make(map[string]*embedding.EmbeddedChunk)

	if m.dataPath != "" {
		return os.Remove(m.dataPath)
	}

	return nil
}

// Count returns the number of stored chunks
func (m *MemoryStore) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.chunks)
}

// Close closes the store
func (m *MemoryStore) Close() error {
	if m.dataPath != "" {
		return m.save()
	}
	return nil
}

// save persists the store to disk
func (m *MemoryStore) save() error {
	// Ensure directory exists
	dir := filepath.Dir(m.dataPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Convert to slice for JSON
	var chunks []*embedding.EmbeddedChunk
	for _, chunk := range m.chunks {
		chunks = append(chunks, chunk)
	}

	data, err := json.Marshal(chunks)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	if err := os.WriteFile(m.dataPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// load loads the store from disk
func (m *MemoryStore) load() error {
	data, err := os.ReadFile(m.dataPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var chunks []*embedding.EmbeddedChunk
	if err := json.Unmarshal(data, &chunks); err != nil {
		return err
	}

	for _, chunk := range chunks {
		m.chunks[chunk.Chunk.ID] = chunk
	}

	return nil
}

// cosineSimilarity calculates the cosine similarity between two vectors
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return float32(dotProduct / (math.Sqrt(normA) * math.Sqrt(normB)))
}

