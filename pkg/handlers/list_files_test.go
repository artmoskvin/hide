package handlers_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hide-org/hide/pkg/files"
	mockfiles "github.com/hide-org/hide/pkg/files/mocks"
	"github.com/hide-org/hide/pkg/handlers"
	"github.com/hide-org/hide/pkg/model"
	"github.com/hide-org/hide/pkg/project"
	"github.com/hide-org/hide/pkg/project/mocks"
	"github.com/stretchr/testify/assert"
)

func TestListFilesHandler_ServeHTTP(t *testing.T) {
	tests := []struct {
		name              string
		target            string
		mockListFilesFunc func(ctx context.Context, projectId string, opts ...files.ListFileOption) (model.Files, error)
		wantStatusCode    int
		wantBody          string
		asText            bool
	}{
		{
			name:   "successful listing",
			target: "/projects/123/files",
			mockListFilesFunc: func(ctx context.Context, projectId string, opts ...files.ListFileOption) (model.Files, error) {
				return model.Files{
					model.EmptyFile("file1.txt"),
					model.EmptyFile("file2.txt"),
				}, nil
			},
			wantStatusCode: http.StatusOK,
			wantBody:       `[{"path":"file1.txt"},{"path":"file2.txt"}]`,
		},
		{
			name:   "successful listing as text",
			target: "/projects/123/files",
			mockListFilesFunc: func(ctx context.Context, projectId string, opts ...files.ListFileOption) (model.Files, error) {
				return model.Files{
					model.EmptyFile("file1.txt"),
					model.EmptyFile("file2.txt"),
				}, nil
			},
			wantStatusCode: http.StatusOK,
			asText:         true,
			wantBody:       ".\n├── file1.txt\n└── file2.txt",
		},
		{
			name:   "successful listing with hidden",
			target: "/projects/123/files?showHidden",
			mockListFilesFunc: func(ctx context.Context, projectId string, opts ...files.ListFileOption) (model.Files, error) {
				return model.Files{
					model.EmptyFile("file1.txt"),
					model.EmptyFile("file2.txt"),
				}, nil
			},
			wantStatusCode: http.StatusOK,
			wantBody:       `[{"path":"file1.txt"},{"path":"file2.txt"}]`,
		},
		{
			name:   "successful listing with filtering",
			target: "/projects/123/files?&include=*.txt&include=*.json&exclude=file1",
			mockListFilesFunc: func(ctx context.Context, projectId string, opts ...files.ListFileOption) (model.Files, error) {
				// check expectations of filter
				if diff := mockfiles.DiffListFilesOpts(
					files.ListFilesOptions{
						WithContent: false,
						Filter: files.PatternFilter{
							Include: []string{"*.txt", "*.json"},
							Exclude: []string{"file1"},
						},
					}, opts...); diff != "" {
					return nil, fmt.Errorf("filter does not match, diff %s", diff)
				}

				return model.Files{
					model.EmptyFile("file2.txt"),
					model.EmptyFile("file2.json"),
				}, nil
			},
			wantStatusCode: http.StatusOK,
			wantBody:       `[{"path":"file2.txt"},{"path":"file2.json"}]`,
		},
		{
			name:   "project not found",
			target: "/projects/123/files",
			mockListFilesFunc: func(ctx context.Context, projectId string, opts ...files.ListFileOption) (model.Files, error) {
				return nil, project.NewProjectNotFoundError(projectId)
			},
			wantStatusCode: http.StatusNotFound,
			wantBody:       "project 123 not found",
		},
		{
			name:   "internal server error",
			target: "/projects/123/files",
			mockListFilesFunc: func(ctx context.Context, projectId string, opts ...files.ListFileOption) (model.Files, error) {
				return nil, errors.New("internal error")
			},
			wantStatusCode: http.StatusInternalServerError,
			wantBody:       "Failed to list files: internal error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockProjectManager := &mocks.MockProjectManager{
				ListFilesFunc: tt.mockListFilesFunc,
			}

			handler := handlers.ListFilesHandler{
				ProjectManager: mockProjectManager,
			}

			router := handlers.NewRouter().WithListFilesHandler(handler).Build()

			req := httptest.NewRequest(http.MethodGet, tt.target, nil)
			if tt.asText {
				req.Header.Add("Accept", "text/plain")
			}
			rr := httptest.NewRecorder()

			router.ServeHTTP(rr, req)

			assert.Equal(t, tt.wantStatusCode, rr.Code)
			assert.Equal(t, strings.TrimSuffix(rr.Body.String(), "\n"), tt.wantBody)
		})
	}
}
