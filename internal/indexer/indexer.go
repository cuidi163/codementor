package indexer

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/codementor/codementor/internal/config"
)

// Indexer is the main code indexer
type Indexer struct {
	config   config.IndexerConfig
	scanner  *Scanner
	goParser *GoParser
	chunker  *Chunker
}

// NewIndexer creates a new indexer
func NewIndexer(cfg config.IndexerConfig) *Indexer {
	return &Indexer{
		config:   cfg,
		scanner:  NewScanner(cfg),
		goParser: NewGoParser(),
		chunker:  NewChunker(cfg.ChunkSize, cfg.ChunkOverlap),
	}
}

// IndexRepository indexes a code repository
func (idx *Indexer) IndexRepository(repoPath string) (*IndexResult, error) {
	startTime := time.Now()

	// Resolve absolute path
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve path: %w", err)
	}

	// Check if path exists
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("path does not exist: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("path is not a directory: %s", absPath)
	}

	// Scan for source files
	files, err := idx.scanner.Scan(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to scan directory: %w", err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no source files found in %s", absPath)
	}

	// Process files concurrently
	var allChunks []*CodeChunk
	var errors []string
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Use a semaphore to limit concurrency
	sem := make(chan struct{}, 10)

	for _, file := range files {
		wg.Add(1)
		go func(f *FileInfo) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			chunks, err := idx.parseFile(f)
			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				errors = append(errors, fmt.Sprintf("%s: %v", f.RelPath, err))
				return
			}
			allChunks = append(allChunks, chunks...)
		}(file)
	}

	wg.Wait()

	// Collect language statistics
	langMap := make(map[string]bool)
	for _, chunk := range allChunks {
		langMap[chunk.Language] = true
	}
	var languages []string
	for lang := range langMap {
		languages = append(languages, lang)
	}

	// Create repository info
	repo := &Repository{
		Path:        absPath,
		Name:        filepath.Base(absPath),
		Languages:   languages,
		TotalFiles:  len(files),
		TotalChunks: len(allChunks),
		IndexedAt:   time.Now().Format(time.RFC3339),
	}

	result := &IndexResult{
		Repository:  repo,
		Chunks:      allChunks,
		Errors:      errors,
		ElapsedTime: time.Since(startTime).String(),
	}

	return result, nil
}

// parseFile parses a single file and returns chunks
func (idx *Indexer) parseFile(file *FileInfo) ([]*CodeChunk, error) {
	lang := GetLanguage(file.Extension)

	switch lang {
	case "go":
		return idx.goParser.Parse(file)
	default:
		// Use generic chunker for other languages
		return idx.chunker.ChunkFile(file)
	}
}

// IndexStats returns statistics about indexed content
type IndexStats struct {
	TotalFiles    int            `json:"total_files"`
	TotalChunks   int            `json:"total_chunks"`
	ChunksByType  map[string]int `json:"chunks_by_type"`
	ChunksByLang  map[string]int `json:"chunks_by_lang"`
	AverageChunks float64        `json:"average_chunks_per_file"`
}

// GetStats returns statistics for an index result
func GetStats(result *IndexResult) *IndexStats {
	stats := &IndexStats{
		TotalFiles:   result.Repository.TotalFiles,
		TotalChunks:  result.Repository.TotalChunks,
		ChunksByType: make(map[string]int),
		ChunksByLang: make(map[string]int),
	}

	for _, chunk := range result.Chunks {
		stats.ChunksByType[string(chunk.ChunkType)]++
		stats.ChunksByLang[chunk.Language]++
	}

	if stats.TotalFiles > 0 {
		stats.AverageChunks = float64(stats.TotalChunks) / float64(stats.TotalFiles)
	}

	return stats
}

// PrintStats prints indexing statistics
func PrintStats(result *IndexResult) {
	stats := GetStats(result)

	fmt.Printf("\n📊 Indexing Statistics\n")
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("   Repository: %s\n", result.Repository.Name)
	fmt.Printf("   Total Files: %d\n", stats.TotalFiles)
	fmt.Printf("   Total Chunks: %d\n", stats.TotalChunks)
	fmt.Printf("   Avg Chunks/File: %.1f\n", stats.AverageChunks)
	fmt.Printf("   Time Elapsed: %s\n", result.ElapsedTime)

	fmt.Printf("\n📁 Chunks by Type:\n")
	for chunkType, count := range stats.ChunksByType {
		fmt.Printf("   • %s: %d\n", chunkType, count)
	}

	fmt.Printf("\n🔤 Chunks by Language:\n")
	for lang, count := range stats.ChunksByLang {
		fmt.Printf("   • %s: %d\n", lang, count)
	}

	if len(result.Errors) > 0 {
		fmt.Printf("\n⚠️  Errors (%d):\n", len(result.Errors))
		for _, err := range result.Errors {
			fmt.Printf("   • %s\n", err)
		}
	}
}

