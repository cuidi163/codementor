package indexer

import (
	"crypto/md5"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
)

// GoParser parses Go source files using AST
type GoParser struct{}

// NewGoParser creates a new Go parser
func NewGoParser() *GoParser {
	return &GoParser{}
}

// Parse parses a Go file and extracts code chunks
func (p *GoParser) Parse(fileInfo *FileInfo) ([]*CodeChunk, error) {
	content, err := os.ReadFile(fileInfo.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, fileInfo.Path, content, parser.ParseComments)
	if err != nil {
		// If AST parsing fails, fall back to generic chunking
		return p.fallbackParse(fileInfo, content)
	}

	var chunks []*CodeChunk
	lines := strings.Split(string(content), "\n")

	// Extract package info
	if file.Name != nil {
		pkgChunk := &CodeChunk{
			FilePath:  fileInfo.RelPath,
			Language:  "go",
			ChunkType: ChunkTypePackage,
			Name:      file.Name.Name,
			Content:   fmt.Sprintf("package %s", file.Name.Name),
			StartLine: fset.Position(file.Package).Line,
			EndLine:   fset.Position(file.Package).Line,
		}
		pkgChunk.ID = generateChunkID(pkgChunk)
		chunks = append(chunks, pkgChunk)
	}

	// Extract imports
	imports := p.extractImports(file, fset, lines, fileInfo.RelPath)
	chunks = append(chunks, imports...)

	// Extract functions and methods
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			chunk := p.parseFuncDecl(d, fset, lines, fileInfo.RelPath, file)
			if chunk != nil {
				chunks = append(chunks, chunk)
			}

		case *ast.GenDecl:
			genChunks := p.parseGenDecl(d, fset, lines, fileInfo.RelPath)
			chunks = append(chunks, genChunks...)
		}
	}

	return chunks, nil
}

// extractImports extracts import declarations
func (p *GoParser) extractImports(file *ast.File, fset *token.FileSet, lines []string, filePath string) []*CodeChunk {
	var chunks []*CodeChunk
	var importNames []string

	for _, imp := range file.Imports {
		importPath := strings.Trim(imp.Path.Value, "\"")
		importNames = append(importNames, importPath)
	}

	if len(importNames) > 0 && len(file.Imports) > 0 {
		startLine := fset.Position(file.Imports[0].Pos()).Line
		endLine := fset.Position(file.Imports[len(file.Imports)-1].End()).Line

		content := extractLines(lines, startLine, endLine)
		chunk := &CodeChunk{
			FilePath:  filePath,
			Language:  "go",
			ChunkType: ChunkTypeImport,
			Name:      "imports",
			Content:   content,
			StartLine: startLine,
			EndLine:   endLine,
			Imports:   importNames,
		}
		chunk.ID = generateChunkID(chunk)
		chunks = append(chunks, chunk)
	}

	return chunks
}

// parseFuncDecl parses a function declaration
func (p *GoParser) parseFuncDecl(fn *ast.FuncDecl, fset *token.FileSet, lines []string, filePath string, file *ast.File) *CodeChunk {
	startLine := fset.Position(fn.Pos()).Line
	endLine := fset.Position(fn.End()).Line

	// Extract doc comment
	var docComment string
	if fn.Doc != nil {
		docComment = fn.Doc.Text()
	}

	// Build signature
	signature := buildFuncSignature(fn)

	// Determine if it's a method or function
	chunkType := ChunkTypeFunction
	var parentName string
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		chunkType = ChunkTypeMethod
		parentName = getReceiverTypeName(fn.Recv.List[0].Type)
	}

	content := extractLines(lines, startLine, endLine)

	chunk := &CodeChunk{
		FilePath:   filePath,
		Language:   "go",
		ChunkType:  chunkType,
		Name:       fn.Name.Name,
		Signature:  signature,
		Content:    content,
		StartLine:  startLine,
		EndLine:    endLine,
		DocComment: strings.TrimSpace(docComment),
		ParentName: parentName,
	}
	chunk.ID = generateChunkID(chunk)

	return chunk
}

