package files_test

import (
	"context"
	"strings"
	"testing"

	"github.com/artmoskvin/hide/pkg/files"
	"github.com/artmoskvin/hide/pkg/model"
	"github.com/spf13/afero"
)

func TestNewReadProps(t *testing.T) {
	tests := []struct {
		name     string
		props    []files.ReadPropsSetter
		expected files.ReadProps
	}{
		{
			name: "ShowLineNumbers",
			props: []files.ReadPropsSetter{
				func(props *files.ReadProps) {
					props.ShowLineNumbers = true
				},
			},
			expected: files.ReadProps{
				ShowLineNumbers: true,
				StartLine:       files.DefaultStartLine,
				NumLines:        files.DefaultNumLines,
			},
		},
		{
			name: "StartLine",
			props: []files.ReadPropsSetter{
				func(props *files.ReadProps) {
					props.StartLine = 10
				},
			},
			expected: files.ReadProps{
				ShowLineNumbers: files.DefaultShowLineNumbers,
				StartLine:       10,
				NumLines:        files.DefaultNumLines,
			},
		},
		{
			name: "NumLines",
			props: []files.ReadPropsSetter{
				func(props *files.ReadProps) {
					props.NumLines = 20
				},
			},
			expected: files.ReadProps{
				ShowLineNumbers: files.DefaultShowLineNumbers,
				StartLine:       files.DefaultStartLine,
				NumLines:        20,
			},
		},
		{
			name: "All",
			props: []files.ReadPropsSetter{
				func(props *files.ReadProps) {
					props.ShowLineNumbers = true
					props.StartLine = 10
					props.NumLines = 20
				},
			},
			expected: files.ReadProps{
				ShowLineNumbers: true,
				StartLine:       10,
				NumLines:        20,
			},
		},
		{
			name:  "Default",
			props: []files.ReadPropsSetter{},
			expected: files.ReadProps{
				ShowLineNumbers: files.DefaultShowLineNumbers,
				StartLine:       files.DefaultStartLine,
				NumLines:        files.DefaultNumLines,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := files.NewReadProps(tt.props...)
			if actual != tt.expected {
				t.Errorf("Expected %+v, got %+v", tt.expected, actual)
			}
		})
	}
}

func TestFileManagerImpl_ReadFile_Success(t *testing.T) {
	filesystem := afero.NewMemMapFs()
	afero.WriteFile(filesystem, "test.txt", []byte("line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10"), 0644)

	tests := []struct {
		name     string
		fs       afero.Fs
		filePath string
		props    files.ReadPropsSetter
		expected model.File
	}{
		{
			name:     "ShowLineNumbers = true",
			fs:       filesystem,
			filePath: "test.txt",
			props: func(props *files.ReadProps) {
				props.ShowLineNumbers = true
				props.StartLine = 2
				props.NumLines = 3
			},
			expected: model.File{
				Path:    "test.txt",
				Content: "2:line2\n3:line3\n4:line4\n",
			},
		},
		{
			name:     "ShowLineNumbers = false",
			fs:       filesystem,
			filePath: "test.txt",
			props: func(props *files.ReadProps) {
				props.ShowLineNumbers = false
				props.StartLine = 4
				props.NumLines = 4
			},
			expected: model.File{
				Path:    "test.txt",
				Content: "line4\nline5\nline6\nline7\n",
			},
		},
		{
			name:     "NumLines = 0",
			fs:       filesystem,
			filePath: "test.txt",
			props: func(props *files.ReadProps) {
				props.ShowLineNumbers = true
				props.StartLine = 2
				props.NumLines = 0
			},
			expected: model.File{
				Path:    "test.txt",
				Content: "",
			},
		},
		{
			name:     "If StartLine + NumLines > number of lines then show all lines",
			fs:       filesystem,
			filePath: "test.txt",
			props: func(props *files.ReadProps) {
				props.ShowLineNumbers = true
				props.StartLine = 5
				props.NumLines = 10
			},
			expected: model.File{
				Path:    "test.txt",
				Content: " 5:line5\n 6:line6\n 7:line7\n 8:line8\n 9:line9\n10:line10\n",
			},
		},
		{
			name:     "Line numbers are padded with spaces",
			fs:       filesystem,
			filePath: "test.txt",
			props: func(props *files.ReadProps) {
				props.ShowLineNumbers = true
				props.StartLine = 5
				props.NumLines = 10
			},
			expected: model.File{
				Path:    "test.txt",
				Content: " 5:line5\n 6:line6\n 7:line7\n 8:line8\n 9:line9\n10:line10\n",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm := files.NewFileManager()
			actual, err := fm.ReadFile(context.Background(), tt.fs, tt.filePath, files.NewReadProps(tt.props))
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if !actual.Equals(&tt.expected) {
				t.Errorf("Expected %+v, got %+v", tt.expected, actual)
			}
		})
	}
}

