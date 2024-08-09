package files

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/artmoskvin/hide/pkg/model"
	"github.com/bluekeyes/go-gitdiff/gitdiff"
	"github.com/rs/zerolog/log"
	"github.com/spf13/afero"
)

const DefaultNumLines = 100
const DefaultStartLine = 1
const DefaultShowLineNumbers = false

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
	CreateFile(ctx context.Context, fs afero.Fs, path, content string) (model.File, error)
	ReadFile(ctx context.Context, fs afero.Fs, path string, props ReadProps) (model.File, error)
	UpdateFile(ctx context.Context, fs afero.Fs, path, content string) (model.File, error)
	DeleteFile(ctx context.Context, fs afero.Fs, path string) error
	ListFiles(ctx context.Context, fs afero.Fs, showHidden bool) ([]model.File, error)
	ApplyPatch(ctx context.Context, fs afero.Fs, path, patch string) (model.File, error)
	UpdateLines(ctx context.Context, fs afero.Fs, path string, lineDiff LineDiffChunk) (model.File, error)
}

type FileManagerImpl struct{}

func NewFileManager() FileManager {
	return &FileManagerImpl{}
}

func (fm *FileManagerImpl) CreateFile(ctx context.Context, fs afero.Fs, path, content string) (model.File, error) {
	log.Debug().Msgf("Creating file %s", path)

	dir := filepath.Dir(path)

	if err := fs.MkdirAll(dir, 0755); err != nil {
		log.Error().Err(err).Msgf("Failed to create directory %s", dir)
		return model.File{}, fmt.Errorf("Failed to create directory %s: %w", dir, err)
	}

	if err := writeFile(fs, path, content); err != nil {
		log.Error().Err(err).Msgf("Failed to create file %s", path)
		return model.File{}, fmt.Errorf("Failed to create file %s: %w", path, err)
	}

	return model.File{Path: path, Content: content}, nil
}

func (fm *FileManagerImpl) ReadFile(ctx context.Context, fs afero.Fs, path string, props ReadProps) (model.File, error) {
	log.Debug().Msgf("Reading file %s", path)

	content, err := readFile(fs, path)

	if err != nil {
		log.Error().Err(err).Msgf("Failed to open file %s", path)
		return model.File{}, fmt.Errorf("Failed to open file: %w", err)
	}

	lines := strings.Split(content.Content, "\n")

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

func (fm *FileManagerImpl) UpdateFile(ctx context.Context, fs afero.Fs, path, content string) (model.File, error) {
	log.Debug().Msgf("Updating file %s", path)

	exists, err := fileExists(fs, path)

	if err != nil {
		log.Error().Err(err).Msgf("Failed to check if file %s exists", path)
		return model.File{}, fmt.Errorf("Failed to check if file %s exists: %w", path, err)
	}

	if !exists {
		log.Error().Msgf("File %s does not exist", path)
		return model.File{}, fmt.Errorf("File %s does not exist", path)
	}

	if err := writeFile(fs, path, content); err != nil {
		log.Error().Err(err).Msgf("Failed to write file %s", path)
		return model.File{}, fmt.Errorf("Failed to write file %s: %w", path, err)
	}

	return readFile(fs, path)
}

func (fm *FileManagerImpl) DeleteFile(ctx context.Context, fs afero.Fs, path string) error {
	return fs.Remove(path)
}

func (fm *FileManagerImpl) ListFiles(ctx context.Context, fs afero.Fs, showHidden bool) ([]model.File, error) {
	log.Debug().Msg("Listing files")

	var files []model.File

	rootPath := "/"
	err := afero.Walk(fs, rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Error().Err(err).Msgf("Error walking file tree on path %s", path)
			return fmt.Errorf("Error walking file tree on path %s: %w", path, err)
		}

		if !showHidden && isHidden(path) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if !info.IsDir() {
			file, err := readFile(fs, path)
			if err != nil {
				log.Error().Err(err).Msgf("Error reading file %s", path)
				return fmt.Errorf("Error reading file %s: %w", path, err)
			}

			files = append(files, file)
		}

		return nil
	})

	return files, err
}