// parseGenDecl parses a general declaration (type, const, var)
func (p *GoParser) parseGenDecl(decl *ast.GenDecl, fset *token.FileSet, lines []string, filePath string) []*CodeChunk {
	var chunks []*CodeChunk

	for _, spec := range decl.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			chunk := p.parseTypeSpec(s, decl, fset, lines, filePath)
			if chunk != nil {
				chunks = append(chunks, chunk)
			}

		case *ast.ValueSpec:
			chunk := p.parseValueSpec(s, decl, fset, lines, filePath)
			if chunk != nil {
				chunks = append(chunks, chunk)
			}
		}
	}

	return chunks
}

// parseTypeSpec parses a type specification (struct, interface)
func (p *GoParser) parseTypeSpec(spec *ast.TypeSpec, decl *ast.GenDecl, fset *token.FileSet, lines []string, filePath string) *CodeChunk {
	startLine := fset.Position(decl.Pos()).Line
	endLine := fset.Position(decl.End()).Line

	var chunkType ChunkType
	var signature string

	switch t := spec.Type.(type) {
	case *ast.StructType:
		chunkType = ChunkTypeStruct
		signature = fmt.Sprintf("type %s struct", spec.Name.Name)
		// Include field count in metadata
		if t.Fields != nil {
			signature = fmt.Sprintf("type %s struct { %d fields }", spec.Name.Name, len(t.Fields.List))
		}

	case *ast.InterfaceType:
		chunkType = ChunkTypeInterface
		signature = fmt.Sprintf("type %s interface", spec.Name.Name)
		if t.Methods != nil {
			signature = fmt.Sprintf("type %s interface { %d methods }", spec.Name.Name, len(t.Methods.List))
		}

	default:
		// Type alias or other type definitions
		chunkType = ChunkTypeGeneric
		signature = fmt.Sprintf("type %s", spec.Name.Name)
	}

	// Extract doc comment
	var docComment string
	if decl.Doc != nil {
		docComment = decl.Doc.Text()
	} else if spec.Doc != nil {
		docComment = spec.Doc.Text()
	}

	content := extractLines(lines, startLine, endLine)

	chunk := &CodeChunk{
		FilePath:   filePath,
		Language:   "go",
		ChunkType:  chunkType,
		Name:       spec.Name.Name,
		Signature:  signature,
		Content:    content,
		StartLine:  startLine,
		EndLine:    endLine,
		DocComment: strings.TrimSpace(docComment),
	}
	chunk.ID = generateChunkID(chunk)

	return chunk
}

// parseValueSpec parses a value specification (const, var)
func (p *GoParser) parseValueSpec(spec *ast.ValueSpec, decl *ast.GenDecl, fset *token.FileSet, lines []string, filePath string) *CodeChunk {
	if len(spec.Names) == 0 {
		return nil
	}

	startLine := fset.Position(decl.Pos()).Line
	endLine := fset.Position(decl.End()).Line

	var chunkType ChunkType
	var name string

	switch decl.Tok {
	case token.CONST:
		chunkType = ChunkTypeConstant
	case token.VAR:
		chunkType = ChunkTypeVariable
	default:
		chunkType = ChunkTypeGeneric
	}

	// Collect all names in this spec
	names := make([]string, len(spec.Names))
	for i, n := range spec.Names {
		names[i] = n.Name
	}
	name = strings.Join(names, ", ")

	// Extract doc comment
	var docComment string
	if decl.Doc != nil {
		docComment = decl.Doc.Text()
	} else if spec.Doc != nil {
		docComment = spec.Doc.Text()
	}

	content := extractLines(lines, startLine, endLine)

	chunk := &CodeChunk{
		FilePath:   filePath,
		Language:   "go",
		ChunkType:  chunkType,
		Name:       name,
		Content:    content,
		StartLine:  startLine,
		EndLine:    endLine,
		DocComment: strings.TrimSpace(docComment),
	}
	chunk.ID = generateChunkID(chunk)

	return chunk
}

