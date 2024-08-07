package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/artmoskvin/hide/pkg/project"
)

type TaskRequest struct {
	Command *string `json:"command,omitempty"`
	Alias   *string `json:"alias,omitempty"`
}

func (t *TaskRequest) Validate() error {
	if t.Command == nil && t.Alias == nil {
		return errors.New("either command or alias must be provided")
	}

	if t.Command != nil && t.Alias != nil {
		return errors.New("only one of command or alias must be provided")
	}

	return nil
}

type CreateTaskHandler struct {
	Manager project.Manager
}

func (h CreateTaskHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	projectID, err := getProjectID(r)
	if err != nil {
		http.Error(w, "invalid project ID", http.StatusBadRequest)
	}

	var request TaskRequest

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Failed parsing request body", http.StatusBadRequest)
		return
	}

	if err := request.Validate(); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %s", err), http.StatusBadRequest)
		return
	}

	var command string

	if request.Alias != nil {
		task, err := h.Manager.ResolveTaskAlias(projectID, *request.Alias)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to resolve task alias: %s", err), http.StatusBadRequest)
			return
		}
		command = task.Command
	} else {
		command = *request.Command
	}

	taskResult, err := h.Manager.CreateTask(projectID, command)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to run task %s", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(taskResult)
}
