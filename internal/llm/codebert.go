package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// CodeBERTClient is a client for the CodeBERT embedding service
type CodeBERTClient struct {
	host       string
	httpClient *http.Client
}

// CodeBERTEmbeddingRequest represents a request to generate embedding
type CodeBERTEmbeddingRequest struct {
	Text      string `json:"text"`
	MaxLength int    `json:"max_length,omitempty"`
}

// CodeBERTBatchRequest represents a batch embedding request
type CodeBERTBatchRequest struct {
	Texts     []string `json:"texts"`
	MaxLength int      `json:"max_length,omitempty"`
}

// CodeBERTEmbeddingResponse represents an embedding response
type CodeBERTEmbeddingResponse struct {
	Embedding []float32 `json:"embedding"`
	Dimension int       `json:"dimension"`
}

// CodeBERTBatchResponse represents a batch embedding response
type CodeBERTBatchResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
	Dimension  int         `json:"dimension"`
	Count      int         `json:"count"`
}

// CodeBERTHealthResponse represents a health check response
type CodeBERTHealthResponse struct {
	Status    string `json:"status"`
	Model     string `json:"model"`
	Device    string `json:"device"`
	Dimension int    `json:"dimension"`
}

// NewCodeBERTClient creates a new CodeBERT client
func NewCodeBERTClient(host string) *CodeBERTClient {
	return &CodeBERTClient{
		host: host,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// CheckHealth checks if the CodeBERT service is healthy
func (c *CodeBERTClient) CheckHealth(ctx context.Context) (*CodeBERTHealthResponse, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.host+"/health", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("CodeBERT service not accessible at %s: %w", c.host, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("health check failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var health CodeBERTHealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &health, nil
}

// Embed generates embedding for a single text
func (c *CodeBERTClient) Embed(ctx context.Context, text string) ([]float32, error) {
	reqBody := CodeBERTEmbeddingRequest{
		Text:      text,
		MaxLength: 512,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.host+"/embed", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("embedding failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var embResp CodeBERTEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return embResp.Embedding, nil
}

// EmbedBatch generates embeddings for multiple texts
func (c *CodeBERTClient) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	reqBody := CodeBERTBatchRequest{
		Texts:     texts,
		MaxLength: 512,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.host+"/embed/batch", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("batch embedding failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var batchResp CodeBERTBatchResponse
	if err := json.NewDecoder(resp.Body).Decode(&batchResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return batchResp.Embeddings, nil
}

// GetDimension returns the embedding dimension
func (c *CodeBERTClient) GetDimension(ctx context.Context) (int, error) {
	health, err := c.CheckHealth(ctx)
	if err != nil {
		return 0, err
	}
	return health.Dimension, nil
}
