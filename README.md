# CodeMentor

<p align="center">
  <strong>🤖 AI-Powered Code Repository Assistant using RAG</strong>
</p>

<p align="center">
  <a href="#features">Features</a> •
  <a href="#quick-start">Quick Start</a> •
  <a href="#architecture">Architecture</a> •
  <a href="#api">API</a> •
  <a href="#tech-highlights">Tech Highlights</a>
</p>

---

CodeMentor is an intelligent code assistant that can "read" and understand your codebase, then answer questions about code structure, functions, architecture, and more. Built with **Go** and powered by **RAG (Retrieval-Augmented Generation)**.

## Features

- 🔍 **Intelligent Code Indexing** - AST-based semantic chunking for Go (function/struct level)
- 🧠 **RAG-Powered Q&A** - Retrieval-Augmented Generation for accurate code understanding
- 🔄 **Hybrid Retrieval** - Vector similarity + BM25 keyword search for better recall
- 💬 **Interactive Chat** - Multi-turn conversations with context awareness
- 🚀 **Streaming Responses** - Real-time SSE streaming for better UX
- 🔌 **Local LLM Support** - Works with Ollama (Qwen2.5, Llama3, etc.)
- 🧬 **CodeBERT Embedding** - Microsoft's code-specialized embedding model for better code understanding
- 📦 **Qdrant Vector DB** - Production-grade vector database with HNSW indexing
- 🐳 **Docker Ready** - One-command deployment with microservices
- 🌐 **REST API** - Full HTTP API for programmatic access

## Quick Start

### Prerequisites

1. **Go 1.21+** installed
2. **Ollama** running locally:
   ```bash
   # Install Ollama from https://ollama.ai
   ollama serve
   
   # Pull required models
   ollama pull llama3.2            # Chat model (3B, recommended)
   ollama pull nomic-embed-text    # Embedding model (274MB, fast)
   
   # Alternative smaller chat model (faster but lower quality)
   # ollama pull qwen2.5:0.5b
   ```

### Installation

```bash
# Clone the repository
git clone https://github.com/yourusername/codementor.git
cd codementor

# Build
go build -o codementor ./cmd/codementor

# Verify installation
./codementor version
```

### Usage

#### Interactive RAG Chat

```bash
# Start a RAG-enabled chat session with your repository
./codementor chat --path /path/to/your/repo

# Example questions:
# > What does the main function do?
# > Explain the Config struct
# > How does the indexer work?
```

#### Single Question

```bash
./codementor ask --path . "What are the main components of this project?"
```

#### Search Code Chunks

```bash
./codementor search --path . "embedding generation"
```

#### HTTP API Server

```bash
./codementor serve

# In another terminal:
curl http://localhost:8080/health
```

### Docker Deployment (Full Stack)

```bash
# Make sure Ollama is running on host
ollama serve
ollama pull qwen2.5:0.5b  # or llama3.2

# Start all services (CodeBERT + Qdrant + CodeMentor API)
docker-compose up -d

# Wait for CodeBERT to load (~2 minutes first time)
# Check services:
curl http://localhost:8001/health  # CodeBERT
curl http://localhost:6333/        # Qdrant
curl http://localhost:8080/health  # CodeMentor API
```

### Local Development (without Docker)

```bash
# 1. Start Qdrant
docker run -d -p 6333:6333 --name qdrant qdrant/qdrant

# 2. Start CodeBERT service
cd services/codebert
pip install -r requirements.txt
python main.py  # Runs on port 8000

# 3. Run CodeMentor
./codementor chat --path /path/to/repo
```

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                           CodeMentor                                 │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌──────────┐     ┌──────────┐     ┌──────────┐     ┌──────────┐   │
│  │   CLI    │     │ HTTP API │     │   SSE    │     │  Search  │   │
│  │  Client  │     │  Server  │     │ Streaming│     │  Debug   │   │
│  └────┬─────┘     └────┬─────┘     └────┬─────┘     └────┬─────┘   │
│       │                │                │                │         │
│       └────────────────┼────────────────┼────────────────┘         │
│                        │                │                          │
│                        ▼                ▼                          │
│              ┌─────────────────────────────────────┐               │
│              │           RAG Agent                  │               │
│              │  ┌─────────────────────────────┐    │               │
│              │  │    Hybrid Retriever         │    │               │
│              │  │  ┌─────────┐  ┌─────────┐   │    │               │
│              │  │  │ Vector  │  │  BM25   │   │    │               │
│              │  │  │ Search  │  │ Search  │   │    │               │
│              │  │  └────┬────┘  └────┬────┘   │    │               │
│              │  │       └──────┬─────┘        │    │               │
│              │  │              │ RRF Fusion   │    │               │
│              │  └──────────────┼──────────────┘    │               │
│              └─────────────────┼───────────────────┘               │
│                                │                                   │
│       ┌────────────────────────┼────────────────────────┐          │
│       │                        │                        │          │
│       ▼                        ▼                        ▼          │
│  ┌─────────┐           ┌─────────────┐           ┌─────────┐       │
│  │ Indexer │           │ Vector Store│           │  Ollama │       │
│  │  (AST)  │           │  (Memory)   │           │   LLM   │       │
│  └─────────┘           └─────────────┘           └─────────┘       │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### Data Flow

1. **Indexing Phase**:
   - Scan repository for source files
   - Parse using AST (Go) or generic chunking (other languages)
   - Extract metadata (function names, signatures, comments)
   - Generate embeddings via Ollama
   - Store in vector database with BM25 index

2. **Query Phase**:
   - Generate query embedding
   - Perform hybrid search (Vector + BM25)
   - Merge results using Reciprocal Rank Fusion (RRF)
   - Build context-aware prompt with retrieved code
   - Stream response from LLM

## Project Structure

