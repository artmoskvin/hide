package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/artmoskvin/hide/pkg/devcontainer"
	"github.com/artmoskvin/hide/pkg/files"
	"github.com/artmoskvin/hide/pkg/handlers"
	"github.com/artmoskvin/hide/pkg/lsp"
	"github.com/artmoskvin/hide/pkg/model"
	"github.com/artmoskvin/hide/pkg/project"
	"github.com/artmoskvin/hide/pkg/util"
	"github.com/docker/docker/client"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const (
	ProjectsDir       = "hide-projects"
	DefaultDotEnvPath = ".env"
)

func main() {
	fmt.Print(Splash)

	envPath := flag.String("env", DefaultDotEnvPath, "path to the .env file")
	debug := flag.Bool("debug", false, "run service in a debug mode")
	flag.Parse()

	setupLogger(*debug)

	_, err := os.Stat(*envPath)

	if os.IsNotExist(err) {
		log.Debug().Msgf("Environment file %s does not exist.", *envPath)
	}

	if err == nil {
		dir, file := filepath.Split(*envPath)

		if dir == "" {
			dir = "."
		}

		err := util.LoadEnv(os.DirFS(dir), file)
		if err != nil {
			log.Error().Err(err).Msgf("Cannot load environment variables from %s", *envPath)
		}
	}

	dockerUser := os.Getenv("DOCKER_USER")
	dockerToken := os.Getenv("DOCKER_TOKEN")

	if dockerUser == "" || dockerToken == "" {
		log.Warn().Msg("DOCKER_USER or DOCKER_TOKEN environment variables are empty. This might cause problems when pulling images from Docker Hub.")
	}

	mux := http.NewServeMux()
	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatal().Err(err).Msg("Cannot initialize docker client")
	}

	context := context.Background()
	containerRunner := devcontainer.NewDockerRunner(dockerClient, util.NewExecutorImpl(), context, devcontainer.DockerRunnerConfig{Username: dockerUser, Password: dockerToken})
	projectStore := project.NewInMemoryStore(make(map[string]*model.Project))
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal().Err(err).Msg("User's home directory is not set")
	}

	projectsDir := filepath.Join(home, ProjectsDir)

	lspServerExecutables := make(map[lsp.LanguageId]string)
	lspServerExecutables[lsp.LanguageId("go")] = "gopls"

	fileManager := files.NewFileManager()
	languageDetector := lsp.NewFileExtensionBasedLanguageDetector()
	lspService := lsp.NewService(languageDetector, lspServerExecutables)
	projectManager := project.NewProjectManager(containerRunner, projectStore, projectsDir, fileManager, lspService, languageDetector)
	createProjectHandler := handlers.CreateProjectHandler{Manager: projectManager}
	deleteProjectHandler := handlers.DeleteProjectHandler{Manager: projectManager}
	createTaskHandler := handlers.CreateTaskHandler{Manager: projectManager}
	listTasksHandler := handlers.ListTasksHandler{Manager: projectManager}
	createFileHandler := handlers.CreateFileHandler{ProjectManager: projectManager}
	readFileHandler := handlers.ReadFileHandler{ProjectManager: projectManager}
	updateFileHandler := handlers.UpdateFileHandler{ProjectManager: projectManager}
	deleteFileHandler := handlers.DeleteFileHandler{ProjectManager: projectManager}
	listFilesHandler := handlers.ListFilesHandler{ProjectManager: projectManager}

	mux.Handle("POST /projects", createProjectHandler)
	mux.Handle("DELETE /projects/{id}", deleteProjectHandler)
	mux.Handle("POST /projects/{id}/tasks", createTaskHandler)
	mux.Handle("GET /projects/{id}/tasks", listTasksHandler)
	mux.Handle("POST /projects/{id}/files", createFileHandler)
	mux.Handle("GET /projects/{id}/files", listFilesHandler)
	mux.Handle("GET /projects/{id}/files/{path...}", readFileHandler)
	mux.Handle("PUT /projects/{id}/files/{path...}", updateFileHandler)
	mux.Handle("DELETE /projects/{id}/files/{path...}", deleteFileHandler)

	// TODO: make configurable
	port := ":8080"

	log.Info().Msgf("Server started on %s\n", port)

	if err := http.ListenAndServe(port, mux); err != nil {
		log.Fatal().Err(err).Str("port", port).Msg("Failed to start server")
	}
}

func setupLogger(debug bool) {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs
	output := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339Nano}
	log.Logger = log.Output(output).With().Caller().Logger()

	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}
}
