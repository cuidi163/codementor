package indexer

import (
	"os"
	"strings"
)

// Chunker provides generic text chunking for files that don't have AST parsers
type Chunker struct {
	chunkSize    int
	chunkOverlap int
}

// NewChunker creates a new chunker
func NewChunker(chunkSize, chunkOverlap int) *Chunker {
	return &Chunker{
		chunkSize:    chunkSize,
		chunkOverlap: chunkOverlap,
	}
}

// ChunkFile chunks a file using a sliding window approach
func (c *Chunker) ChunkFile(fileInfo *FileInfo) ([]*CodeChunk, error) {
	content, err := os.ReadFile(fileInfo.Path)
	if err != nil {
		return nil, err
	}

	text := string(content)
	language := GetLanguage(fileInfo.Extension)

	// If file is small enough, return as single chunk
	if len(text) <= c.chunkSize {
		chunk := &CodeChunk{
			FilePath:  fileInfo.RelPath,
			Language:  language,
			ChunkType: ChunkTypeFile,
			Name:      fileInfo.RelPath,
			Content:   text,
			StartLine: 1,
			EndLine:   strings.Count(text, "\n") + 1,
		}
		chunk.ID = generateChunkID(chunk)
		return []*CodeChunk{chunk}, nil
	}

	// Split into chunks with overlap
	return c.chunkText(text, fileInfo.RelPath, language)
}

// chunkText splits text into overlapping chunks
func (c *Chunker) chunkText(text, filePath, language string) ([]*CodeChunk, error) {
	lines := strings.Split(text, "\n")
	var chunks []*CodeChunk

	// Calculate approximate lines per chunk
	avgLineLen := len(text) / len(lines)
	if avgLineLen == 0 {
		avgLineLen = 50
	}
	linesPerChunk := c.chunkSize / avgLineLen
	overlapLines := c.chunkOverlap / avgLineLen

	if linesPerChunk < 10 {
		linesPerChunk = 10
	}
	if overlapLines < 2 {
		overlapLines = 2
	}

	for i := 0; i < len(lines); i += linesPerChunk - overlapLines {
		endIdx := i + linesPerChunk
		if endIdx > len(lines) {
			endIdx = len(lines)
		}

		chunkLines := lines[i:endIdx]
		content := strings.Join(chunkLines, "\n")

		// Skip empty chunks
		if strings.TrimSpace(content) == "" {
			continue
		}

		chunk := &CodeChunk{
			FilePath:  filePath,
			Language:  language,
			ChunkType: ChunkTypeGeneric,
			Name:      filePath,
			Content:   content,
			StartLine: i + 1,
			EndLine:   endIdx,
		}
		chunk.ID = generateChunkID(chunk)
		chunks = append(chunks, chunk)

		// If we've reached the end, stop
		if endIdx >= len(lines) {
			break
		}
	}

	return chunks, nil
}

// ChunkByDelimiter chunks text by specific delimiters (like function definitions)
func (c *Chunker) ChunkByDelimiter(text, filePath, language string, delimiters []string) []*CodeChunk {
	// This is a simple implementation that could be enhanced
	// with language-specific regex patterns
	var chunks []*CodeChunk
	lines := strings.Split(text, "\n")

	var currentChunk strings.Builder
	startLine := 1
	currentLine := 1

	for _, line := range lines {
		isDelimiter := false
		for _, delim := range delimiters {
			if strings.Contains(line, delim) {
				isDelimiter = true
				break
			}
		}

		if isDelimiter && currentChunk.Len() > 0 {
			// Save current chunk
			content := currentChunk.String()
			if strings.TrimSpace(content) != "" {
				chunk := &CodeChunk{
					FilePath:  filePath,
					Language:  language,
					ChunkType: ChunkTypeGeneric,
					Name:      filePath,
					Content:   content,
					StartLine: startLine,
					EndLine:   currentLine - 1,
				}
				chunk.ID = generateChunkID(chunk)
				chunks = append(chunks, chunk)
			}

			// Start new chunk
			currentChunk.Reset()
			startLine = currentLine
		}

		currentChunk.WriteString(line)
		currentChunk.WriteString("\n")
		currentLine++
	}

	// Don't forget the last chunk
	if currentChunk.Len() > 0 {
		content := currentChunk.String()
		if strings.TrimSpace(content) != "" {
			chunk := &CodeChunk{
				FilePath:  filePath,
				Language:  language,
				ChunkType: ChunkTypeGeneric,
				Name:      filePath,
				Content:   content,
				StartLine: startLine,
				EndLine:   currentLine - 1,
			}
			chunk.ID = generateChunkID(chunk)
			chunks = append(chunks, chunk)
		}
	}

	return chunks
}

