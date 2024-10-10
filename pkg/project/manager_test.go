package project_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/hide-org/hide/pkg/devcontainer"
	dc_mocks "github.com/hide-org/hide/pkg/devcontainer/mocks"
	"github.com/hide-org/hide/pkg/lsp"
	lsp_mocks "github.com/hide-org/hide/pkg/lsp/mocks"
	"github.com/hide-org/hide/pkg/model"
	"github.com/hide-org/hide/pkg/project"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestProject_findTaskByAlias(t *testing.T) {
	project := model.Project{
		Config: model.Config{
			DevContainerConfig: devcontainer.Config{
				GeneralProperties: devcontainer.GeneralProperties{
					Customizations: devcontainer.Customizations{
						Hide: &devcontainer.HideCustomization{
							Tasks: []devcontainer.Task{
								{Alias: "test-task", Command: "echo test"},
							},
						},
					},
				},
			},
		},
	}

	task, err := project.FindTaskByAlias("test-task")

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if task.Alias != "test-task" {
		t.Errorf("Expected task alias to be test-task, got %s", task.Alias)
	}
}

func TestProject_findTaskByAlias_notFound(t *testing.T) {
	project := model.Project{
		Config: model.Config{
			DevContainerConfig: devcontainer.Config{
				GeneralProperties: devcontainer.GeneralProperties{
					Customizations: devcontainer.Customizations{
						Hide: &devcontainer.HideCustomization{
							Tasks: []devcontainer.Task{
								{Alias: "test-task", Command: "echo test"},
							},
						},
					},
				},
			},
		},
	}

	_, err := project.FindTaskByAlias("missing-task")

	if err == nil {
		t.Errorf("Expected error, got nil")
	}
}

func TestManagerImpl_CreateProject(t *testing.T) {
	t.Skip("Skipping test because it depends on external shell command `git` and file system")
}

func TestManagerImpl_GetProject_Succeeds(t *testing.T) {
	_project := model.Project{Id: "test-project", Path: "/tmp/test-project", Config: model.Config{}}
	pm := project.NewProjectManager(nil, project.NewInMemoryStore(map[string]*model.Project{"test-project": &_project}), "/tmp", nil, nil, nil, nil, nil)
	project, err := pm.GetProject(context.Background(), "test-project")

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if !reflect.DeepEqual(project, _project) {
		t.Errorf("Expected project id to be test-project, got %s", project.Id)
	}
}

func TestManagerImpl_GetProject_Fails(t *testing.T) {
	pm := project.NewProjectManager(nil, project.NewInMemoryStore(map[string]*model.Project{}), "/tmp", nil, nil, nil, nil, nil)
	_, err := pm.GetProject(context.Background(), "missing-project")

	if err == nil {
		t.Errorf("Expected error, got nil")
	}
}

func TestManagerImpl_ResolveTaskAlias_Succeeds(t *testing.T) {
	task := devcontainer.Task{Alias: "test-alias", Command: "echo test"}
	_project := model.Project{
		Id:   "test-project",
		Path: "/tmp/test-project",
		Config: model.Config{
			DevContainerConfig: devcontainer.Config{
				GeneralProperties: devcontainer.GeneralProperties{
					Customizations: devcontainer.Customizations{
						Hide: &devcontainer.HideCustomization{
							Tasks: []devcontainer.Task{task},
						},
					},
				},
			},
		},
	}
	pm := project.NewProjectManager(nil, project.NewInMemoryStore(map[string]*model.Project{"test-project": &_project}), "/tmp", nil, nil, nil, nil, nil)
	resolvedTask, err := pm.ResolveTaskAlias(context.Background(), "test-project", "test-alias")

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if !reflect.DeepEqual(resolvedTask, task) {
		t.Errorf("Expected task alias to be test-alias, got %s", resolvedTask.Alias)
	}
}

func TestManagerImpl_ResolveTaskAlias_ProjectNotFound(t *testing.T) {
	pm := project.NewProjectManager(nil, project.NewInMemoryStore(map[string]*model.Project{}), "/tmp", nil, nil, nil, nil, nil)
	_, err := pm.ResolveTaskAlias(context.Background(), "missing-project", "test-alias")

	if err == nil {
		t.Errorf("Expected error, got nil")
	}
}

func TestManagerImpl_ResolveTaskAlias_TaskNotFound(t *testing.T) {
	_project := model.Project{Id: "test-project", Path: "/tmp/test-project", Config: model.Config{}}
	pm := project.NewProjectManager(nil, project.NewInMemoryStore(map[string]*model.Project{"test-project": &_project}), "/tmp", nil, nil, nil, nil, nil)
	_, err := pm.ResolveTaskAlias(context.Background(), "test-project", "missing-alias")

	if err == nil {
		t.Errorf("Expected error, got nil")
	}
}

