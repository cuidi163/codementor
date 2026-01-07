package indexer

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/codementor/codementor/internal/config"
)

// Scanner scans a directory for source code files
type Scanner struct {
	config config.IndexerConfig
}

// NewScanner creates a new file scanner
func NewScanner(cfg config.IndexerConfig) *Scanner {
	return &Scanner{config: cfg}
}

// Scan scans a directory and returns all matching source files
func (s *Scanner) Scan(rootPath string) ([]*FileInfo, error) {
	var files []*FileInfo

	// Create extension map for quick lookup
	extMap := make(map[string]bool)
	for _, ext := range s.config.Extensions {
		extMap[ext] = true
	}

	// Create ignore dirs map
	ignoreMap := make(map[string]bool)
	for _, dir := range s.config.IgnoreDirs {
		ignoreMap[dir] = true
	}

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip ignored directories
		if info.IsDir() {
			if ignoreMap[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		// Check file extension
		ext := strings.ToLower(filepath.Ext(path))
		if !extMap[ext] {
			return nil
		}

		// Calculate relative path
		relPath, err := filepath.Rel(rootPath, path)
		if err != nil {
			relPath = path
		}

		files = append(files, &FileInfo{
			Path:      path,
			RelPath:   relPath,
			Extension: ext,
			Size:      info.Size(),
		})

		return nil
	})

	return files, err
}

// GetLanguage returns the programming language based on file extension
func GetLanguage(ext string) string {
	switch strings.ToLower(ext) {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".js":
		return "javascript"
	case ".ts":
		return "typescript"
	case ".tsx":
		return "typescript"
	case ".jsx":
		return "javascript"
	case ".java":
		return "java"
	case ".rs":
		return "rust"
	case ".cpp", ".cc", ".cxx":
		return "cpp"
	case ".c":
		return "c"
	case ".h", ".hpp":
		return "c_header"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	case ".swift":
		return "swift"
	case ".kt":
		return "kotlin"
	case ".scala":
		return "scala"
	case ".md":
		return "markdown"
	case ".yaml", ".yml":
		return "yaml"
	case ".json":
		return "json"
	case ".sql":
		return "sql"
	case ".sh", ".bash":
		return "shell"
	default:
		return "unknown"
	}
}

