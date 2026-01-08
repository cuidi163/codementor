package config

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config holds all configuration for the application
type Config struct {
	Ollama    OllamaConfig    `mapstructure:"ollama"`
	Embedding EmbeddingConfig `mapstructure:"embedding"`
	Vector    VectorConfig    `mapstructure:"vector"`
	Indexer   IndexerConfig   `mapstructure:"indexer"`
	Server    ServerConfig    `mapstructure:"server"`
}

// EmbeddingConfig holds embedding service configuration
type EmbeddingConfig struct {
	Provider string `mapstructure:"provider"` // ollama, codebert
	Host     string `mapstructure:"host"`     // For codebert service
}

// OllamaConfig holds Ollama-related configuration
type OllamaConfig struct {
	Host           string `mapstructure:"host"`
	ChatModel      string `mapstructure:"chat_model"`
	EmbeddingModel string `mapstructure:"embedding_model"`
	Timeout        int    `mapstructure:"timeout"` // seconds
}

// VectorConfig holds vector database configuration
type VectorConfig struct {
	Type       string `mapstructure:"type"` // milvus, qdrant, memory
	Host       string `mapstructure:"host"`
	Port       int    `mapstructure:"port"`
	Collection string `mapstructure:"collection"`
	Dimension  int    `mapstructure:"dimension"`
}

// IndexerConfig holds code indexing configuration
type IndexerConfig struct {
	ChunkSize    int      `mapstructure:"chunk_size"`
	ChunkOverlap int      `mapstructure:"chunk_overlap"`
	Extensions   []string `mapstructure:"extensions"`
	IgnoreDirs   []string `mapstructure:"ignore_dirs"`
}

// ServerConfig holds HTTP server configuration
type ServerConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Ollama: OllamaConfig{
			Host:           "http://localhost:11434",
			ChatModel:      "qwen2.5:7b",
			EmbeddingModel: "nomic-embed-text",
			Timeout:        120,
		},
		Embedding: EmbeddingConfig{
			Provider: "codebert",                 // codebert or ollama
			Host:     "http://localhost:8001",    // CodeBERT service host
		},
		Vector: VectorConfig{
			Type:       "memory",
			Host:       "localhost",
			Port:       19530,
			Collection: "codementor",
			Dimension:  768,
		},
		Indexer: IndexerConfig{
			ChunkSize:    1000,
			ChunkOverlap: 200,
			Extensions:   []string{".go", ".py", ".js", ".ts", ".java", ".rs", ".cpp", ".c", ".h"},
			IgnoreDirs:   []string{".git", "node_modules", "vendor", "__pycache__", ".idea", ".vscode"},
		},
		Server: ServerConfig{
			Host: "0.0.0.0",
			Port: 8080,
		},
	}
}

// Load loads configuration from file and environment
func Load(configPath string) (*Config, error) {
	cfg := DefaultConfig()

	if configPath != "" {
		viper.SetConfigFile(configPath)
	} else {
		// Look for config in default locations
		home, err := os.UserHomeDir()
		if err == nil {
			viper.AddConfigPath(filepath.Join(home, ".codementor"))
		}
		viper.AddConfigPath(".")
		viper.AddConfigPath("./configs")
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	// Environment variable overrides
	viper.SetEnvPrefix("CODEMENTOR")
	viper.AutomaticEnv()

	// Bind environment variables
	viper.BindEnv("ollama.host", "CODEMENTOR_OLLAMA_HOST")
	viper.BindEnv("ollama.chat_model", "CODEMENTOR_OLLAMA_CHAT_MODEL")
	viper.BindEnv("ollama.embedding_model", "CODEMENTOR_OLLAMA_EMBEDDING_MODEL")
	viper.BindEnv("embedding.provider", "CODEMENTOR_EMBEDDING_PROVIDER")
	viper.BindEnv("embedding.host", "CODEMENTOR_EMBEDDING_HOST")
	viper.BindEnv("vector.type", "CODEMENTOR_VECTOR_TYPE")
	viper.BindEnv("vector.host", "CODEMENTOR_VECTOR_HOST")
	viper.BindEnv("vector.port", "CODEMENTOR_VECTOR_PORT")
	viper.BindEnv("vector.collection", "CODEMENTOR_VECTOR_COLLECTION")
	viper.BindEnv("server.host", "CODEMENTOR_SERVER_HOST")
	viper.BindEnv("server.port", "CODEMENTOR_SERVER_PORT")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
		// Config file not found, use defaults
	}

	if err := viper.Unmarshal(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

