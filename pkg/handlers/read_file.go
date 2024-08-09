package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/artmoskvin/hide/pkg/files"
	"github.com/artmoskvin/hide/pkg/project"
)

type ReadFileHandler struct {
	ProjectManager project.Manager
}

func (h ReadFileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	projectId := r.PathValue("id")
	filePath := r.PathValue("path")
	queryParams := r.URL.Query()

	showLineNumbers, err := parseBoolQueryParam(queryParams, "showLineNumbers", files.DefaultShowLineNumbers)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	startLine, err := parseIntQueryParam(queryParams, "startLine", files.DefaultStartLine)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	numLines, err := parseIntQueryParam(queryParams, "numLines", files.DefaultNumLines)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	file, err := h.ProjectManager.ReadFile(r.Context(), projectId, filePath, files.NewReadProps(
		func(props *files.ReadProps) {
			props.ShowLineNumbers = showLineNumbers
			props.StartLine = startLine
			props.NumLines = numLines
		},
	))

	if err != nil {
		var projectNotFoundError *project.ProjectNotFoundError
		if errors.As(err, &projectNotFoundError) {
			http.Error(w, projectNotFoundError.Error(), http.StatusNotFound)
			return
		}

		http.Error(w, fmt.Sprintf("Failed to read file: %s", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(file)
}
