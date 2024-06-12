package handlers_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/artmoskvin/hide/pkg/handlers"
	"github.com/artmoskvin/hide/pkg/project"
	"github.com/artmoskvin/hide/pkg/project/mocks"
)

const repoUrl = "https://github.com/example/repo.git"

func TestCreateProjectHandler_Success(t *testing.T) {
	// Expected project
	expectedProject := project.Project{Id: "123", Path: "/test/path"}

	// Setup
	mockManager := &mocks.MockProjectManager{
		CreateProjectFunc: func(req project.CreateProjectRequest) (project.Project, error) {
			return expectedProject, nil
		},
	}

	handler := handlers.CreateProjectHandler{Manager: mockManager}

	requestBody := project.CreateProjectRequest{Repository: project.Repository{Url: repoUrl}}
	body, _ := json.Marshal(requestBody)
	request, _ := http.NewRequest("POST", "/projects", bytes.NewBuffer(body))
	response := httptest.NewRecorder()

	// Execute
	handler.ServeHTTP(response, request)

	// Verify
	if response.Code != http.StatusCreated {
		t.Errorf("Expected status %d, got %d", http.StatusCreated, response.Code)
	}

	var respProject project.Project
	if err := json.NewDecoder(response.Body).Decode(&respProject); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !reflect.DeepEqual(respProject, expectedProject) {
		t.Errorf("Unexpected project returned: %+v", respProject)
	}
}

func TestCreateProjectHandler_Failure(t *testing.T) {
	// Setup
	mockManager := &mocks.MockProjectManager{
		CreateProjectFunc: func(req project.CreateProjectRequest) (project.Project, error) {
			return project.Project{}, errors.New("Test error")
		},
	}

	handler := handlers.CreateProjectHandler{Manager: mockManager}

	requestBody := project.CreateProjectRequest{Repository: project.Repository{Url: repoUrl}}
	body, _ := json.Marshal(requestBody)
	request, _ := http.NewRequest("POST", "/projects", bytes.NewBuffer(body))
	response := httptest.NewRecorder()

	// Execute
	handler.ServeHTTP(response, request)

	// Verify
	if response.Code != http.StatusInternalServerError {
		t.Errorf("Expected status %d, got %d", http.StatusInternalServerError, response.Code)
	}
}

func TestCreateProjectHandler_BadRequest(t *testing.T) {
	// Setup
	mockManager := &mocks.MockProjectManager{}

	handler := handlers.CreateProjectHandler{Manager: mockManager}

	request, _ := http.NewRequest("POST", "/projects", bytes.NewBuffer([]byte("invalid json")))
	response := httptest.NewRecorder()

	// Execute
	handler.ServeHTTP(response, request)

	// Verify
	if response.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, response.Code)
	}
}
