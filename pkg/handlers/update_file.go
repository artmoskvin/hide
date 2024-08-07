package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/artmoskvin/hide/pkg/files"
	"github.com/artmoskvin/hide/pkg/model"
	"github.com/artmoskvin/hide/pkg/project"
)

type UpdateType string

const (
	Udiff     UpdateType = "udiff"
	LineDiff  UpdateType = "linediff"
	Overwrite UpdateType = "overwrite"
)

type UdiffRequest struct {
	Patch string `json:"patch"`
}

type LineDiffRequest struct {
	StartLine int    `json:"startLine"`
	EndLine   int    `json:"endLine"`
	Content   string `json:"content"`
}

type OverwriteRequest struct {
	Content string `json:"content"`
}

type UpdateFileRequest struct {
	Type      UpdateType        `json:"type"`
	Udiff     *UdiffRequest     `json:"udiff,omitempty"`
	LineDiff  *LineDiffRequest  `json:"linediff,omitempty"`
	Overwrite *OverwriteRequest `json:"overwrite,omitempty"`
}

func (r *UpdateFileRequest) Validate() error {
	if r.Type == "" {
		return errors.New("type must be provided")
	}

	switch r.Type {
	case Udiff:
		if r.Udiff == nil {
			return errors.New("udiff must be provided")
		}
	case LineDiff:
		if r.LineDiff == nil {
			return errors.New("lineDiff must be provided")
		}
	case Overwrite:
		if r.Overwrite == nil {
			return errors.New("overwrite must be provided")
		}
	default:
		return fmt.Errorf("invalid type: %s", r.Type)
	}

	return nil
}

type UpdateFileHandler struct {
	ProjectManager project.Manager
}

func (h UpdateFileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	projectID, err := getProjectID(r)
	if err != nil {
		http.Error(w, "invalid project ID", http.StatusBadRequest)
	}

	filePath, err := getFilePath(r)
	if err != nil {
		http.Error(w, "invalid file path", http.StatusBadRequest)
	}

	var request UpdateFileRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Failed parsing request body", http.StatusBadRequest)
		return
	}

	if err := request.Validate(); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %s", err), http.StatusBadRequest)
		return
	}

	var file model.File

	switch request.Type {
	case Udiff:
		updatedFile, err := h.ProjectManager.ApplyPatch(r.Context(), projectID, filePath, request.Udiff.Patch)
		if err != nil {
			var projectNotFoundError *project.ProjectNotFoundError
			if errors.As(err, &projectNotFoundError) {
				http.Error(w, projectNotFoundError.Error(), http.StatusNotFound)
				return
			}

			http.Error(w, "Failed to update file", http.StatusInternalServerError)
			return
		}
		file = updatedFile
	case LineDiff:
		lineDiff := request.LineDiff
		updatedFile, err := h.ProjectManager.UpdateLines(r.Context(), projectID, filePath, files.LineDiffChunk{StartLine: lineDiff.StartLine, EndLine: lineDiff.EndLine, Content: lineDiff.Content})
		if err != nil {
			var projectNotFoundError *project.ProjectNotFoundError
			if errors.As(err, &projectNotFoundError) {
				http.Error(w, projectNotFoundError.Error(), http.StatusNotFound)
				return
			}

			http.Error(w, "Failed to update file", http.StatusInternalServerError)
			return
		}
		file = updatedFile
	case Overwrite:
		updatedFile, err := h.ProjectManager.UpdateFile(r.Context(), projectID, filePath, request.Overwrite.Content)
		if err != nil {
			var projectNotFoundError *project.ProjectNotFoundError
			if errors.As(err, &projectNotFoundError) {
				http.Error(w, projectNotFoundError.Error(), http.StatusNotFound)
				return
			}

			http.Error(w, "Failed to update file", http.StatusInternalServerError)
			return
		}
		file = updatedFile
	default:
		http.Error(w, "Invalid request: type must be provided", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(file)
}
