package handlers_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/artmoskvin/hide/pkg/devcontainer"
	"github.com/artmoskvin/hide/pkg/handlers"
	"github.com/artmoskvin/hide/pkg/project"
	"github.com/artmoskvin/hide/pkg/project/mocks"
	"github.com/gorilla/mux"
)

func TestCreateTaskHandler_Command_Success(t *testing.T) {
	// Expected result
	expectedResult := project.TaskResult{StdOut: "Test output", StdErr: "Test error", ExitCode: 0}

	// Setup
	mockManager := &mocks.MockProjectManager{
		CreateTaskFunc: func(projectId string, command string) (project.TaskResult, error) {
			return expectedResult, nil
		},
	}

	handler := handlers.CreateTaskHandler{Manager: mockManager}

	requestBody := handlers.TaskRequest{Command: new(string)}
	*requestBody.Command = "test command"

	body, _ := json.Marshal(requestBody)
	request, _ := http.NewRequest("POST", "/projects/123/tasks", bytes.NewBuffer(body))
	response := httptest.NewRecorder()
	router := mux.NewRouter()
	router.Handle("/projects/{id}/tasks", handler).Methods("POST")

	// Execute
	router.ServeHTTP(response, request)

	// Verify
	if response.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, response.Code)
	}

	var respResult project.TaskResult
	if err := json.NewDecoder(response.Body).Decode(&respResult); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if respResult != expectedResult {
		t.Errorf("Unexpected result returned: %+v", respResult)
	}
}

func TestCreateTaskHandler_Alias_Success(t *testing.T) {
	// Expected result
	expectedResult := project.TaskResult{StdOut: "Test output", StdErr: "Test error", ExitCode: 0}

	// Setup
	mockManager := &mocks.MockProjectManager{
		ResolveTaskAliasFunc: func(projectId string, alias string) (devcontainer.Task, error) {
			return devcontainer.Task{Command: "resolved command"}, nil
		},
		CreateTaskFunc: func(projectId string, command string) (project.TaskResult, error) {
			return expectedResult, nil
		},
	}

	handler := handlers.CreateTaskHandler{Manager: mockManager}

	requestBody := handlers.TaskRequest{Alias: new(string)}
	*requestBody.Alias = "test alias"

	body, _ := json.Marshal(requestBody)
	request, _ := http.NewRequest("POST", "/ignored", bytes.NewBuffer(body))
	response := httptest.NewRecorder()

	// Execute
	handler.ServeHTTP(response, request)

	// Verify
	if response.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, response.Code)
	}

	var respResult project.TaskResult
	if err := json.NewDecoder(response.Body).Decode(&respResult); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if respResult != expectedResult {
		t.Errorf("Unexpected result returned: %+v", respResult)
	}
}

func TestCreateTaskHandler_Failure(t *testing.T) {
	// Setup
	mockManager := &mocks.MockProjectManager{
		CreateTaskFunc: func(projectId string, command string) (project.TaskResult, error) {
			return project.TaskResult{}, errors.New("Test error")
		},
	}

	handler := handlers.CreateTaskHandler{Manager: mockManager}

	requestBody := handlers.TaskRequest{Command: new(string)}
	*requestBody.Command = "test command"

	body, _ := json.Marshal(requestBody)
	request, _ := http.NewRequest("POST", "/ignored", bytes.NewBuffer(body))
	response := httptest.NewRecorder()

	// Execute
	handler.ServeHTTP(response, request)

	// Verify
	if response.Code != http.StatusInternalServerError {
		t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, response.Code)
	}
}

func TestCreateTaskHandler_BadRequest(t *testing.T) {
	// Setup
	mockManager := &mocks.MockProjectManager{}

	handler := handlers.CreateTaskHandler{Manager: mockManager}

	request, _ := http.NewRequest("POST", "/projects/123/exec", bytes.NewBuffer([]byte("invalid json")))
	response := httptest.NewRecorder()

	// Execute
	handler.ServeHTTP(response, request)

	// Verify
	if response.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, response.Code)
	}
}

func TestTaskRequest_Validate(t *testing.T) {
	tests := []struct {
		name     string
		request  handlers.TaskRequest
		expected error
	}{
		{
			name:     "Empty request",
			request:  handlers.TaskRequest{},
			expected: errors.New("either command or alias must be provided"),
		},
		{
			name:     "Both command and alias",
			request:  handlers.TaskRequest{Command: new(string), Alias: new(string)},
			expected: errors.New("only one of command or alias must be provided"),
		},
		{
			name:     "Command provided",
			request:  handlers.TaskRequest{Command: new(string)},
			expected: nil,
		},
		{
			name:     "Alias provided",
			request:  handlers.TaskRequest{Alias: new(string)},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.request.Validate()
			if err == nil && tt.expected != nil {
				t.Errorf("Expected error %v, got nil", tt.expected)
			} else if err != nil && tt.expected == nil {
				t.Errorf("Unexpected error %v", err)
			} else if err != nil && tt.expected != nil && err.Error() != tt.expected.Error() {
				t.Errorf("Expected error %v, got %v", tt.expected, err)
			}
		})
	}
}
