package handlers

import (
	"errors"
	"net/http"

	"github.com/artmoskvin/hide/pkg/project"
)

type DeleteFileHandler struct {
	ProjectManager project.Manager
}

func (h DeleteFileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	projectID, err := getProjectID(r)
	if err != nil {
		http.Error(w, "invalid project ID", http.StatusBadRequest)
	}

	filePath, err := getFilePath(r)
	if err != nil {
		http.Error(w, "invalid file path", http.StatusBadRequest)
	}

	if err := h.ProjectManager.DeleteFile(r.Context(), projectID, filePath); err != nil {
		var projectNotFoundError *project.ProjectNotFoundError
		if errors.As(err, &projectNotFoundError) {
			http.Error(w, projectNotFoundError.Error(), http.StatusNotFound)
			return
		}

		http.Error(w, "Failed to delete file", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