func TestFileManagerImpl_ReadFile_Failure(t *testing.T) {
	fs := afero.NewMemMapFs()
	afero.WriteFile(fs, "test.txt", []byte("line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10"), 0644)

	tests := []struct {
		name     string
		fs       afero.Fs
		filePath string
		props    files.ReadPropsSetter
		expected string
	}{
		{
			name:     "StartLine < 1",
			fs:       fs,
			filePath: "test.txt",
			props: func(props *files.ReadProps) {
				props.StartLine = 0
			},
			expected: "Start line must be greater than or equal to 1",
		},
		{
			name:     "StartLine > number of lines",
			fs:       fs,
			filePath: "test.txt",
			props: func(props *files.ReadProps) {
				props.StartLine = 11
			},
			expected: "Start line must be less than or equal to 10",
		},
		{
			name:     "NumLines < 0",
			fs:       fs,
			filePath: "test.txt",
			props: func(props *files.ReadProps) {
				props.NumLines = -1
			},
			expected: "Number of lines must be greater than or equal to 0",
		},
		{
			name:     "Failed to read file",
			fs:       afero.NewMemMapFs(),
			filePath: "test.txt",
			props:    func(props *files.ReadProps) {},
			expected: "Failed to open file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm := files.NewFileManager()
			_, err := fm.ReadFile(context.Background(), tt.fs, tt.filePath, files.NewReadProps(tt.props))
			if err == nil {
				t.Fatalf("Expected error, got nil")
			}

			if !strings.Contains(err.Error(), tt.expected) {
				t.Errorf("Expected error to contain '%s', got %s", tt.expected, err.Error())
			}
		})
	}
}

func TestFileManagerImpl_ApplyPatch_Success(t *testing.T) {
	tests := []struct {
		name     string
		patch    string
		expected model.File
	}{
		{
			name: "Apply patch to file",
			patch: `--- test.txt
+++ test.txt
@@ -1,10 +1,9 @@
 line1
-line2
+line20
 line3
-line4
+line40
 line5
-line6
 line7
 line8
-line9
 line10
+line11`,
			expected: model.File{
				Path:    "test.txt",
				Content: "line1\nline20\nline3\nline40\nline5\nline7\nline8\nline10\nline11",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filesystem := afero.NewMemMapFs()
			afero.WriteFile(filesystem, "test.txt", []byte("line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10\n"), 0644)
			fm := files.NewFileManager()
			actual, err := fm.ApplyPatch(context.Background(), filesystem, "test.txt", tt.patch)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if !actual.Equals(&tt.expected) {
				t.Errorf("Expected %+v, got %+v", tt.expected, actual)
			}
		})
	}
}

func TestFileManagerImpl_ApplyPatch_Failure(t *testing.T) {
	tests := []struct {
		name          string
		file          string
		patch         string
		expectedError string
	}{
		{
			name:          "File not found",
			file:          "not-found.txt",
			patch:         "",
			expectedError: "failed to read file",
		},
		{
			name: "Patch with multiple files",
			patch: `--- file1
+++ file1
@@ -1,3 +1,3 @@
 line1
-line2
+line20
 line3
--- file2
+++ file2
@@ -1,3 +1,3 @@
 line1
-line2
+line20
 line3
`,
			expectedError: "multiple files",
		},
		{
			name:          "Patch with no files",
			patch:         "",
			expectedError: "no files changed in patch",
		},
		{
			name: "Patch cannot be applied (no newline at end of file)",
			patch: `--- test.txt
+++ test.txt
@@ -1,3 +1,3 @@
 line1
-line2
+line20
 line3`,
			expectedError: "failed to apply patch",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fileSystem := afero.NewMemMapFs()
			afero.WriteFile(fileSystem, "test.txt", []byte("line1\nline2\nline3\n"), 0644)
			fm := files.NewFileManager()
			_, err := fm.ApplyPatch(context.Background(), fileSystem, tt.file, tt.patch)
			if err == nil {
				t.Fatalf("Expected error, got nil")
			}

			if !strings.Contains(strings.ToLower(err.Error()), tt.expectedError) {
				t.Errorf("Expected error to contain '%s', got %s", tt.expectedError, err.Error())
			}
		})
	}
}

