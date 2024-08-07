package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/docker/docker/client"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/artmoskvin/hide/pkg/devcontainer"
	"github.com/artmoskvin/hide/pkg/files"
	"github.com/artmoskvin/hide/pkg/handlers"
	"github.com/artmoskvin/hide/pkg/model"
	"github.com/artmoskvin/hide/pkg/project"
	"github.com/artmoskvin/hide/pkg/util"
)

const (
	ProjectsDir       = "hide-projects"
	DefaultDotEnvPath = ".env"
)

func main() {
	fmt.Print(Splash)

	envPath := flag.String("env", DefaultDotEnvPath, "path to the .env file")
	debug := flag.Bool("debug", false, "run service in a debug mode")
	port := flag.Int("port", 8080, "service port")
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

	dockerClient, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatal().Err(err).Msg("Cannot initialize docker client")
	}

	ctx := context.Background()
	containerRunner := devcontainer.NewDockerRunner(dockerClient, util.NewExecutorImpl(), ctx, devcontainer.DockerRunnerConfig{Username: dockerUser, Password: dockerToken})
	projectStore := project.NewInMemoryStore(make(map[string]*model.Project))
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal().Err(err).Msg("User's home directory is not set")
	}

	projectsDir := filepath.Join(home, ProjectsDir)

	fileManager := files.NewFileManager()
	projectManager := project.NewProjectManager(containerRunner, projectStore, projectsDir, fileManager)
	createProjectHandler := handlers.CreateProjectHandler{Manager: projectManager}
	deleteProjectHandler := handlers.DeleteProjectHandler{Manager: projectManager}
	createTaskHandler := handlers.CreateTaskHandler{Manager: projectManager}
	listTasksHandler := handlers.ListTasksHandler{Manager: projectManager}
	createFileHandler := handlers.CreateFileHandler{ProjectManager: projectManager}
	readFileHandler := handlers.ReadFileHandler{ProjectManager: projectManager}
	updateFileHandler := handlers.UpdateFileHandler{ProjectManager: projectManager}
	deleteFileHandler := handlers.DeleteFileHandler{ProjectManager: projectManager}
	listFilesHandler := handlers.ListFilesHandler{ProjectManager: projectManager}

	router := mux.NewRouter()
	router.Handle("/projects", createProjectHandler).Methods("POST")
	router.Handle("/projects/{id}", deleteProjectHandler).Methods("DELETE")
	router.Handle("/projects/{id}/tasks", createTaskHandler).Methods("POST")
	router.Handle("/projects/{id}/tasks", listTasksHandler).Methods("GET")
	router.Handle("/projects/{id}/files", createFileHandler).Methods("POST")
	router.Handle("/projects/{id}/files", listFilesHandler).Methods("GET")
	router.Handle("/projects/{id}/files/{path:.*}", readFileHandler).Methods("GET")
	router.Handle("/projects/{id}/files/{path:.*}", updateFileHandler).Methods("PUT")
	router.Handle("/projects/{id}/files/{path:.*}", deleteFileHandler).Methods("DELETE")

	addr := fmt.Sprintf("127.0.0.1:%d", *port)

	server := &http.Server{
		Handler:      router,
		Addr:         addr,
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Info().Msgf("Server started on %s\n", addr)

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		log.Info().Msg("Server shutting down ...")
		projectManager.Cleanup()

		if err := server.Shutdown(ctx); err != nil {
			log.Warn().Err(err).Msgf("HTTP shutdown error: %v", err)
		}
	}()

	if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		log.Fatal().Err(err).Msgf("HTTP server error: %v", err)
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