func TestManagerImpl_CreateTask(t *testing.T) {
	const projectId = "test-project"
	_project := model.NewProject(projectId, "/tmp/test-project", model.Config{}, "test-container")
	devContainerRunner := &dc_mocks.MockDevContainerRunner{
		ExecFunc: func(ctx context.Context, containerId string, command []string) (devcontainer.ExecResult, error) {
			return devcontainer.ExecResult{StdOut: "test-stdout", StdErr: "test-stderr", ExitCode: 1}, nil
		}}
	pm := project.NewProjectManager(devContainerRunner, project.NewInMemoryStore(map[string]*model.Project{projectId: &_project}), "/tmp", nil, nil, nil, nil, nil)

	taskResult, err := pm.CreateTask(context.Background(), projectId, "echo test")

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	expectedTaskResult := project.TaskResult{StdOut: "test-stdout", StdErr: "test-stderr", ExitCode: 1}

	if !reflect.DeepEqual(taskResult, expectedTaskResult) {
		t.Errorf("Expected empty stdout, got %s", taskResult.StdOut)
	}
}

func TestManagerImpl_CreateTask_ProjectNotFound(t *testing.T) {
	pm := project.NewProjectManager(nil, project.NewInMemoryStore(map[string]*model.Project{}), "/tmp", nil, nil, nil, nil, nil)
	_, err := pm.CreateTask(context.Background(), "missing-project", "echo test")

	if err == nil {
		t.Errorf("Expected error, got nil")
	}
}

func TestManagerImpl_CreateTask_ExecError(t *testing.T) {
	const projectId = "test-project"
	_project := model.NewProject(projectId, "/tmp/test-project", model.Config{}, "test-container")
	devContainerRunner := &dc_mocks.MockDevContainerRunner{
		ExecFunc: func(ctx context.Context, containerId string, command []string) (devcontainer.ExecResult, error) {
			return devcontainer.ExecResult{}, errors.New("exec error")
		},
	}
	pm := project.NewProjectManager(devContainerRunner, project.NewInMemoryStore(map[string]*model.Project{projectId: &_project}), "/tmp", nil, nil, nil, nil, nil)

	_, err := pm.CreateTask(context.Background(), projectId, "echo test")

	if err == nil {
		t.Errorf("Expected error, got nil")
	}
}

func TestManagerImpl_SearchSymbols(t *testing.T) {
	symbols := []lsp.SymbolInfo{
		{Name: "symbol1", Kind: "kind1", Location: lsp.Location{Path: "path1", Range: lsp.Range{Start: lsp.Position{Line: 1, Character: 1}, End: lsp.Position{Line: 2, Character: 2}}}},
		{Name: "symbol2", Kind: "kind2", Location: lsp.Location{Path: "path2", Range: lsp.Range{Start: lsp.Position{Line: 3, Character: 3}, End: lsp.Position{Line: 4, Character: 4}}}},
	}

	tests := []struct {
		name         string
		store        map[string]*model.Project
		mockSetup    func(*lsp_mocks.MockLspService)
		context      context.Context
		projectId    string
		query        string
		symbolFilter lsp.SymbolFilter
		want         []lsp.SymbolInfo
		wantErr      string
	}{
		{
			name: "success",
			store: map[string]*model.Project{
				"project-id": {},
			},
			mockSetup: func(m *lsp_mocks.MockLspService) {
				m.On("GetWorkspaceSymbols", mock.Anything, "query").Return(symbols, nil)
			},
			context:   context.Background(),
			projectId: "project-id",
			query:     "query",
			want:      symbols,
		},
		{
			name:      "project not found",
			store:     map[string]*model.Project{},
			mockSetup: func(m *lsp_mocks.MockLspService) {},
			context:   context.Background(),
			projectId: "project-id",
			query:     "query",
			wantErr:   "project project-id not found",
		},
		{
			name: "failed to get workspace symbols",
			store: map[string]*model.Project{
				"project-id": {},
			},
			mockSetup: func(m *lsp_mocks.MockLspService) {
				m.On("GetWorkspaceSymbols", mock.Anything, "query").Return(nil, errors.New("failed to get workspace symbols"))
			},
			context:   context.Background(),
			projectId: "project-id",
			query:     "query",
			wantErr:   "failed to get workspace symbols",
		},
		{
			name: "context cancelled",
			store: map[string]*model.Project{
				"project-id": {},
			},
			mockSetup: func(m *lsp_mocks.MockLspService) {},
			context: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()

				return ctx
			}(),
			projectId: "project-id",
			query:     "query",
			wantErr:   "context cancelled",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lspService := &lsp_mocks.MockLspService{}
			tt.mockSetup(lspService)
			pm := project.NewProjectManager(nil, project.NewInMemoryStore(tt.store), "/tmp", nil, lspService, nil, nil, nil)

			symbols, err := pm.SearchSymbols(tt.context, tt.projectId, tt.query, tt.symbolFilter)

			if tt.wantErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, symbols)
			}

		})
	}
}
