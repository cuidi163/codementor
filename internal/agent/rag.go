package agent

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/codementor/codementor/internal/config"
	"github.com/codementor/codementor/internal/indexer"
	"github.com/codementor/codementor/internal/llm"
	"github.com/codementor/codementor/internal/retriever"
)

// RAGAgent is the main agent that orchestrates RAG-based code Q&A
type RAGAgent struct {
	config      *config.Config
	llmClient   *llm.Client
	vectorStore retriever.VectorStore
	retriever   *retriever.HybridRetriever
	indexer     *indexer.Indexer
	history     []llm.Message
	repoName    string
	qdrantStore *retriever.QdrantStore // Keep reference for HasData check
}

// NewRAGAgent creates a new RAG agent
func NewRAGAgent(cfg *config.Config) *RAGAgent {
	llmClient := llm.NewClient(cfg.Ollama)

	var store retriever.VectorStore
	var qdrantStore *retriever.QdrantStore

	// Choose vector store based on config
	switch cfg.Vector.Type {
	case "qdrant":
		qdrantHost := fmt.Sprintf("http://%s:%d", cfg.Vector.Host, cfg.Vector.Port)
		var err error
		qdrantStore, err = retriever.NewQdrantStore(qdrantHost, cfg.Vector.Collection, cfg.Vector.Dimension)
		if err != nil {
			fmt.Printf("⚠️  Failed to connect to Qdrant: %v\n", err)
			fmt.Println("   Falling back to memory store")
			dataPath := fmt.Sprintf(".codementor/vectors_%s.json", cfg.Vector.Collection)
			store = retriever.NewMemoryStore(dataPath)
		} else {
			store = qdrantStore
		}
	default:
		// Default to memory store
		dataPath := fmt.Sprintf(".codementor/vectors_%s.json", cfg.Vector.Collection)
		store = retriever.NewMemoryStore(dataPath)
	}

	hybridRetriever := retriever.NewHybridRetriever(store, llmClient)

	return &RAGAgent{
		config:      cfg,
		llmClient:   llmClient,
		vectorStore: store,
		retriever:   hybridRetriever,
		indexer:     indexer.NewIndexer(cfg.Indexer),
		history:     []llm.Message{},
		qdrantStore: qdrantStore,
	}
}

// IndexRepository indexes a code repository
func (a *RAGAgent) IndexRepository(ctx context.Context, repoPath string, progressFn func(stage string, current, total int)) error {
	// Get repo name from path
	absPath, _ := filepath.Abs(repoPath)
	a.repoName = filepath.Base(absPath)

	// Check if we already have data in the store (skip re-indexing)
	existingCount := a.vectorStore.Count()
	if existingCount > 0 {
		fmt.Printf("✅ Found existing index with %d chunks (skipping re-indexing)\n", existingCount)

		// Still need to build BM25 index from existing data
		// For Qdrant, we need to load chunks for BM25
		if a.qdrantStore != nil {
			fmt.Println("   Loading chunks for keyword search...")
			// Parse repo to get chunks for BM25 (fast, no embedding needed)
			result, err := a.indexer.IndexRepository(repoPath)
			if err == nil {
				a.retriever.BuildBM25Index(result.Chunks)
			}
		}
		return nil
	}

	// Stage 1: Parse code files
	if progressFn != nil {
		progressFn("parsing", 0, 0)
	}

	result, err := a.indexer.IndexRepository(repoPath)
	if err != nil {
		return fmt.Errorf("failed to parse repository: %w", err)
	}

	// Stage 2: Generate embeddings and index
	if progressFn != nil {
		progressFn("embedding", 0, len(result.Chunks))
	}

	err = a.retriever.Index(ctx, result.Chunks, func(current, total int) {
		if progressFn != nil {
			progressFn("embedding", current, total)
		}
	})

	if err != nil {
		return fmt.Errorf("failed to index: %w", err)
	}

	return nil
}

// Ask asks a question about the indexed codebase
func (a *RAGAgent) Ask(ctx context.Context, question string) (string, error) {
	// Retrieve relevant code chunks
	results, err := a.retriever.Search(ctx, question, 5)
	if err != nil {
		return "", fmt.Errorf("retrieval failed: %w", err)
	}

	// Build context from retrieved chunks
	context := a.buildContext(results)

	// Build prompt with context
	prompt := a.buildPrompt(question, context)

	// Add to history
	a.history = append(a.history, llm.Message{
		Role:    "user",
		Content: prompt,
	})

	// Generate response
	response, err := a.llmClient.Chat(ctx, a.getMessagesWithSystem())
	if err != nil {
		// Remove failed message from history
		a.history = a.history[:len(a.history)-1]
		return "", fmt.Errorf("generation failed: %w", err)
	}

	// Add response to history
	a.history = append(a.history, llm.Message{
		Role:    "assistant",
		Content: response,
	})

	return response, nil
}