// fallbackParse provides fallback parsing for files that can't be parsed with AST
func (p *GoParser) fallbackParse(fileInfo *FileInfo, content []byte) ([]*CodeChunk, error) {
	chunk := &CodeChunk{
		FilePath:  fileInfo.RelPath,
		Language:  "go",
		ChunkType: ChunkTypeFile,
		Name:      fileInfo.RelPath,
		Content:   string(content),
		StartLine: 1,
		EndLine:   strings.Count(string(content), "\n") + 1,
	}
	chunk.ID = generateChunkID(chunk)

	return []*CodeChunk{chunk}, nil
}

// buildFuncSignature builds a human-readable function signature
func buildFuncSignature(fn *ast.FuncDecl) string {
	var sb strings.Builder

	sb.WriteString("func ")

	// Add receiver if present
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		recv := fn.Recv.List[0]
		recvType := getReceiverTypeName(recv.Type)
		if len(recv.Names) > 0 {
			sb.WriteString(fmt.Sprintf("(%s %s) ", recv.Names[0].Name, recvType))
		} else {
			sb.WriteString(fmt.Sprintf("(%s) ", recvType))
		}
	}

	sb.WriteString(fn.Name.Name)
	sb.WriteString("(")

	// Add parameters
	if fn.Type.Params != nil {
		params := formatFieldList(fn.Type.Params)
		sb.WriteString(params)
	}
	sb.WriteString(")")

	// Add return types
	if fn.Type.Results != nil && len(fn.Type.Results.List) > 0 {
		results := formatFieldList(fn.Type.Results)
		if len(fn.Type.Results.List) == 1 && len(fn.Type.Results.List[0].Names) == 0 {
			sb.WriteString(" ")
			sb.WriteString(results)
		} else {
			sb.WriteString(" (")
			sb.WriteString(results)
			sb.WriteString(")")
		}
	}

	return sb.String()
}

// getReceiverTypeName extracts the type name from a receiver expression
func getReceiverTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + getReceiverTypeName(t.X)
	default:
		return "unknown"
	}
}

// formatFieldList formats a field list (params or results)
func formatFieldList(fl *ast.FieldList) string {
	var parts []string

	for _, field := range fl.List {
		typeName := exprToString(field.Type)

		if len(field.Names) == 0 {
			parts = append(parts, typeName)
		} else {
			for _, name := range field.Names {
				parts = append(parts, fmt.Sprintf("%s %s", name.Name, typeName))
			}
		}
	}

	return strings.Join(parts, ", ")
}

// exprToString converts an expression to its string representation
func exprToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + exprToString(t.X)
	case *ast.SelectorExpr:
		return exprToString(t.X) + "." + t.Sel.Name
	case *ast.ArrayType:
		if t.Len == nil {
			return "[]" + exprToString(t.Elt)
		}
		return "[...]" + exprToString(t.Elt)
	case *ast.MapType:
		return fmt.Sprintf("map[%s]%s", exprToString(t.Key), exprToString(t.Value))
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.FuncType:
		return "func(...)"
	case *ast.ChanType:
		return "chan " + exprToString(t.Value)
	case *ast.Ellipsis:
		return "..." + exprToString(t.Elt)
	default:
		return "unknown"
	}
}

// extractLines extracts lines from start to end (1-indexed)
func extractLines(lines []string, start, end int) string {
	if start < 1 {
		start = 1
	}
	if end > len(lines) {
		end = len(lines)
	}
	if start > end || start > len(lines) {
		return ""
	}

	return strings.Join(lines[start-1:end], "\n")
}

// generateChunkID generates a unique ID for a chunk
func generateChunkID(chunk *CodeChunk) string {
	data := fmt.Sprintf("%s:%s:%s:%d", chunk.FilePath, chunk.ChunkType, chunk.Name, chunk.StartLine)
	hash := md5.Sum([]byte(data))
	return fmt.Sprintf("%x", hash[:8])
}

