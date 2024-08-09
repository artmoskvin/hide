package project

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"time"

	"github.com/artmoskvin/hide/pkg/devcontainer"
	"github.com/artmoskvin/hide/pkg/files"
	"github.com/artmoskvin/hide/pkg/lsp"
	"github.com/artmoskvin/hide/pkg/model"
	"github.com/artmoskvin/hide/pkg/result"
	"github.com/artmoskvin/hide/pkg/util"
	protocol "github.com/tliron/glsp/protocol_3_16"

	"github.com/rs/zerolog/log"

	"github.com/spf13/afero"
)

const MaxDiagnosticsDelay = time.Second * 1

type Repository struct {
	Url    string  `json:"url"`
	Commit *string `json:"commit,omitempty"`
}

type CreateProjectRequest struct {
	Repository   Repository           `json:"repository"`
	DevContainer *devcontainer.Config `json:"devcontainer,omitempty"`
}

type TaskResult struct {
	StdOut   string `json:"stdOut"`
	StdErr   string `json:"stdErr"`
	ExitCode int    `json:"exitCode"`
}

type Manager interface {
	CreateProject(request CreateProjectRequest) <-chan result.Result[model.Project]
	GetProject(projectId model.ProjectId) (model.Project, error)
	GetProjects() ([]*model.Project, error)
	DeleteProject(projectId model.ProjectId) <-chan result.Empty
	ResolveTaskAlias(projectId model.ProjectId, alias string) (devcontainer.Task, error)
	CreateTask(projectId model.ProjectId, command string) (TaskResult, error)
	Cleanup() error
	CreateFile(ctx context.Context, projectId, path, content string) (model.File, error)
	ReadFile(ctx context.Context, projectId, path string, props files.ReadProps) (model.File, error)
	UpdateFile(ctx context.Context, projectId, path, content string) (model.File, error)
	DeleteFile(ctx context.Context, projectId, path string) error
	ListFiles(ctx context.Context, projectId string, showHidden bool) ([]model.File, error)
	ApplyPatch(ctx context.Context, projectId, path, patch string) (model.File, error)
	UpdateLines(ctx context.Context, projectId, path string, lineDiff files.LineDiffChunk) (model.File, error)
}

type ManagerImpl struct {
	DevContainerRunner devcontainer.Runner
	Store              Store
	ProjectsRoot       string
	fileManager        files.FileManager
	lspService         lsp.Service
	languageDetector   lsp.LanguageDetector
}

func NewProjectManager(
	devContainerRunner devcontainer.Runner,
	projectStore Store,
	projectsRoot string,
	fileManager files.FileManager,
	lspService lsp.Service,
	languageDetector lsp.LanguageDetector,
) Manager {
	return ManagerImpl{
		DevContainerRunner: devContainerRunner,
		Store:              projectStore,
		ProjectsRoot:       projectsRoot,
		fileManager:        fileManager,
		lspService:         lspService,
		languageDetector:   languageDetector,
	}
}