```
codementor/
├── cmd/codementor/           # CLI entry point
│   └── main.go
├── internal/
│   ├── config/               # Configuration management (Viper)
│   ├── indexer/              # Code parsing and chunking
│   │   ├── parser_go.go      # Go AST parser
│   │   ├── chunker.go        # Generic text chunking
│   │   └── scanner.go        # File system scanner
│   ├── embedding/            # Vector embedding service
│   ├── retriever/            # Search and retrieval
│   │   ├── hybrid.go         # Hybrid retriever
│   │   ├── memory_store.go   # In-memory vector store
│   │   └── bm25.go           # BM25 implementation
│   ├── llm/                  # LLM client (Ollama)
│   ├── agent/                # RAG agent orchestration
│   └── api/                  # HTTP API server (Gin)
├── configs/                  # Configuration files
├── Dockerfile
├── docker-compose.yml
└── README.md
```

## API Reference

### Health Check

```bash
GET /health
```

### Create Session

```bash
POST /api/v1/sessions
Content-Type: application/json

{
  "repo_path": "/path/to/repository"
}
```

### Chat (Non-Streaming)

```bash
POST /api/v1/chat
Content-Type: application/json

{
  "session_id": "session_xxx",
  "message": "What does the main function do?"
}
```

### Chat (Streaming SSE)

```bash
GET /api/v1/chat/stream?session_id=xxx&message=What+does+main+do
```

### Search Code

```bash
POST /api/v1/search
Content-Type: application/json

{
  "session_id": "session_xxx",
  "query": "database connection",
  "top_k": 5
}
```

## Tech Highlights

### 1. CodeBERT Embedding (Microservice Architecture)

Using Microsoft's CodeBERT model for code-specialized embeddings:

```
┌─────────────┐     HTTP/REST     ┌─────────────────┐
│  Go Client  │ ───────────────▶ │ CodeBERT Service │
│ (codementor)│                   │   (FastAPI)      │
└─────────────┘                   └─────────────────┘
                                         │
                                         ▼
                                  ┌─────────────────┐
                                  │ microsoft/      │
                                  │ codebert-base   │
                                  └─────────────────┘
```

**Why CodeBERT over general text models?**
- Trained on code + natural language pairs
- Understands code syntax and semantics natively
- Better retrieval accuracy for code-related queries

### 2. AST-Based Intelligent Chunking

Unlike simple character-based splitting, CodeMentor uses Go's `go/ast` package to parse code at the semantic level:

```go
// Chunks are created per function/struct, preserving semantic boundaries
type CodeChunk struct {
    ChunkType  string  // function, method, struct, interface
    Name       string  // Function/struct name
    Signature  string  // Full signature
    DocComment string  // Associated documentation
    // ...
}
```

**Why it matters**: Semantic chunking improves retrieval accuracy because each chunk represents a complete, meaningful code unit.

### 2. Hybrid Retrieval (Vector + BM25)

Combines two retrieval methods for better recall:

- **Vector Search**: Semantic similarity via embeddings
- **BM25**: Keyword matching for exact terms (function names, variables)
- **RRF Fusion**: Reciprocal Rank Fusion to merge results

```go
// Merge using Reciprocal Rank Fusion
score = vectorWeight * (1/(k + vectorRank)) + bm25Weight * (1/(k + bm25Rank))
```

**Why it matters**: Vector search excels at semantic similarity but may miss exact matches. BM25 catches keyword matches. Together they provide better coverage.

### 3. Production-Grade Streaming

Real-time SSE (Server-Sent Events) implementation for responsive UX:

```go
// Stream response tokens as they're generated
err := agent.AskStream(ctx, question, func(content string, done bool) error {
    c.SSEvent("message", content)
    c.Writer.Flush()
    return nil
})
```

### 4. Local-First Architecture

No external API dependencies - runs entirely on local hardware:

- **Ollama** for LLM inference (supports Qwen2.5, Llama3, Mistral, etc.)
- **In-memory vector store** with persistence
- **Zero cloud costs** for inference

### 5. Clean Architecture

- Clear separation of concerns (indexer, retriever, agent, api)
- Interface-based design for vector stores
- Configuration via Viper (files, env vars)
- Comprehensive error handling

## Configuration

### Config File (configs/config.yaml)

```yaml
ollama:
  host: "http://localhost:11434"
  chat_model: "llama3.2:latest"       # or qwen2.5:0.5b for faster response
  embedding_model: "nomic-embed-text" # lightweight and fast
  timeout: 120

vector:
  type: "memory"
  collection: "codementor"
  dimension: 768

indexer:
  chunk_size: 1000
  extensions: [".go", ".py", ".js", ".ts"]
  ignore_dirs: [".git", "node_modules", "vendor"]

server:
  host: "0.0.0.0"
  port: 8080
```

### Environment Variables

```bash
export CODEMENTOR_OLLAMA_HOST="http://localhost:11434"
export CODEMENTOR_OLLAMA_CHAT_MODEL="qwen2.5:7b"
export CODEMENTOR_OLLAMA_EMBEDDING_MODEL="nomic-embed-text"
```

## Development

```bash
# Run tests
go test ./...

# Build
go build -o codementor ./cmd/codementor

# Run with debug output
./codementor chat --path . 
```

## Roadmap

- [x] Core RAG pipeline
- [x] Go AST parsing
- [x] Hybrid retrieval (Vector + BM25)
- [x] HTTP API with SSE streaming
- [x] Docker deployment
- [ ] Support for more languages (Python AST, TypeScript)
- [ ] Milvus/Qdrant integration for production vector storage
- [ ] Web UI
- [ ] Multi-repository support
- [ ] Conversation summarization for long sessions

## License

MIT License

---

<p align="center">
  Built with ❤️ in Go
</p>