// AskStream asks a question and streams the response
func (a *RAGAgent) AskStream(ctx context.Context, question string, handler llm.StreamHandler) error {
	// Retrieve relevant code chunks
	results, err := a.retriever.Search(ctx, question, 5)
	if err != nil {
		return fmt.Errorf("retrieval failed: %w", err)
	}

	// Build context from retrieved chunks
	codeContext := a.buildContext(results)

	// Build prompt with context
	prompt := a.buildPrompt(question, codeContext)

	// Add to history
	a.history = append(a.history, llm.Message{
		Role:    "user",
		Content: prompt,
	})

	// Stream response
	var fullResponse strings.Builder
	err = a.llmClient.ChatStream(ctx, a.getMessagesWithSystem(), func(content string, done bool) error {
		fullResponse.WriteString(content)
		return handler(content, done)
	})

	if err != nil {
		// Remove failed message from history
		a.history = a.history[:len(a.history)-1]
		return err
	}

	// Add response to history
	a.history = append(a.history, llm.Message{
		Role:    "assistant",
		Content: fullResponse.String(),
	})

	return nil
}

// buildContext builds the context string from retrieved chunks
func (a *RAGAgent) buildContext(results []*retriever.SearchResult) string {
	if len(results) == 0 {
		return "No relevant code found in the repository."
	}

	var sb strings.Builder
	sb.WriteString("Here are relevant code snippets from the repository:\n\n")

	for i, result := range results {
		chunk := result.Chunk

		sb.WriteString(fmt.Sprintf("--- Code Snippet %d ---\n", i+1))
		sb.WriteString(fmt.Sprintf("File: %s (lines %d-%d)\n", chunk.FilePath, chunk.StartLine, chunk.EndLine))

		if chunk.ChunkType != "" {
			sb.WriteString(fmt.Sprintf("Type: %s\n", chunk.ChunkType))
		}
		if chunk.Name != "" && chunk.Name != chunk.FilePath {
			sb.WriteString(fmt.Sprintf("Name: %s\n", chunk.Name))
		}
		if chunk.Signature != "" {
			sb.WriteString(fmt.Sprintf("Signature: %s\n", chunk.Signature))
		}
		if chunk.DocComment != "" {
			sb.WriteString(fmt.Sprintf("Documentation: %s\n", chunk.DocComment))
		}

		sb.WriteString(fmt.Sprintf("```%s\n%s\n```\n\n", chunk.Language, chunk.Content))
	}

	return sb.String()
}

// buildPrompt builds the full prompt with context
func (a *RAGAgent) buildPrompt(question, context string) string {
	return fmt.Sprintf(`Based on the following code context, please answer the question.

%s

Question: %s

Instructions:
1. Answer based on the provided code context
2. If the code doesn't contain enough information, say so
3. Reference specific files and line numbers when relevant
4. Be concise but thorough`, context, question)
}

// getMessagesWithSystem returns messages with system prompt
func (a *RAGAgent) getMessagesWithSystem() []llm.Message {
	systemPrompt := fmt.Sprintf(`You are CodeMentor, an expert code analyst assistant. You help developers understand codebases by analyzing code and answering questions.

Repository: %s

Your responsibilities:
1. Analyze code structure, patterns, and architecture
2. Explain functions, types, and their relationships
3. Answer questions based on the provided code context
4. Reference specific files and line numbers when helpful
5. Admit when information is not available in the provided context

Be accurate, concise, and helpful.`, a.repoName)

	messages := []llm.Message{
		{Role: "system", Content: systemPrompt},
	}

	return append(messages, a.history...)
}

// ClearHistory clears the conversation history
func (a *RAGAgent) ClearHistory() {
	a.history = []llm.Message{}
}

// GetChunkCount returns the number of indexed chunks
func (a *RAGAgent) GetChunkCount() int {
	return a.vectorStore.Count()
}

// CheckHealth checks if the LLM is accessible
func (a *RAGAgent) CheckHealth(ctx context.Context) error {
	return a.llmClient.CheckHealth(ctx)
}

// Close closes the agent and releases resources
func (a *RAGAgent) Close() error {
	return a.vectorStore.Close()
}

// GetRetrievedChunks returns chunks for a query (for debugging/display)
func (a *RAGAgent) GetRetrievedChunks(ctx context.Context, query string, topK int) ([]*retriever.SearchResult, error) {
	return a.retriever.Search(ctx, query, topK)
}

// ClearIndex clears the vector store index
func (a *RAGAgent) ClearIndex() error {
	return a.vectorStore.Clear()
}