func (pm ManagerImpl) CreateProject(request CreateProjectRequest) <-chan result.Result[model.Project] {
	c := make(chan result.Result[model.Project])

	go func() {
		log.Debug().Msgf("Creating project for repo %s", request.Repository.Url)

		projectId := util.RandomString(10)
		projectPath := path.Join(pm.ProjectsRoot, projectId)

		// Clone git repo
		if err := pm.createProjectDir(projectPath); err != nil {
			log.Error().Err(err).Msg("Failed to create project directory")
			c <- result.Failure[model.Project](fmt.Errorf("Failed to create project directory: %w", err))
			return
		}

		if r := <-cloneGitRepo(request.Repository, projectPath); r.IsFailure() {
			log.Error().Err(r.Error).Msg("Failed to clone git repo")
			removeProjectDir(projectPath)
			c <- result.Failure[model.Project](fmt.Errorf("Failed to clone git repo: %w", r.Error))
			return
		}

		// Start devcontainer
		var devContainerConfig devcontainer.Config

		if request.DevContainer != nil {
			devContainerConfig = *request.DevContainer
		} else {
			config, err := pm.configFromProject(os.DirFS(projectPath))

			if err != nil {
				log.Error().Err(err).Msgf("Failed to get devcontainer config from repository %s", request.Repository.Url)
				removeProjectDir(projectPath)
				c <- result.Failure[model.Project](fmt.Errorf("Failed to read devcontainer.json: %w", err))
				return
			}

			devContainerConfig = config
		}

		containerId, err := pm.DevContainerRunner.Run(projectPath, devContainerConfig)

		if err != nil {
			log.Error().Err(err).Msg("Failed to launch devcontainer")
			removeProjectDir(projectPath)
			c <- result.Failure[model.Project](fmt.Errorf("Failed to launch devcontainer: %w", err))
			return
		}

		project := model.Project{Id: projectId, Path: projectPath, Config: model.Config{DevContainerConfig: devContainerConfig}, ContainerId: containerId}

		// Start LSP server if language is supported
		files, err := pm.fileManager.ListFiles(model.NewContextWithProject(context.Background(), &project), afero.NewBasePathFs(afero.NewOsFs(), projectPath), false)
		if err != nil {
			log.Error().Err(err).Msg("Failed to list files")
			removeProjectDir(projectPath)
			c <- result.Failure[model.Project](fmt.Errorf("Failed to list files: %w", err))
			return
		}

		language := pm.languageDetector.DetectMainLanguage(files)
		log.Debug().Msgf("Detected main language %s for project %s", language, projectId)

		if err := pm.lspService.StartServer(model.NewContextWithProject(context.Background(), &project), language); err != nil {
			log.Error().Err(err).Msg("Failed to start LSP server")
			removeProjectDir(projectPath)
			c <- result.Failure[model.Project](fmt.Errorf("Failed to start LSP server: %w", err))
			return
		}

		// Save project in store
		if err := pm.Store.CreateProject(&project); err != nil {
			log.Error().Err(err).Msg("Failed to save project")
			removeProjectDir(projectPath)
			c <- result.Failure[model.Project](fmt.Errorf("Failed to save project: %w", err))
			return
		}

		log.Debug().Msgf("Created project %s for repo %s", projectId, request.Repository.Url)

		c <- result.Success(project)
	}()

	return c
}

func (pm ManagerImpl) GetProject(projectId string) (model.Project, error) {
	project, err := pm.Store.GetProject(projectId)

	if err != nil {
		log.Error().Err(err).Msgf("Failed to get project with id %s", projectId)
		return model.Project{}, err
	}

	return *project, nil
}

func (pm ManagerImpl) GetProjects() ([]*model.Project, error) {
	projects, err := pm.Store.GetProjects()

	if err != nil {
		log.Error().Err(err).Msg("Failed to get projects")
		return nil, fmt.Errorf("Failed to get projects: %w", err)
	}

	return projects, nil
}

func (pm ManagerImpl) DeleteProject(projectId string) <-chan result.Empty {
	c := make(chan result.Empty)

	go func() {
		log.Debug().Msgf("Deleting project %s", projectId)

		project, err := pm.GetProject(projectId)

		if err != nil {
			log.Error().Err(err).Msgf("Failed to get project with id %s", projectId)
			c <- result.EmptyFailure(fmt.Errorf("Failed to get project with id %s: %w", projectId, err))
			return
		}

		if err := pm.DevContainerRunner.Stop(project.ContainerId); err != nil {
			log.Error().Err(err).Msgf("Failed to stop container %s", project.ContainerId)
			c <- result.EmptyFailure(fmt.Errorf("Failed to stop container: %w", err))
			return
		}

		if err := pm.Store.DeleteProject(projectId); err != nil {
			log.Error().Err(err).Msgf("Failed to delete project %s", projectId)
			c <- result.EmptyFailure(fmt.Errorf("Failed to delete project: %w", err))
			return
		}

		log.Debug().Msgf("Deleted project %s", projectId)

		c <- result.EmptySuccess()
	}()

	return c
}

func (pm ManagerImpl) ResolveTaskAlias(projectId string, alias string) (devcontainer.Task, error) {
	log.Debug().Msgf("Resolving task alias %s for project %s", alias, projectId)

	project, err := pm.GetProject(projectId)

	if err != nil {
		log.Error().Err(err).Msgf("Failed to get project with id %s", projectId)
		return devcontainer.Task{}, fmt.Errorf("Failed to get project with id %s: %w", projectId, err)
	}

	task, err := project.FindTaskByAlias(alias)

	if err != nil {
		log.Error().Err(err).Msgf("Task with alias %s for project %s not found", alias, projectId)
		return devcontainer.Task{}, fmt.Errorf("Task with alias %s not found", alias)
	}

	log.Debug().Msgf("Resolved task alias %s for project %s: %+v", alias, projectId, task)

	return task, nil
}

