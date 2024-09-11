package model

import (
	"bufio"
	"fmt"
	"strings"

	protocol "github.com/tliron/glsp/protocol_3_16"
)

type Line struct {
	Number  int    `json:"number"`
	Content string `json:"content"`
}

type File struct {
	Path  string `json:"path"`
	Lines []Line `json:"lines"`
	// NOTE: should diagnostics be part of a line?
	Diagnostics []protocol.Diagnostic `json:"diagnostics,omitempty"`
}

func (f *File) Equals(other *File) bool {
	if f == nil && other == nil {
		return true
	}

	if f == nil || other == nil {
		return false
	}

	if f.Path != other.Path || len(f.Lines) != len(other.Lines) {
		return false
	}

	for i := range f.Lines {
		if f.Lines[i] != other.Lines[i] {
			return false
		}
	}

	// NOTE: I'm not sure that diagnostics should be part of comparison, so I'm leaving it out for now

	return true
}

func (f *File) GetContent() string {
	var content strings.Builder

	for _, line := range f.Lines {
		content.WriteString(line.Content)
		content.WriteString("\n")
	}

	return content.String()
}

// GetLine returns the line with the given line number. Line numbers are 1-based.
func (f *File) GetLine(lineNumber int) Line {
	if lineNumber < 1 || lineNumber > len(f.Lines) {
		return Line{}
	}

	return f.Lines[lineNumber-1]
}

// GetLineRange returns the lines between start and end (exclusive). Line numbers are 1-based.
func (f *File) GetLineRange(start, end int) []Line {
	// Convert to 0-based indexing
	start -= 1
	end -= 1

	if start < 0 {
		start = 0
	}

	if end > len(f.Lines) {
		end = len(f.Lines)
	}

	return f.Lines[start:end]
}

// WithLineRange returns a new File with the lines between start and end (exclusive). Line numbers are 1-based.
func (f *File) WithLineRange(start, end int) *File {
	return &File{Path: f.Path, Lines: f.GetLineRange(start, end)}
}

func (f *File) WithPath(path string) *File {
	return &File{Path: path, Lines: f.Lines}
}

// ReplaceLineRange replaces the lines between start and end (exclusive) with the given content. Line numbers are 1-based.
func (f *File) ReplaceLineRange(start, end int, content string) (*File, error) {
	if start == end {
		return f, nil
	}

	replacement, err := NewLines(content)
	if err != nil {
		return f, err
	}

	newLength := len(f.Lines) - (end - start) + len(replacement)
	result := make([]Line, newLength)

	// Convert to 0-based indexing
	start -= 1
	end -= 1

	copy(result, f.Lines[:start])
	copy(result[start:], replacement)
	if end < len(f.Lines) {
		copy(result[start+len(replacement):], f.Lines[end:])
	}

	for i := start; i < len(result); i++ {
		result[i].Number = i + 1
	}

	return &File{Path: f.Path, Lines: result}, nil
}

// NewFile creates a new File from the given path and content. Content is split into lines. Line numbers are 1-based.
func NewFile(path string, content string) (*File, error) {
	lines, err := NewLines(content)

	if err != nil {
		return nil, fmt.Errorf("Failed to create lines from content: %w", err)
	}

	return &File{Path: path, Lines: lines}, nil
}

// NewLines splits the given content into lines. Line numbers are 1-based.
func NewLines(content string) ([]Line, error) {
	var lines []Line

	scanner := bufio.NewScanner(strings.NewReader(content))
	lineNumber := 1

	for scanner.Scan() {
		lines = append(lines, Line{Number: lineNumber, Content: scanner.Text()})
		lineNumber++
	}

	return lines, scanner.Err()
}

func EmptyFile(path string) *File {
	return &File{Path: path, Lines: []Line{}}
}
