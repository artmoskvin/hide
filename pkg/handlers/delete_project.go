package handlers

import (
	"fmt"
	"net/http"

	"github.com/artmoskvin/hide/pkg/project"
)

type DeleteProjectHandler struct {
	Manager project.Manager
}

func (h DeleteProjectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	projectID, err := getProjectID(r)
	if err != nil {
		http.Error(w, "invalid project ID", http.StatusBadRequest)
	}

	// TODO: check if project exists
	result := <-h.Manager.DeleteProject(projectID)

	if result.IsFailure() {
		http.Error(w, fmt.Sprintf("Failed to delete project: %s", result.Error), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNoContent)
}