func (pm ManagerImpl) CreateTask(projectId string, command string) (TaskResult, error) {
	log.Debug().Msgf("Creating task for project %s. Command: %s", projectId, command)

	project, err := pm.GetProject(projectId)

	if err != nil {
		log.Error().Err(err).Msgf("Failed to get project with id %s", projectId)
		return TaskResult{}, fmt.Errorf("Failed to get project with id %s: %w", projectId, err)
	}

	execResult, err := pm.DevContainerRunner.Exec(project.ContainerId, []string{"/bin/bash", "-c", command})

	if err != nil {
		log.Error().Err(err).Msgf("Failed to execute command '%s' in container %s", command, project.ContainerId)
		return TaskResult{}, fmt.Errorf("Failed to execute command: %w", err)
	}

	log.Debug().Msgf("Task '%s' for project %s completed", command, projectId)

	return TaskResult{StdOut: execResult.StdOut, StdErr: execResult.StdErr, ExitCode: execResult.ExitCode}, nil
}

func (pm ManagerImpl) Cleanup() error {
	log.Info().Msg("Cleaning up projects")

	projects, err := pm.GetProjects()

	if err != nil {
		log.Error().Err(err).Msg("Failed to get projects")
		return fmt.Errorf("Failed to get projects: %w", err)
	}

	for _, project := range projects {
		log.Debug().Msgf("Cleaning up project %s", project.Id)
		pm.DevContainerRunner.Stop(project.ContainerId)
	}

	log.Info().Msg("Cleaned up projects")

	return nil
}

func (pm ManagerImpl) CreateFile(ctx context.Context, projectId, path, content string) (model.File, error) {
	log.Debug().Msgf("Creating file %s in project %s", path, projectId)

	project, err := pm.GetProject(projectId)

	if err != nil {
		log.Error().Err(err).Msgf("Failed to get project with id %s", projectId)
		return model.File{}, fmt.Errorf("Failed to get project with id %s: %w", projectId, err)
	}

	ctx = model.NewContextWithProject(ctx, &project)

	file, err := pm.fileManager.CreateFile(ctx, afero.NewBasePathFs(afero.NewOsFs(), project.Path), path, content)

	if err != nil {
		log.Error().Err(err).Str("projectId", projectId).Msgf("Failed to create file %s", path)
		return file, err
	}

	// wait for diagnostics
	time.Sleep(MaxDiagnosticsDelay)
	diagnostics, err := pm.getDiagnostics(ctx, file)

	if err != nil {
		log.Error().Err(err).Str("projectId", projectId).Msgf("Failed to get diagnostics for file %s", path)
		return model.File{}, fmt.Errorf("Failed to get diagnostics for file %s: %w", path, err)
	}

	file.Diagnostics = diagnostics
	return file, nil
}

func (pm ManagerImpl) ReadFile(ctx context.Context, projectId, path string, props files.ReadProps) (model.File, error) {
	log.Debug().Msgf("Reading file %s in project %s", path, projectId)

	project, err := pm.GetProject(projectId)

	if err != nil {
		log.Error().Err(err).Msgf("Failed to get project with id %s", projectId)
		return model.File{}, fmt.Errorf("Failed to get project with id %s: %w", projectId, err)
	}

	ctx = model.NewContextWithProject(ctx, &project)
	file, err := pm.fileManager.ReadFile(ctx, afero.NewBasePathFs(afero.NewOsFs(), project.Path), path, props)

	if err != nil {
		log.Error().Err(err).Str("projectId", projectId).Msgf("Failed to read file %s", path)
		return file, err
	}

	// wait for diagnostics
	time.Sleep(MaxDiagnosticsDelay)
	diagnostics, err := pm.getDiagnostics(ctx, file)

	if err != nil {
		log.Error().Err(err).Str("projectId", projectId).Msgf("Failed to get diagnostics for file %s", path)
		return model.File{}, fmt.Errorf("Failed to get diagnostics for file %s: %w", path, err)
	}

	file.Diagnostics = diagnostics
	return file, nil
}

