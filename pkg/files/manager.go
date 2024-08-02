package files

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/artmoskvin/hide/pkg/languageserver"
	"github.com/artmoskvin/hide/pkg/model"
	"github.com/bluekeyes/go-gitdiff/gitdiff"
	"github.com/spf13/afero"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

const DefaultNumLines = 100
const DefaultStartLine = 1
const DefaultShowLineNumbers = false
const MaxDiagnosticsDelay = time.Second * 1

type ReadProps struct {
	ShowLineNumbers bool
	StartLine       int
	NumLines        int
}

type ReadPropsSetter func(*ReadProps)

func NewReadProps(setters ...ReadPropsSetter) ReadProps {
	props := ReadProps{ShowLineNumbers: DefaultShowLineNumbers, StartLine: DefaultStartLine, NumLines: DefaultNumLines}

	for _, setter := range setters {
		setter(&props)
	}

	return props
}

type FileManager interface {
	CreateFile(path string, content string) (model.File, error)
	ReadFile(fileSystem fs.FS, path string, props ReadProps) (model.File, error)
	UpdateFile(fileSystem afero.Fs, path string, content string) (model.File, error)
	DeleteFile(path string) error
	ListFiles(rootPath string) ([]model.File, error)
	ApplyPatch(fileSystem afero.Fs, path string, patch string) (model.File, error)
	UpdateLines(filesystem afero.Fs, path string, lineDiff LineDiffChunk) (model.File, error)
}

type FileManagerImpl struct{}

func NewFileManager() FileManager {
	return &FileManagerImpl{}
}

func (fm *FileManagerImpl) CreateFile(path string, content string) (model.File, error) {
	log.Println("Creating file", path)

	dir := filepath.Dir(path)

	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("Failed to create directory %s: %s", dir, err)
		return model.File{}, fmt.Errorf("Failed to create directory %s: %w", dir, err)
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		log.Printf("Failed to create file %s: %s", path, err)
		return model.File{}, fmt.Errorf("Failed to create file %s: %w", path, err)
	}

	return model.File{Path: path, Content: content}, nil
}

func (fm *FileManagerImpl) ReadFile(fileSystem fs.FS, path string, props ReadProps) (model.File, error) {
	log.Println("Reading file", path)
	content, err := fs.ReadFile(fileSystem, path)

	if err != nil {
		log.Printf("Failed to open file %s: %s", path, err)
		return model.File{}, fmt.Errorf("Failed to open file: %w", err)
	}

	lines := strings.Split(string(content), "\n")

	if props.StartLine < 1 {
		return model.File{}, fmt.Errorf("Start line must be greater than or equal to 1")
	}

	if props.StartLine > len(lines) {
		return model.File{}, fmt.Errorf("Start line must be less than or equal to %d", len(lines))
	}

	if props.NumLines < 0 {
		return model.File{}, fmt.Errorf("Number of lines must be greater than or equal to 0")
	}

	endLine := props.StartLine + props.NumLines

	// Convert to 0-based index for slice operations; limit endLine index; endLine is exclusive
	selectedLines := lines[props.StartLine-1 : min(endLine-1, len(lines))]

	// Calculate the width needed for line numbers
	lineNumberWidth := len(fmt.Sprintf("%d", endLine))

	var result strings.Builder

	for i, line := range selectedLines {
		if props.ShowLineNumbers {
			lineNumber := props.StartLine + i
			result.WriteString(fmt.Sprintf("%*d:", lineNumberWidth, lineNumber))
		}

		result.WriteString(line)
		result.WriteString("\n")
	}

	return model.File{Path: path, Content: result.String()}, nil
}

func (fm *FileManagerImpl) UpdateFile(fileSystem afero.Fs, path string, content string) (model.File, error) {
	log.Println("Updating file", path)

	exists, err := fileExists(fileSystem, path)

	if err != nil {
		log.Printf("Failed to check if file %s exists: %s", path, err)
		return model.File{}, fmt.Errorf("Failed to check if file %s exists: %w", path, err)
	}

	if !exists {
		log.Printf("model.File %s does not exist", path)
		return model.File{}, fmt.Errorf("File %s does not exist", path)
	}

	if err := writeFile(fileSystem, path, content); err != nil {
		log.Printf("Failed to write file %s: %s", path, err)
		return model.File{}, fmt.Errorf("Failed to write file %s: %w", path, err)
	}

	return readFile(fileSystem, path)
}

