package retriever

import (
	"context"
	"sort"

	"github.com/codementor/codementor/internal/embedding"
	"github.com/codementor/codementor/internal/indexer"
	"github.com/codementor/codementor/internal/llm"
)

// HybridRetriever combines vector search with BM25 keyword search
type HybridRetriever struct {
	vectorStore  VectorStore
	bm25         *BM25
	embedder     *embedding.Embedder
	vectorWeight float32
	bm25Weight   float32
}

// NewHybridRetriever creates a new hybrid retriever
func NewHybridRetriever(store VectorStore, client *llm.Client) *HybridRetriever {
	return &HybridRetriever{
		vectorStore:  store,
		bm25:         NewBM25(),
		embedder:     embedding.NewEmbedder(client),
		vectorWeight: 0.7, // Weight for vector similarity
		bm25Weight:   0.3, // Weight for BM25
	}
}

// Index indexes chunks for both vector and BM25 search
func (h *HybridRetriever) Index(ctx context.Context, chunks []*indexer.CodeChunk, progressFn func(current, total int)) error {
	// Build BM25 index (fast, no network calls)
	h.bm25.Index(chunks)

	// Generate embeddings and store (slow, requires LLM)
	embeddedChunks, err := h.embedder.EmbedChunks(ctx, chunks, progressFn)
	if err != nil {
		return err
	}

	return h.vectorStore.Insert(embeddedChunks)
}

// Search performs hybrid search combining vector and BM25
func (h *HybridRetriever) Search(ctx context.Context, query string, topK int) ([]*SearchResult, error) {
	// Get more candidates from each method, then merge
	candidateK := topK * 3

	// Vector search
	queryEmbedding, err := h.embedder.EmbedChunks(ctx, []*indexer.CodeChunk{{Content: query}}, nil)
	if err != nil {
		// Fall back to BM25 only
		return h.bm25.Search(query, topK), nil
	}

	vectorResults, err := h.vectorStore.Search(queryEmbedding[0].Embedding, candidateK)
	if err != nil {
		return nil, err
	}

	// BM25 search
	bm25Results := h.bm25.Search(query, candidateK)

	// Merge results using Reciprocal Rank Fusion (RRF)
	return h.mergeResults(vectorResults, bm25Results, topK), nil
}

// VectorSearch performs vector-only search
func (h *HybridRetriever) VectorSearch(ctx context.Context, query string, topK int) ([]*SearchResult, error) {
	queryEmbedding, err := h.embedder.EmbedChunks(ctx, []*indexer.CodeChunk{{Content: query}}, nil)
	if err != nil {
		return nil, err
	}

	return h.vectorStore.Search(queryEmbedding[0].Embedding, topK)
}

// KeywordSearch performs BM25-only search
func (h *HybridRetriever) KeywordSearch(query string, topK int) []*SearchResult {
	return h.bm25.Search(query, topK)
}

// mergeResults merges vector and BM25 results using Reciprocal Rank Fusion
func (h *HybridRetriever) mergeResults(vectorResults, bm25Results []*SearchResult, topK int) []*SearchResult {
	const k = 60.0 // RRF constant

	// Create a map to store combined scores
	scoreMap := make(map[string]float32)
	chunkMap := make(map[string]*indexer.CodeChunk)

	// Add vector results with RRF scores
	for rank, result := range vectorResults {
		id := result.Chunk.ID
		score := h.vectorWeight * float32(1.0/(k+float64(rank+1)))
		scoreMap[id] += score
		chunkMap[id] = result.Chunk
	}

	// Add BM25 results with RRF scores
	for rank, result := range bm25Results {
		id := result.Chunk.ID
		score := h.bm25Weight * float32(1.0/(k+float64(rank+1)))
		scoreMap[id] += score
		if _, exists := chunkMap[id]; !exists {
			chunkMap[id] = result.Chunk
		}
	}

	// Convert to slice and sort
	type scored struct {
		id    string
		score float32
	}

	var scores []scored
	for id, score := range scoreMap {
		scores = append(scores, scored{id: id, score: score})
	}

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
			Chunk: chunkMap[scores[i].id],
			Score: scores[i].score,
		}
	}

	return results
}

// SetWeights sets the weights for vector and BM25 search
func (h *HybridRetriever) SetWeights(vectorWeight, bm25Weight float32) {
	h.vectorWeight = vectorWeight
	h.bm25Weight = bm25Weight
}

// GetChunkCount returns the number of indexed chunks
func (h *HybridRetriever) GetChunkCount() int {
	return h.vectorStore.Count()
}

// BuildBM25Index builds only the BM25 index (for when vector data already exists)
func (h *HybridRetriever) BuildBM25Index(chunks []*indexer.CodeChunk) {
	h.bm25.Index(chunks)
}

// Close closes the retriever
func (h *HybridRetriever) Close() error {
	return h.vectorStore.Close()
}