func (pm ManagerImpl) UpdateFile(ctx context.Context, projectId, path, content string) (model.File, error) {
	log.Debug().Msgf("Updating file %s in project %s", path, projectId)

	project, err := pm.GetProject(projectId)

	if err != nil {
		log.Error().Err(err).Msgf("Failed to get project with id %s", projectId)
		return model.File{}, fmt.Errorf("Failed to get project with id %s: %w", projectId, err)
	}

	ctx = model.NewContextWithProject(ctx, &project)

	file, err := pm.fileManager.UpdateFile(ctx, afero.NewBasePathFs(afero.NewOsFs(), project.Path), path, content)

	if err != nil {
		log.Error().Err(err).Str("projectId", projectId).Msgf("Failed to update file %s", path)
		return file, err
	}

	// wait for diagnostics
	time.Sleep(MaxDiagnosticsDelay)
	diagnostics, err := pm.getDiagnostics(ctx, file)

	if err != nil {
		log.Error().Err(err).Str("projectId", projectId).Msgf("Failed to get diagnostics for file %s", path)
		return model.File{}, fmt.Errorf("Failed to get diagnostics for file %s: %w", path, err)
	}

	file.Diagnostics = diagnostics
	return file, nil
}

func (pm ManagerImpl) DeleteFile(ctx context.Context, projectId, path string) error {
	log.Debug().Msgf("Deleting file %s in project %s", path, projectId)

	project, err := pm.GetProject(projectId)

	if err != nil {
		log.Error().Err(err).Msgf("Failed to get project with id %s", projectId)
		return fmt.Errorf("Failed to get project with id %s: %w", projectId, err)
	}

	return pm.fileManager.DeleteFile(model.NewContextWithProject(ctx, &project), afero.NewBasePathFs(afero.NewOsFs(), project.Path), path)
}

func (pm ManagerImpl) ListFiles(ctx context.Context, projectId string, showHidden bool) ([]model.File, error) {
	log.Debug().Msgf("Listing files in project %s", projectId)

	project, err := pm.GetProject(projectId)

	if err != nil {
		log.Error().Err(err).Msgf("Failed to get project with id %s", projectId)
		return nil, fmt.Errorf("Failed to get project with id %s: %w", projectId, err)
	}

	ctx = model.NewContextWithProject(ctx, &project)
	files, err := pm.fileManager.ListFiles(ctx, afero.NewBasePathFs(afero.NewOsFs(), project.Path), showHidden)

	if err != nil {
		log.Error().Err(err).Str("projectId", projectId).Msgf("Failed to list files in project %s", projectId)
		return nil, fmt.Errorf("Failed to list files in project %s: %w", projectId, err)
	}

	// wait for diagnostics
	time.Sleep(MaxDiagnosticsDelay)

	for _, file := range files {
		// TODO: it will take a long time because there's a timeout for each file; fix me!
		diagnostics, err := pm.getDiagnostics(ctx, file)

		if err != nil {
			log.Error().Err(err).Str("projectId", projectId).Msgf("Failed to get diagnostics for file %s", file.Path)
			return nil, fmt.Errorf("Failed to get diagnostics for file %s: %w", file.Path, err)
		}

		file.Diagnostics = diagnostics
	}

	return files, nil
}

func (pm ManagerImpl) ApplyPatch(ctx context.Context, projectId, path, patch string) (model.File, error) {
	log.Debug().Msgf("Applying patch %s to file %s in project %s", patch, path, projectId)

	project, err := pm.GetProject(projectId)

	if err != nil {
		log.Error().Err(err).Msgf("Failed to get project with id %s", projectId)
		return model.File{}, fmt.Errorf("Failed to get project with id %s: %w", projectId, err)
	}

	ctx = model.NewContextWithProject(ctx, &project)
	file, err := pm.fileManager.ApplyPatch(ctx, afero.NewBasePathFs(afero.NewOsFs(), project.Path), path, patch)

	if err != nil {
		log.Error().Err(err).Str("projectId", projectId).Msgf("Failed to apply patch %s to file %s", patch, path)
		return model.File{}, fmt.Errorf("Failed to apply patch %s to file %s: %w", patch, path, err)
	}

	// wait for diagnostics
	time.Sleep(MaxDiagnosticsDelay)
	diagnostics, err := pm.getDiagnostics(ctx, file)

	if err != nil {
		log.Error().Err(err).Str("projectId", projectId).Msgf("Failed to get diagnostics for file %s", path)
		return model.File{}, fmt.Errorf("Failed to get diagnostics for file %s: %w", path, err)
	}

	file.Diagnostics = diagnostics
	return file, nil
}

