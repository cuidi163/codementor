package api

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/codementor/codementor/internal/agent"
	"github.com/codementor/codementor/internal/config"
	"github.com/gin-gonic/gin"
)

// Server represents the HTTP API server
type Server struct {
	config   *config.Config
	router   *gin.Engine
	sessions map[string]*Session
	mu       sync.RWMutex
}

// Session represents a chat session with RAG agent
type Session struct {
	ID        string
	Agent     *agent.RAGAgent
	RepoPath  string
	CreatedAt time.Time
	LastUsed  time.Time
}

// NewServer creates a new API server
func NewServer(cfg *config.Config) *Server {
	gin.SetMode(gin.ReleaseMode)

	s := &Server{
		config:   cfg,
		router:   gin.New(),
		sessions: make(map[string]*Session),
	}

	s.setupRoutes()
	return s
}

// setupRoutes configures all API routes
func (s *Server) setupRoutes() {
	s.router.Use(gin.Recovery())
	s.router.Use(corsMiddleware())

	// Health check
	s.router.GET("/health", s.handleHealth)

	// API v1
	v1 := s.router.Group("/api/v1")
	{
		// Session management
		v1.POST("/sessions", s.handleCreateSession)
		v1.DELETE("/sessions/:id", s.handleDeleteSession)
		v1.GET("/sessions/:id", s.handleGetSession)

		// Chat endpoints
		v1.POST("/chat", s.handleChat)
		v1.GET("/chat/stream", s.handleChatStream)

		// Index endpoint
		v1.POST("/index", s.handleIndex)

		// Search endpoint
		v1.POST("/search", s.handleSearch)
	}
}

// Run starts the server
func (s *Server) Run() error {
	addr := fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.Port)
	fmt.Printf("🚀 Starting CodeMentor API server on %s\n", addr)
	return s.router.Run(addr)
}

// corsMiddleware adds CORS headers
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// handleHealth handles health check requests
func (s *Server) handleHealth(c *gin.Context) {
	// Check Ollama connectivity
	ragAgent := agent.NewRAGAgent(s.config)
	defer ragAgent.Close()

	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()

	ollamaStatus := "ok"
	if err := ragAgent.CheckHealth(ctx); err != nil {
		ollamaStatus = fmt.Sprintf("error: %v", err)
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"version": "0.1.0",
		"ollama":  ollamaStatus,
	})
}

// CreateSessionRequest represents a session creation request
type CreateSessionRequest struct {
	RepoPath string `json:"repo_path" binding:"required"`
}

// handleCreateSession creates a new chat session
func (s *Server) handleCreateSession(c *gin.Context) {
	var req CreateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()

	// Create new agent
	ragAgent := agent.NewRAGAgent(s.config)

	// Check health
	if err := ragAgent.CheckHealth(ctx); err != nil {
		ragAgent.Close()
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Ollama not available"})
		return
	}

	// Index repository
	err := ragAgent.IndexRepository(ctx, req.RepoPath, nil)
	if err != nil {
		ragAgent.Close()
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Failed to index: %v", err)})
		return
	}

	// Create session
	sessionID := fmt.Sprintf("session_%d", time.Now().UnixNano())
	session := &Session{
		ID:        sessionID,
		Agent:     ragAgent,
		RepoPath:  req.RepoPath,
		CreatedAt: time.Now(),
		LastUsed:  time.Now(),
	}

	s.mu.Lock()
	s.sessions[sessionID] = session
	s.mu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"session_id":   sessionID,
		"repo_path":    req.RepoPath,
		"chunk_count":  ragAgent.GetChunkCount(),
		"created_at":   session.CreatedAt,
	})
}

// handleDeleteSession deletes a chat session
func (s *Server) handleDeleteSession(c *gin.Context) {
	sessionID := c.Param("id")

	s.mu.Lock()
	session, exists := s.sessions[sessionID]
	if exists {
		session.Agent.Close()
		delete(s.sessions, sessionID)
	}
	s.mu.Unlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Session deleted"})
}

// handleGetSession gets session info
func (s *Server) handleGetSession(c *gin.Context) {
	sessionID := c.Param("id")

	s.mu.RLock()
	session, exists := s.sessions[sessionID]
	s.mu.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"session_id":  session.ID,
		"repo_path":   session.RepoPath,
		"chunk_count": session.Agent.GetChunkCount(),
		"created_at":  session.CreatedAt,
		"last_used":   session.LastUsed,
	})
}

// ChatRequest represents a chat request
type ChatRequest struct {
	SessionID string `json:"session_id" binding:"required"`
	Message   string `json:"message" binding:"required"`
}

// handleChat handles non-streaming chat requests
func (s *Server) handleChat(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	s.mu.RLock()
	session, exists := s.sessions[req.SessionID]
	s.mu.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	ctx := c.Request.Context()

	response, err := session.Agent.Ask(ctx, req.Message)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Update last used
	s.mu.Lock()
	session.LastUsed = time.Now()
	s.mu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"response": response,
	})
}

// handleChatStream handles SSE streaming chat requests
func (s *Server) handleChatStream(c *gin.Context) {
	sessionID := c.Query("session_id")
	message := c.Query("message")

	if sessionID == "" || message == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session_id and message required"})
		return
	}

	s.mu.RLock()
	session, exists := s.sessions[sessionID]
	s.mu.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	ctx := c.Request.Context()

	// Stream response
	err := session.Agent.AskStream(ctx, message, func(content string, done bool) error {
		if done {
			c.SSEvent("done", "")
		} else {
			c.SSEvent("message", content)
		}
		c.Writer.Flush()
		return nil
	})

	if err != nil {
		c.SSEvent("error", err.Error())
		c.Writer.Flush()
	}

	// Update last used
	s.mu.Lock()
	session.LastUsed = time.Now()
	s.mu.Unlock()
}

// IndexRequest represents an index request
type IndexRequest struct {
	RepoPath string `json:"repo_path" binding:"required"`
}

// handleIndex handles repository indexing (one-shot, no session)
func (s *Server) handleIndex(c *gin.Context) {
	var req IndexRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()

	ragAgent := agent.NewRAGAgent(s.config)
	defer ragAgent.Close()

	if err := ragAgent.CheckHealth(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Ollama not available"})
		return
	}

	startTime := time.Now()
	err := ragAgent.IndexRepository(ctx, req.RepoPath, nil)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"repo_path":   req.RepoPath,
		"chunk_count": ragAgent.GetChunkCount(),
		"elapsed":     time.Since(startTime).String(),
	})
}

// SearchRequest represents a search request
type SearchRequest struct {
	SessionID string `json:"session_id" binding:"required"`
	Query     string `json:"query" binding:"required"`
	TopK      int    `json:"top_k"`
}

// handleSearch handles code search requests
func (s *Server) handleSearch(c *gin.Context) {
	var req SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.TopK <= 0 {
		req.TopK = 5
	}

	s.mu.RLock()
	session, exists := s.sessions[req.SessionID]
	s.mu.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	ctx := c.Request.Context()

	results, err := session.Agent.GetRetrievedChunks(ctx, req.Query, req.TopK)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Format results
	var formattedResults []gin.H
	for _, r := range results {
		formattedResults = append(formattedResults, gin.H{
			"file_path":   r.Chunk.FilePath,
			"start_line":  r.Chunk.StartLine,
			"end_line":    r.Chunk.EndLine,
			"chunk_type":  r.Chunk.ChunkType,
			"name":        r.Chunk.Name,
			"signature":   r.Chunk.Signature,
			"content":     r.Chunk.Content,
			"score":       r.Score,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"query":   req.Query,
		"results": formattedResults,
	})
}

