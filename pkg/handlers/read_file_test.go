package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/artmoskvin/hide/pkg/files"
	"github.com/artmoskvin/hide/pkg/handlers"
	"github.com/artmoskvin/hide/pkg/model"
	"github.com/artmoskvin/hide/pkg/project"
	project_mocks "github.com/artmoskvin/hide/pkg/project/mocks"
)

func TestReadFileHandler_Success(t *testing.T) {
	tests := []struct {
		name         string
		query        string
		expectedFile model.File
	}{
		{
			name:  "Read file with default params",
			query: "",
			expectedFile: model.File{
				Path:    "test.txt",
				Content: "line1\nline2\nline3\n",
			},
		},
		{
			name:  "Read file with showLineNumbers=true",
			query: "showLineNumbers=true",
			expectedFile: model.File{
				Path:    "test.txt",
				Content: "1:line1\n2:line2\n3:line3\n",
			},
		},
		{
			name:  "Read file with startLine=2",
			query: "startLine=2",
			expectedFile: model.File{
				Path:    "test.txt",
				Content: "2:line2\n3:line3\n",
			},
		},
		{
			name:  "Read file with numLines=2",
			query: "numLines=2",
			expectedFile: model.File{
				Path:    "test.txt",
				Content: "line1\nline2\n",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockManager := &project_mocks.MockProjectManager{
				ReadFileFunc: func(ctx context.Context, projectId string, path string, props files.ReadProps) (model.File, error) {
					return tt.expectedFile, nil
				},
			}

			handler := handlers.ReadFileHandler{ProjectManager: mockManager}

			request, _ := http.NewRequest("GET", "/projects/123/files/test.txt?"+tt.query, nil)
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", response.Code)
			}

			var respFile model.File
			if err := json.NewDecoder(response.Body).Decode(&respFile); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if !respFile.Equals(&tt.expectedFile) {
				t.Errorf("Expected file %+v, got %+v", tt.expectedFile, respFile)
			}
		})
	}
}

func TestReadFileHandler_Fails_WithInvalidQueryParams(t *testing.T) {
	tests := []struct {
		name         string
		query        string
		expectedCode int
	}{
		{
			name:         "Read file with invalid showLineNumbers param",
			query:        "showLineNumbers=invalid",
			expectedCode: http.StatusBadRequest,
		},
		{
			name:         "Read file with invalid startLine param",
			query:        "startLine=invalid",
			expectedCode: http.StatusBadRequest,
		},
		{
			name:         "Read file with invalid numLines param",
			query:        "numLines=invalid",
			expectedCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := handlers.ReadFileHandler{}

			request, _ := http.NewRequest("GET", "/projects/123/files/test.txt?"+tt.query, nil)
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != tt.expectedCode {
				t.Errorf("Expected status %d, got %d", tt.expectedCode, response.Code)
				t.Errorf("Body: %s", response.Body.String())
			}
		})
	}
}

func TestReadFileHandler_Returns404_WhenProjectNotFound(t *testing.T) {
	t.Run("Read file with invalid project ID", func(t *testing.T) {
		mockManager := &project_mocks.MockProjectManager{
			ReadFileFunc: func(ctx context.Context, projectId string, path string, props files.ReadProps) (model.File, error) {
				return model.File{}, &project.ProjectNotFoundError{ProjectId: projectId}
			},
		}

		handler := handlers.ReadFileHandler{ProjectManager: mockManager}

		request, _ := http.NewRequest("GET", "/projects/123/files/test.txt", nil)
		response := httptest.NewRecorder()

		handler.ServeHTTP(response, request)

		if response.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", response.Code)
		}
	})
}

func TestReadFileHandler_Returns500_WhenReadFileFails(t *testing.T) {
	t.Run("Read file with invalid file path", func(t *testing.T) {
		mockManager := &project_mocks.MockProjectManager{
			ReadFileFunc: func(ctx context.Context, projectId string, path string, props files.ReadProps) (model.File, error) {
				return model.File{}, errors.New("file not found")
			},
		}

		handler := handlers.ReadFileHandler{ProjectManager: mockManager}

		request, _ := http.NewRequest("GET", "/projects/123/files/invalid.txt", nil)
		response := httptest.NewRecorder()

		handler.ServeHTTP(response, request)

		if response.Code != http.StatusInternalServerError {
			t.Errorf("Expected status 500, got %d", response.Code)
		}
	})
}