func (pm ManagerImpl) UpdateLines(ctx context.Context, projectId, path string, lineDiff files.LineDiffChunk) (model.File, error) {
	log.Debug().Msgf("Updating lines in file %s in project %s", path, projectId)

	project, err := pm.GetProject(projectId)

	if err != nil {
		log.Error().Err(err).Msgf("Failed to get project with id %s", projectId)
		return model.File{}, fmt.Errorf("Failed to get project with id %s: %w", projectId, err)
	}

	ctx = model.NewContextWithProject(ctx, &project)
	file, err := pm.fileManager.UpdateLines(ctx, afero.NewBasePathFs(afero.NewOsFs(), project.Path), path, lineDiff)

	if err != nil {
		log.Error().Err(err).Str("projectId", projectId).Msgf("Failed to update lines in file %s", path)
		return model.File{}, fmt.Errorf("Failed to update lines in file %s: %w", path, err)
	}

	// wait for diagnostics
	time.Sleep(MaxDiagnosticsDelay)
	diagnostics, err := pm.getDiagnostics(ctx, file)

	if err != nil {
		log.Error().Err(err).Str("projectId", projectId).Msgf("Failed to get diagnostics for file %s", path)
		return model.File{}, fmt.Errorf("Failed to get diagnostics for file %s: %w", path, err)
	}

	file.Diagnostics = diagnostics
	return file, nil
}

func (pm ManagerImpl) createProjectDir(path string) error {
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("Failed to create project directory: %w", err)
	}

	log.Debug().Msgf("Created project directory: %s", path)

	return nil
}

func (pm ManagerImpl) configFromProject(fileSystem fs.FS) (devcontainer.Config, error) {
	configFile, err := devcontainer.FindConfig(fileSystem)

	if err != nil {
		return devcontainer.Config{}, fmt.Errorf("Failed to find devcontainer.json: %w", err)
	}

	config, err := devcontainer.ParseConfig(configFile)

	if err != nil {
		return devcontainer.Config{}, fmt.Errorf("Failed to parse devcontainer.json: %w", err)
	}

	return *config, nil
}

func (pm ManagerImpl) getDiagnostics(ctx context.Context, file model.File) ([]protocol.Diagnostic, error) {
	// TODO: check if notification are required
	if err := pm.lspService.NotifyDidOpen(ctx, file); err != nil {
		return nil, fmt.Errorf("Failed to notify didOpen while reading file %s: %w", file.Path, err)
	}

	diagnostics, err := pm.lspService.GetDiagnostics(ctx, file)
	if err != nil {
		return nil, fmt.Errorf("Failed to get diagnostics for file %s: %w", file.Path, err)
	}

	// if err := pm.lspService.NotifyDidClose(ctx, file); err != nil {
	// 	return nil, fmt.Errorf("Failed to notify didClose while reading file %s: %w", file.Path, err)
	// }

	return diagnostics, nil
}

func removeProjectDir(projectPath string) {
	if err := os.RemoveAll(projectPath); err != nil {
		log.Error().Err(err).Msgf("Failed to remove project directory %s", projectPath)
		return
	}

	log.Debug().Msgf("Removed project directory: %s", projectPath)

	return
}

func cloneGitRepo(repository Repository, projectPath string) <-chan result.Empty {
	c := make(chan result.Empty)

	go func() {
		cmd := exec.Command("git", "clone", repository.Url, projectPath)
		cmdOut, err := cmd.Output()

		if err != nil {
			c <- result.EmptyFailure(fmt.Errorf("Failed to clone git repo: %w", err))
			return
		}

		log.Debug().Msgf("Cloned git repo %s to %s", repository.Url, projectPath)
		log.Debug().Msg(string(cmdOut))

		if repository.Commit != nil {
			cmd = exec.Command("git", "checkout", *repository.Commit)
			cmd.Dir = projectPath
			cmdOut, err = cmd.Output()

			if err != nil {
				c <- result.EmptyFailure(fmt.Errorf("Failed to checkout commit %s: %w", *repository.Commit, err))
				return
			}

			log.Debug().Msgf("Checked out commit %s", *repository.Commit)
			log.Debug().Msg(string(cmdOut))
		}

		c <- result.EmptySuccess()
	}()

	return c
}