func (fm *FileManagerImpl) DeleteFile(path string) error {
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("Failed to delete file: %w", err)
	}

	return nil
}

func (fm *FileManagerImpl) ListFiles(rootPath string) ([]model.File, error) {
	log.Println("Listing files in", rootPath)

	var files []model.File

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("Error walking directory %s on path %s: %s", rootPath, path, err)
			return fmt.Errorf("Error walking directory %s on path %s: %w", rootPath, path, err)
		}

		relativePath, err := filepath.Rel(rootPath, path)
		if err != nil {
			log.Printf("Error getting relative path from %s to %s: %s", rootPath, path, err)
			return fmt.Errorf("Error getting relative path from %s to %s: %w", rootPath, path, err)
		}

		if !info.IsDir() {
			file, err := fm.ReadFile(os.DirFS(rootPath), relativePath, NewReadProps())
			if err != nil {
				log.Printf("Error reading file %s: %s", path, err)
				return fmt.Errorf("Error reading file %s: %w", path, err)
			}

			files = append(files, file)
		}

		return nil
	})

	return files, err
}

func (fm *FileManagerImpl) ApplyPatch(fileSystem afero.Fs, path string, patch string) (model.File, error) {
	log.Printf("Applying patch to %s:\n%s", path, patch)

	file, err := readFile(fileSystem, path)
	if err != nil {
		log.Printf("Failed to read file %s: %s", path, err)
		return model.File{}, fmt.Errorf("Failed to read file %s: %w", path, err)
	}

	files, _, err := gitdiff.Parse(strings.NewReader(patch))

	if err != nil {
		log.Printf("Failed to parse patch: %s\n%s", err, patch)
		return model.File{}, fmt.Errorf("Failed to parse patch: %w", err)
	}

	if len(files) == 0 {
		log.Printf("No files changed in patch:\n%s", patch)
		return model.File{}, fmt.Errorf("No files changed in patch")
	}

	if len(files) > 1 {
		log.Printf("Multiple files changed in patch:\n%s", patch)
		return model.File{}, fmt.Errorf("Patch cannot contain multiple files")
	}

	var output bytes.Buffer

	if err := gitdiff.Apply(&output, strings.NewReader(file.Content), files[0]); err != nil {
		log.Printf("Failed to apply patch: %s", err)
		return model.File{}, fmt.Errorf("Failed to apply patch to %s: %w\n%s", path, err, patch)
	}

	if err := afero.WriteFile(fileSystem, path, output.Bytes(), 0644); err != nil {
		log.Printf("Failed to write file %s after applying patch: %s", path, err)
		return model.File{}, fmt.Errorf("Failed to write file %s after applying patch: %w", path, err)
	}

	log.Printf("Applied patch to %s", path)

	return readFile(fileSystem, path)
}

func (fm *FileManagerImpl) UpdateLines(filesystem afero.Fs, path string, lineDiff LineDiffChunk) (model.File, error) {
	log.Printf("Updating lines in %s", path)

	lines, err := readLinesFromFile(filesystem, path)

	if err != nil {
		log.Printf("Failed to read file %s: %s", path, err)
		return model.File{}, fmt.Errorf("Failed to read file %s: %w", path, err)
	}

	if lineDiff.StartLine > len(lines) {
		log.Printf("Start line must be less than or equal to %d", len(lines))
		return model.File{}, fmt.Errorf("Start line must be less than or equal to %d", len(lines))
	}

	if lineDiff.EndLine > len(lines) {
		log.Printf("End line must be less than or equal to %d", len(lines))
		return model.File{}, fmt.Errorf("End line must be less than or equal to %d", len(lines))
	}

	newLines, err := readLinesFromString(lineDiff.Content)

	if err != nil {
		log.Printf("Failed to read lines from linediff content: %s\n%s", err, lineDiff.Content)
		return model.File{}, fmt.Errorf("Failed to read lines from linediff content: %w", err)
	}

	// slicing is 0-based so we need to subtract 1 from the start line number; end line is exclusive so remains the same
	lines = replaceSlice(lines, newLines, lineDiff.StartLine-1, lineDiff.EndLine)

	if err := writeLines(filesystem, path, lines); err != nil {
		log.Printf("Failed to write file %s when updating lines: %s", path, err)
		return model.File{}, fmt.Errorf("Failed to write file %s: %w", path, err)
	}

	return readFile(filesystem, path)
}

