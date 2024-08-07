package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/artmoskvin/hide/pkg/project"
)

type FileResponse struct {
	Path string `json:"path"`
}

type ListFilesHandler struct {
	ProjectManager project.Manager
}

func (h ListFilesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	projectID, err := getProjectID(r)
	if err != nil {
		http.Error(w, "invalid project ID", http.StatusBadRequest)
	}

	files, err := h.ProjectManager.ListFiles(r.Context(), projectID)
	if err != nil {
		var projectNotFoundError *project.ProjectNotFoundError
		if errors.As(err, &projectNotFoundError) {
			http.Error(w, projectNotFoundError.Error(), http.StatusNotFound)
			return
		}

		http.Error(w, "Failed to list files", http.StatusInternalServerError)
		return
	}

	var fileResponses []FileResponse

	for _, file := range files {
		fileResponses = append(fileResponses, FileResponse{Path: file.Path})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(fileResponses)
}
