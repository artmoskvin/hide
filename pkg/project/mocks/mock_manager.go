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
	CreateProjectFunc    func(ctx context.Context, request project.CreateProjectRequest) <-chan result.Result[model.Project]
	GetProjectFunc       func(ctx context.Context, projectId string) (model.Project, error)
	GetProjectsFunc      func(ctx context.Context) ([]*model.Project, error)
	DeleteProjectFunc    func(ctx context.Context, projectId string) <-chan result.Empty
	ResolveTaskAliasFunc func(ctx context.Context, projectId string, alias string) (devcontainer.Task, error)
	CreateTaskFunc       func(ctx context.Context, projectId string, command string) (project.TaskResult, error)
	CleanupFunc          func(ctx context.Context) error
	CreateFileFunc       func(ctx context.Context, projectId, path, content string) (*model.File, error)
	ReadFileFunc         func(ctx context.Context, projectId, path string) (*model.File, error)
	UpdateFileFunc       func(ctx context.Context, projectId, path, content string) (*model.File, error)
	DeleteFileFunc       func(ctx context.Context, projectId, path string) error
	ListFilesFunc        func(ctx context.Context, projectId string, showHidden bool) ([]*model.File, error)
	ApplyPatchFunc       func(ctx context.Context, projectId, path, patch string) (*model.File, error)
	UpdateLinesFunc      func(ctx context.Context, projectId, path string, lineDiff files.LineDiffChunk) (*model.File, error)
}

func (m *MockProjectManager) CreateProject(ctx context.Context, request project.CreateProjectRequest) <-chan result.Result[model.Project] {
	return m.CreateProjectFunc(ctx, request)
}

func (m *MockProjectManager) GetProject(ctx context.Context, projectId string) (model.Project, error) {
	return m.GetProjectFunc(ctx, projectId)
}

func (m *MockProjectManager) GetProjects(ctx context.Context) ([]*model.Project, error) {
	return m.GetProjectsFunc(ctx)
}

func (m *MockProjectManager) DeleteProject(ctx context.Context, projectId string) <-chan result.Empty {
	return m.DeleteProjectFunc(ctx, projectId)
}

func (m *MockProjectManager) ResolveTaskAlias(ctx context.Context, projectId string, alias string) (devcontainer.Task, error) {
	return m.ResolveTaskAliasFunc(ctx, projectId, alias)
}

func (m *MockProjectManager) CreateTask(ctx context.Context, projectId string, command string) (project.TaskResult, error) {
	return m.CreateTaskFunc(ctx, projectId, command)
}

func (m *MockProjectManager) Cleanup(ctx context.Context) error {
	return m.CleanupFunc(ctx)
}

func (m *MockProjectManager) CreateFile(ctx context.Context, projectId, path, content string) (*model.File, error) {
	return m.CreateFileFunc(ctx, projectId, path, content)
}

func (m *MockProjectManager) ReadFile(ctx context.Context, projectId, path string) (*model.File, error) {
	return m.ReadFileFunc(ctx, projectId, path)
}

func (m *MockProjectManager) UpdateFile(ctx context.Context, projectId, path, content string) (*model.File, error) {
	return m.UpdateFileFunc(ctx, projectId, path, content)
}

func (m *MockProjectManager) DeleteFile(ctx context.Context, projectId, path string) error {
	return m.DeleteFileFunc(ctx, projectId, path)
}

func (m *MockProjectManager) ListFiles(ctx context.Context, projectId string, showHidden bool) ([]*model.File, error) {
	return m.ListFilesFunc(ctx, projectId, showHidden)
}

func (m *MockProjectManager) ApplyPatch(ctx context.Context, projectId, path, patch string) (*model.File, error) {
	return m.ApplyPatchFunc(ctx, projectId, path, patch)
}

func (m *MockProjectManager) UpdateLines(ctx context.Context, projectId, path string, lineDiff files.LineDiffChunk) (*model.File, error) {
	return m.UpdateLinesFunc(ctx, projectId, path, lineDiff)
}
