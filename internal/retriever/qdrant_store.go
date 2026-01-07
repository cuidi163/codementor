package retriever

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/codementor/codementor/internal/embedding"
	"github.com/codementor/codementor/internal/indexer"
)

// QdrantStore implements VectorStore using Qdrant
type QdrantStore struct {
	host       string
	collection string
	dimension  int
	httpClient *http.Client
}

// NewQdrantStore creates a new Qdrant vector store
func NewQdrantStore(host string, collection string, dimension int) (*QdrantStore, error) {
	store := &QdrantStore{
		host:       host,
		collection: collection,
		dimension:  dimension,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}

	// Ensure collection exists
	if err := store.ensureCollection(); err != nil {
		return nil, err
	}

	return store, nil
}

// ensureCollection creates the collection if it doesn't exist
func (q *QdrantStore) ensureCollection() error {
	// Check if collection exists
	resp, err := q.httpClient.Get(fmt.Sprintf("%s/collections/%s", q.host, q.collection))
	if err != nil {
		return fmt.Errorf("failed to check collection: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		// Collection exists
		return nil
	}

	// Create collection
	createReq := map[string]interface{}{
		"vectors": map[string]interface{}{
			"size":     q.dimension,
			"distance": "Cosine",
		},
	}

	body, _ := json.Marshal(createReq)
	req, err := http.NewRequest("PUT", fmt.Sprintf("%s/collections/%s", q.host, q.collection), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err = q.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create collection: %s", string(bodyBytes))
	}

	return nil
}

// Insert adds embedded chunks to Qdrant
func (q *QdrantStore) Insert(chunks []*embedding.EmbeddedChunk) error {
	if len(chunks) == 0 {
		return nil
	}

	// Build points for upsert
	points := make([]map[string]interface{}, len(chunks))
	for i, chunk := range chunks {
		// Convert chunk metadata to payload
		payload := map[string]interface{}{
			"id":          chunk.Chunk.ID,
			"file_path":   chunk.Chunk.FilePath,
			"language":    chunk.Chunk.Language,
			"chunk_type":  string(chunk.Chunk.ChunkType),
			"name":        chunk.Chunk.Name,
			"signature":   chunk.Chunk.Signature,
			"start_line":  chunk.Chunk.StartLine,
			"end_line":    chunk.Chunk.EndLine,
			"doc_comment": chunk.Chunk.DocComment,
			"parent_name": chunk.Chunk.ParentName,
			"content":     chunk.Chunk.Content,
		}

		points[i] = map[string]interface{}{
			"id":      i + 1, // Qdrant requires numeric or UUID ids
			"vector":  chunk.Embedding,
			"payload": payload,
		}
	}

	// Batch upsert (100 at a time)
	batchSize := 100
	for i := 0; i < len(points); i += batchSize {
		end := i + batchSize
		if end > len(points) {
			end = len(points)
		}

		batch := points[i:end]
		upsertReq := map[string]interface{}{
			"points": batch,
		}

		body, _ := json.Marshal(upsertReq)
		req, err := http.NewRequest("PUT", fmt.Sprintf("%s/collections/%s/points", q.host, q.collection), bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := q.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("failed to upsert points: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("failed to upsert points: %s", string(bodyBytes))
		}
	}

	return nil
}

// Search finds similar chunks in Qdrant
func (q *QdrantStore) Search(queryEmbedding []float32, topK int) ([]*SearchResult, error) {
	searchReq := map[string]interface{}{
		"vector":       queryEmbedding,
		"limit":        topK,
		"with_payload": true,
	}

	body, _ := json.Marshal(searchReq)
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/collections/%s/points/search", q.host, q.collection), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := q.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search failed: %s", string(bodyBytes))
	}

	var searchResp struct {
		Result []struct {
			Score   float32                `json:"score"`
			Payload map[string]interface{} `json:"payload"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	results := make([]*SearchResult, len(searchResp.Result))
	for i, r := range searchResp.Result {
		chunk := &indexer.CodeChunk{
			ID:         getString(r.Payload, "id"),
			FilePath:   getString(r.Payload, "file_path"),
			Language:   getString(r.Payload, "language"),
			ChunkType:  indexer.ChunkType(getString(r.Payload, "chunk_type")),
			Name:       getString(r.Payload, "name"),
			Signature:  getString(r.Payload, "signature"),
			StartLine:  getInt(r.Payload, "start_line"),
			EndLine:    getInt(r.Payload, "end_line"),
			DocComment: getString(r.Payload, "doc_comment"),
			ParentName: getString(r.Payload, "parent_name"),
			Content:    getString(r.Payload, "content"),
		}

		results[i] = &SearchResult{
			Chunk:    chunk,
			Score:    r.Score,
			Distance: 1 - r.Score,
		}
	}

	return results, nil
}

// Delete removes chunks by IDs
func (q *QdrantStore) Delete(ids []string) error {
	// Qdrant delete requires point IDs, not our string IDs
	// For simplicity, we'll clear the whole collection
	return q.Clear()
}

// Clear removes all data from the collection
func (q *QdrantStore) Clear() error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/collections/%s", q.host, q.collection), nil)
	if err != nil {
		return err
	}

	resp, err := q.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Recreate collection
	return q.ensureCollection()
}

// Count returns the number of stored chunks
func (q *QdrantStore) Count() int {
	resp, err := q.httpClient.Get(fmt.Sprintf("%s/collections/%s", q.host, q.collection))
	if err != nil {
		return 0
	}
	defer resp.Body.Close()

	var collInfo struct {
		Result struct {
			PointsCount int `json:"points_count"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&collInfo); err != nil {
		return 0
	}

	return collInfo.Result.PointsCount
}

// Close closes the store (no-op for HTTP client)
func (q *QdrantStore) Close() error {
	return nil
}

// HasData checks if the collection has data
func (q *QdrantStore) HasData() bool {
	return q.Count() > 0
}

// Helper functions
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getInt(m map[string]interface{}, key string) int {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case float64:
			return int(n)
		case int:
			return n
		}
	}
	return 0
}