func TestFileManagerImpl_UpdateLines_Success(t *testing.T) {
	tests := []struct {
		name     string
		lineDiff files.LineDiffChunk
		expected model.File
	}{
		{
			name: "Update 1 line",
			lineDiff: files.LineDiffChunk{
				StartLine: 1,
				EndLine:   1,
				Content:   "line11",
			},
			expected: model.File{
				Path:    "test.txt",
				Content: "line11\nline2\nline3\n",
			},
		},
		{
			name: "Update multiple lines",
			lineDiff: files.LineDiffChunk{
				StartLine: 1,
				EndLine:   2,
				Content:   "line11\nline12\n",
			},
			expected: model.File{
				Path:    "test.txt",
				Content: "line11\nline12\nline3\n",
			},
		},
		{
			name: "Add multiple lines at the end",
			lineDiff: files.LineDiffChunk{
				StartLine: 3,
				EndLine:   3,
				Content:   "line3\nline11\nline12\n",
			},
			expected: model.File{
				Path:    "test.txt",
				Content: "line1\nline2\nline3\nline11\nline12\n",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm := files.NewFileManager()
			filesystem := afero.NewMemMapFs()
			afero.WriteFile(filesystem, "test.txt", []byte("line1\nline2\nline3\n"), 0644)
			actual, err := fm.UpdateLines(context.Background(), filesystem, "test.txt", tt.lineDiff)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if !actual.Equals(&tt.expected) {
				t.Errorf("Expected %+v, got %+v", tt.expected, actual)
			}
		})
	}
}

func TestFileManagerImpl_UpdateLines_Failure(t *testing.T) {
	tests := []struct {
		name     string
		lineDiff files.LineDiffChunk
		expected string
	}{
		{
			name: "Start line > number of lines",
			lineDiff: files.LineDiffChunk{
				StartLine: 11,
				EndLine:   10,
				Content:   "line11",
			},
			expected: "Start line must be less than or equal to 3",
		},
		{
			name: "End line > number of lines",
			lineDiff: files.LineDiffChunk{
				StartLine: 1,
				EndLine:   11,
				Content:   "line11",
			},
			expected: "End line must be less than or equal to 3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filesystem := afero.NewMemMapFs()
			afero.WriteFile(filesystem, "test.txt", []byte("line1\nline2\nline3\n"), 0644)
			fm := files.NewFileManager()
			_, err := fm.UpdateLines(context.Background(), filesystem, "test.txt", tt.lineDiff)
			if err == nil {
				t.Fatalf("Expected error, got nil")
			}

			if !strings.Contains(err.Error(), tt.expected) {
				t.Errorf("Expected error to contain '%s', got %s", tt.expected, err.Error())
			}
		})
	}
}

func TestUpdateFile_Success(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected model.File
	}{
		{
			name:     "Update file",
			content:  "line1\nline2\nline3\n",
			expected: model.File{Path: "test.txt", Content: "line1\nline2\nline3\n"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filesystem := afero.NewMemMapFs()
			afero.WriteFile(filesystem, "test.txt", []byte("line11\nline12\n"), 0644)
			fm := files.NewFileManager()
			actual, err := fm.UpdateFile(context.Background(), filesystem, "test.txt", tt.content)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if !actual.Equals(&tt.expected) {
				t.Errorf("Expected %+v, got %+v", tt.expected, actual)
			}
		})
	}
}

func TestUpdateFile_Failure(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "File not found",
			content:  "whatever",
			expected: "file test.txt does not exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filesystem := afero.NewMemMapFs()
			fm := files.NewFileManager()
			_, err := fm.UpdateFile(context.Background(), filesystem, "test.txt", tt.content)
			if err == nil {
				t.Fatalf("Expected error, got nil")
			}

			if !strings.Contains(strings.ToLower(err.Error()), tt.expected) {
				t.Errorf("Expected error to contain '%s', got %s", tt.expected, err.Error())
			}
		})
	}
}
