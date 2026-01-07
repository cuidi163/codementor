package embedding

import (
	"context"
	"fmt"

	"github.com/codementor/codementor/internal/config"
	"github.com/codementor/codementor/internal/llm"
)

// Provider is the interface for embedding providers
type Provider interface {
	// Embed generates embedding for a single text
	Embed(ctx context.Context, text string) ([]float32, error)

	// EmbedBatch generates embeddings for multiple texts
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)

	// CheckHealth checks if the provider is healthy
	CheckHealth(ctx context.Context) error

	// GetDimension returns the embedding dimension
	GetDimension() int

	// Name returns the provider name
	Name() string
}

// OllamaProvider wraps Ollama client as embedding provider
type OllamaProvider struct {
	client    *llm.Client
	dimension int
}

// NewOllamaProvider creates a new Ollama embedding provider
func NewOllamaProvider(cfg config.OllamaConfig) *OllamaProvider {
	return &OllamaProvider{
		client:    llm.NewClient(cfg),
		dimension: 768, // nomic-embed-text dimension
	}
}

func (p *OllamaProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	return p.client.Embed(ctx, text)
}

func (p *OllamaProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	return p.client.EmbedBatch(ctx, texts)
}

func (p *OllamaProvider) CheckHealth(ctx context.Context) error {
	return p.client.CheckHealth(ctx)
}

func (p *OllamaProvider) GetDimension() int {
	return p.dimension
}

func (p *OllamaProvider) Name() string {
	return "ollama"
}

// CodeBERTProvider wraps CodeBERT client as embedding provider
type CodeBERTProvider struct {
	client    *llm.CodeBERTClient
	dimension int
}

// NewCodeBERTProvider creates a new CodeBERT embedding provider
func NewCodeBERTProvider(host string) *CodeBERTProvider {
	return &CodeBERTProvider{
		client:    llm.NewCodeBERTClient(host),
		dimension: 768, // CodeBERT dimension
	}
}

func (p *CodeBERTProvider) Embed(ctx context.Context, text string) ([]float32, error) {
	return p.client.Embed(ctx, text)
}

func (p *CodeBERTProvider) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	return p.client.EmbedBatch(ctx, texts)
}

func (p *CodeBERTProvider) CheckHealth(ctx context.Context) error {
	_, err := p.client.CheckHealth(ctx)
	return err
}

func (p *CodeBERTProvider) GetDimension() int {
	return p.dimension
}

func (p *CodeBERTProvider) Name() string {
	return "codebert"
}

// NewProvider creates an embedding provider based on configuration
func NewProvider(cfg *config.Config) (Provider, error) {
	switch cfg.Embedding.Provider {
	case "codebert":
		return NewCodeBERTProvider(cfg.Embedding.Host), nil
	case "ollama":
		return NewOllamaProvider(cfg.Ollama), nil
	default:
		return nil, fmt.Errorf("unknown embedding provider: %s", cfg.Embedding.Provider)
	}
}

