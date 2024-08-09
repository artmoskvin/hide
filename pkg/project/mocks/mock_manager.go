package mocks

import (
	"context"

	"github.com/artmoskvin/hide/pkg/devcontainer"
	"github.com/artmoskvin/hide/pkg/files"
	"github.com/artmoskvin/hide/pkg/model"
	"github.com/artmoskvin/hide/pkg/project"
	"github.com/artmoskvin/hide/pkg/result"
)

// MockProjectManager is a mock of the project.Manager interface for testing
type MockProjectManager struct {
	CreateProjectFunc    func(request project.CreateProjectRequest) <-chan result.Result[model.Project]
	GetProjectFunc       func(projectId string) (model.Project, error)
	GetProjectsFunc      func() ([]*model.Project, error)
	DeleteProjectFunc    func(projectId string) <-chan result.Empty
	ResolveTaskAliasFunc func(projectId string, alias string) (devcontainer.Task, error)
	CreateTaskFunc       func(projectId string, command string) (project.TaskResult, error)
	CleanupFunc          func() error
	CreateFileFunc       func(ctx context.Context, projectId, path, content string) (model.File, error)
	ReadFileFunc         func(ctx context.Context, projectId, path string, props files.ReadProps) (model.File, error)
	UpdateFileFunc       func(ctx context.Context, projectId, path, content string) (model.File, error)
	DeleteFileFunc       func(ctx context.Context, projectId, path string) error
	ListFilesFunc        func(ctx context.Context, projectId string, showHidden bool) ([]model.File, error)
	ApplyPatchFunc       func(ctx context.Context, projectId, path, patch string) (model.File, error)
	UpdateLinesFunc      func(ctx context.Context, projectId, path string, lineDiff files.LineDiffChunk) (model.File, error)
}

func (m *MockProjectManager) CreateProject(request project.CreateProjectRequest) <-chan result.Result[model.Project] {
	return m.CreateProjectFunc(request)
}

func (m *MockProjectManager) GetProject(projectId string) (model.Project, error) {
	return m.GetProjectFunc(projectId)
}

func (m *MockProjectManager) GetProjects() ([]*model.Project, error) {
	return m.GetProjectsFunc()
}

func (m *MockProjectManager) DeleteProject(projectId string) <-chan result.Empty {
	return m.DeleteProjectFunc(projectId)
}

func (m *MockProjectManager) ResolveTaskAlias(projectId string, alias string) (devcontainer.Task, error) {
	return m.ResolveTaskAliasFunc(projectId, alias)
}

func (m *MockProjectManager) CreateTask(projectId string, command string) (project.TaskResult, error) {
	return m.CreateTaskFunc(projectId, command)
}

func (m *MockProjectManager) Cleanup() error {
	return m.CleanupFunc()
}

func (m *MockProjectManager) CreateFile(ctx context.Context, projectId, path, content string) (model.File, error) {
	return m.CreateFileFunc(ctx, projectId, path, content)
}

func (m *MockProjectManager) ReadFile(ctx context.Context, projectId, path string, props files.ReadProps) (model.File, error) {
	return m.ReadFileFunc(ctx, projectId, path, props)
}

func (m *MockProjectManager) UpdateFile(ctx context.Context, projectId, path, content string) (model.File, error) {
	return m.UpdateFileFunc(ctx, projectId, path, content)
}

func (m *MockProjectManager) DeleteFile(ctx context.Context, projectId, path string) error {
	return m.DeleteFileFunc(ctx, projectId, path)
}

func (m *MockProjectManager) ListFiles(ctx context.Context, projectId string, showHidden bool) ([]model.File, error) {
	return m.ListFilesFunc(ctx, projectId, showHidden)
}

func (m *MockProjectManager) ApplyPatch(ctx context.Context, projectId, path, patch string) (model.File, error) {
	return m.ApplyPatchFunc(ctx, projectId, path, patch)
}

func (m *MockProjectManager) UpdateLines(ctx context.Context, projectId, path string, lineDiff files.LineDiffChunk) (model.File, error) {
	return m.UpdateLinesFunc(ctx, projectId, path, lineDiff)
}