func (fm *FileManagerImpl) ApplyPatch(ctx context.Context, fs afero.Fs, path, patch string) (model.File, error) {
	log.Debug().Msgf("Applying patch to %s:\n%s", path, patch)

	file, err := readFile(fs, path)
	if err != nil {
		log.Error().Err(err).Msgf("Failed to read file %s", path)
		return model.File{}, fmt.Errorf("Failed to read file %s: %w", path, err)
	}

	files, _, err := gitdiff.Parse(strings.NewReader(patch))

	if err != nil {
		log.Error().Err(err).Msgf("Failed to parse patch: %s\n%s", patch, err)
		return model.File{}, fmt.Errorf("Failed to parse patch: %w", err)
	}

	if len(files) == 0 {
		log.Error().Msgf("No files changed in patch:\n%s", patch)
		return model.File{}, fmt.Errorf("No files changed in patch")
	}

	if len(files) > 1 {
		log.Error().Msgf("Multiple files changed in patch:\n%s", patch)
		return model.File{}, fmt.Errorf("Patch cannot contain multiple files")
	}

	var output bytes.Buffer

	if err := gitdiff.Apply(&output, strings.NewReader(file.Content), files[0]); err != nil {
		log.Error().Err(err).Msgf("Failed to apply patch to %s", path)
		return model.File{}, fmt.Errorf("Failed to apply patch to %s: %w\n%s", path, err, patch)
	}

	if err := afero.WriteFile(fs, path, output.Bytes(), 0644); err != nil {
		log.Error().Err(err).Msgf("Failed to write file %s after applying patch", path)
		return model.File{}, fmt.Errorf("Failed to write file %s after applying patch: %w", path, err)
	}

	log.Debug().Msgf("Applied patch to %s", path)

	return readFile(fs, path)
}

func (fm *FileManagerImpl) UpdateLines(ctx context.Context, fs afero.Fs, path string, lineDiff LineDiffChunk) (model.File, error) {
	log.Debug().Msgf("Updating lines in %s", path)

	lines, err := readLinesFromFile(fs, path)

	if err != nil {
		log.Error().Err(err).Msgf("Failed to read file %s", path)
		return model.File{}, fmt.Errorf("Failed to read file %s: %w", path, err)
	}

	if lineDiff.StartLine > len(lines) {
		log.Error().Msgf("Start line must be less than or equal to %d", len(lines))
		return model.File{}, fmt.Errorf("Start line must be less than or equal to %d", len(lines))
	}

	if lineDiff.EndLine > len(lines) {
		log.Error().Msgf("End line must be less than or equal to %d", len(lines))
		return model.File{}, fmt.Errorf("End line must be less than or equal to %d", len(lines))
	}

	newLines, err := readLinesFromString(lineDiff.Content)

	if err != nil {
		log.Error().Err(err).Msgf("Failed to read lines from linediff content: %s\n%s", lineDiff.Content, err)
		return model.File{}, fmt.Errorf("Failed to read lines from linediff content: %w", err)
	}

	// slicing is 0-based so we need to subtract 1 from the start line number; end line is exclusive so remains the same
	lines = replaceSlice(lines, newLines, lineDiff.StartLine-1, lineDiff.EndLine)

	if err := writeLines(fs, path, lines); err != nil {
		log.Error().Err(err).Msgf("Failed to write file %s when updating lines", path)
		return model.File{}, fmt.Errorf("Failed to write file %s: %w", path, err)
	}

	return readFile(fs, path)
}

func readFile(fs afero.Fs, path string) (model.File, error) {
	content, err := afero.ReadFile(fs, path)

	if err != nil {
		return model.File{}, err
	}

	return model.File{Path: path, Content: string(content)}, nil
}

func writeFile(fs afero.Fs, path string, content string) error {
	return afero.WriteFile(fs, path, []byte(content), 0644)
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

func fileExists(fs afero.Fs, path string) (bool, error) {
	return afero.Exists(fs, path)
}

func replaceSlice(original []string, replacement []string, start, end int) []string {
	newLength := len(original) - (end - start) + len(replacement)
	result := make([]string, newLength)

	copy(result, original[:start])
	copy(result[start:], replacement)
	copy(result[start+len(replacement):], original[end:])

	return result
}

func isHidden(path string) bool {
	name := filepath.Base(path)
	return strings.HasPrefix(name, ".") && name != "." && name != ".."
}
