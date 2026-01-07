package indexer

// ChunkType represents the type of code chunk
type ChunkType string

const (
	ChunkTypeFunction  ChunkType = "function"
	ChunkTypeMethod    ChunkType = "method"
	ChunkTypeStruct    ChunkType = "struct"
	ChunkTypeInterface ChunkType = "interface"
	ChunkTypeConstant  ChunkType = "constant"
	ChunkTypeVariable  ChunkType = "variable"
	ChunkTypeImport    ChunkType = "import"
	ChunkTypePackage   ChunkType = "package"
	ChunkTypeComment   ChunkType = "comment"
	ChunkTypeFile      ChunkType = "file"      // Entire file (for small files)
	ChunkTypeGeneric   ChunkType = "generic"   // Fallback for non-AST parsing
)

// CodeChunk represents a piece of code with metadata
type CodeChunk struct {
	ID          string            `json:"id"`
	Content     string            `json:"content"`
	FilePath    string            `json:"file_path"`
	Language    string            `json:"language"`
	ChunkType   ChunkType         `json:"chunk_type"`
	Name        string            `json:"name"`        // Function/struct/variable name
	Signature   string            `json:"signature"`   // Function signature, struct definition
	StartLine   int               `json:"start_line"`
	EndLine     int               `json:"end_line"`
	DocComment  string            `json:"doc_comment"` // Documentation comment
	ParentName  string            `json:"parent_name"` // For methods: the struct name
	Imports     []string          `json:"imports"`     // Related imports
	Metadata    map[string]string `json:"metadata"`    // Additional metadata
}

// Repository represents an indexed code repository
type Repository struct {
	Path        string   `json:"path"`
	URL         string   `json:"url,omitempty"`
	Name        string   `json:"name"`
	Languages   []string `json:"languages"`
	TotalFiles  int      `json:"total_files"`
	TotalChunks int      `json:"total_chunks"`
	IndexedAt   string   `json:"indexed_at"`
}

// IndexResult represents the result of indexing a repository
type IndexResult struct {
	Repository  *Repository  `json:"repository"`
	Chunks      []*CodeChunk `json:"chunks"`
	Errors      []string     `json:"errors,omitempty"`
	ElapsedTime string       `json:"elapsed_time"`
}

// FileInfo holds information about a file to be indexed
type FileInfo struct {
	Path      string
	RelPath   string // Relative path from repo root
	Extension string
	Size      int64
}