func readFile(fileSystem afero.Fs, path string) (model.File, error) {
	content, err := afero.ReadFile(fileSystem, path)

	if err != nil {
		return model.File{}, err
	}

	return model.File{Path: path, Content: string(content)}, nil
}

func writeFile(fileSystem afero.Fs, path string, content string) error {
	return afero.WriteFile(fileSystem, path, []byte(content), 0644)
}

func readLinesFromFile(fs afero.Fs, filename string) ([]string, error) {
	file, err := fs.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return readLines(file)
}

func readLinesFromString(content string) ([]string, error) {
	return readLines(strings.NewReader(content))
}

func readLines(r io.Reader) ([]string, error) {
	var lines []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func writeLines(fs afero.Fs, filename string, lines []string) error {
	file, err := fs.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	for _, line := range lines {
		_, err := writer.WriteString(line + "\n")
		if err != nil {
			return err
		}
	}
	return writer.Flush()
}

func fileExists(fileSystem afero.Fs, path string) (bool, error) {
	return afero.Exists(fileSystem, path)
}

func replaceSlice(original []string, replacement []string, start, end int) []string {
	newLength := len(original) - (end - start) + len(replacement)
	result := make([]string, newLength)

	copy(result, original[:start])
	copy(result[start:], replacement)
	copy(result[start+len(replacement):], original[end:])

	return result
}

type LanguageServerAwareFileManager struct {
	delegate             FileManager
	languageServerClient languageserver.Client
	languageDetector     languageserver.LanguageDetector
	diagnostics          map[string][]protocol.Diagnostic
}

func NewLanguageServerAwareFileManager(delegate FileManager, languageServerClient languageserver.Client, diagnosticsChannel chan protocol.PublishDiagnosticsParams, languageDetector languageserver.LanguageDetector) LanguageServerAwareFileManager {
	fm := LanguageServerAwareFileManager{
		delegate:             delegate,
		languageServerClient: languageServerClient,
		languageDetector:     languageDetector,
		diagnostics:          make(map[string][]protocol.Diagnostic)}

	go fm.listenForDiagnostics(diagnosticsChannel)
	return fm
}

func (fsm *LanguageServerAwareFileManager) CreateFile(path string, content string) (model.File, error) {
	// TODO: do we need to send workspace/willCreateFiles and workspace/didCreateFiles?
	file, err := fsm.delegate.CreateFile(path, content)
	if err != nil {
		return file, err
	}

	// TODO: fix me
	uri := pathToURI(path)

	err = fsm.languageServerClient.NotifyDidOpen(
		context.Background(),
		protocol.DidOpenTextDocumentParams{
			TextDocument: protocol.TextDocumentItem{
				URI:        uri,
				LanguageID: fsm.languageDetector.DetectLanguage(file),
				Version:    1,
				Text:       content}})

	if err != nil {
		return file, err
	}

	// wait for diagnostics
	time.Sleep(MaxDiagnosticsDelay)

	if diagnostics, ok := fsm.diagnostics[uri]; ok {
		file.Diagnostics = diagnostics
		return file, nil
	}

	return file, nil
}

func (fsm *LanguageServerAwareFileManager) listenForDiagnostics(channel chan protocol.PublishDiagnosticsParams) {
	for {
		select {
		case diagnostics := <-channel:
			log.Printf("Received diagnostics for %s", diagnostics.URI)

			if documentDiagnostics, ok := fsm.diagnostics[diagnostics.URI]; ok {
				documentDiagnostics = append(documentDiagnostics, diagnostics.Diagnostics...)
				fsm.diagnostics[diagnostics.URI] = documentDiagnostics
			} else {
				fsm.diagnostics[diagnostics.URI] = diagnostics.Diagnostics
			}
		}
	}
}

func pathToURI(path string) protocol.DocumentUri {
	return protocol.DocumentUri("file://" + path)
}
