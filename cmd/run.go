package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/docker/docker/client"
	"github.com/go-playground/validator/v10"
	"github.com/hide-org/hide/pkg/devcontainer"
	"github.com/hide-org/hide/pkg/files"
	"github.com/hide-org/hide/pkg/git"
	"github.com/hide-org/hide/pkg/gitignore"
	"github.com/hide-org/hide/pkg/handlers"
	"github.com/hide-org/hide/pkg/lsp"
	"github.com/hide-org/hide/pkg/middleware"
	"github.com/hide-org/hide/pkg/model"
	"github.com/hide-org/hide/pkg/project"
	"github.com/hide-org/hide/pkg/random"
	"github.com/hide-org/hide/pkg/util"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

const (
	HidePath           = ".hide"
	ProjectsDir        = "projects"
	DefaultDotEnvPath  = ".env"
	RepositoryCacheDir = "repositories"
)

var (
	envPath string
	debug   bool
	port    int
)

func init() {
	pf := runCmd.PersistentFlags()
	pf.StringVar(&envPath, "env", DefaultDotEnvPath, "path to the .env file")
	pf.BoolVar(&debug, "debug", false, "run service in a debug mode")
	pf.IntVar(&port, "port", 8080, "service port")
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Runs Hide service",
	PreRun: func(cmd *cobra.Command, args []string) {
		setupLogger(debug)
	},
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Print(splash)

		_, err := os.Stat(envPath)

		if os.IsNotExist(err) {
			log.Debug().Msgf("Environment file %s does not exist.", envPath)
		}

		if err == nil {
			dir, file := filepath.Split(envPath)

			if dir == "" {
				dir = "."
			}

			err := util.LoadEnv(os.DirFS(dir), file)
			if err != nil {
				log.Error().Err(err).Msgf("Cannot load environment variables from %s", envPath)
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

		containerRunner := devcontainer.NewDockerRunner(devcontainer.NewExecutorImpl(), devcontainer.NewImageManager(dockerClient, random.String, devcontainer.NewDockerHubRegistryCredentials(dockerUser, dockerToken)), devcontainer.NewDockerContainerManager(dockerClient))
		projectStore := project.NewInMemoryStore(make(map[string]*model.Project))
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatal().Err(err).Msg("User's home directory is not set")
		}

		projectsDir := filepath.Join(home, HidePath, ProjectsDir)
		repositoryCacheDir := filepath.Join(home, HidePath, RepositoryCacheDir)

		fileManager := files.NewFileManager(gitignore.NewMatcherFactory())
		languageDetector := lsp.NewLanguageDetector()
		diagnosticsStore := lsp.NewDiagnosticsStore()
		clientPool := lsp.NewClientPool()
		lspService := lsp.NewService(languageDetector, lsp.LspServerExecutables, diagnosticsStore, clientPool)
		gitClient := git.NewClientImpl()
		gitCache := git.NewFsCache(afero.NewOsFs(), repositoryCacheDir, git.Clone)
		gitService := git.NewService(gitClient, gitCache)
		projectManager := project.NewProjectManager(containerRunner, projectStore, projectsDir, fileManager, lspService, languageDetector, random.String, gitService)
		validator := validator.New(validator.WithRequiredStructEnabled())

		router := handlers.
			NewRouter().
			WithCreateProjectHandler(handlers.CreateProjectHandler{Manager: projectManager, Validator: validator}).
			WithDeleteProjectHandler(handlers.DeleteProjectHandler{Manager: projectManager}).
			WithCreateTaskHandler(handlers.CreateTaskHandler{Manager: projectManager}).
			WithListTasksHandler(handlers.ListTasksHandler{Manager: projectManager}).
			WithCreateFileHandler(handlers.CreateFileHandler{ProjectManager: projectManager}).
			WithListFilesHandler(handlers.ListFilesHandler{ProjectManager: projectManager}).
			WithReadFileHandler(middleware.PathValidator(handlers.ReadFileHandler{ProjectManager: projectManager})).
			WithUpdateFileHandler(middleware.PathValidator(handlers.UpdateFileHandler{ProjectManager: projectManager})).
			WithDeleteFileHandler(middleware.PathValidator(handlers.DeleteFileHandler{ProjectManager: projectManager})).
			WithSearchFileHandler(handlers.SearchFilesHandler{ProjectManager: projectManager}).
			WithSearchSymbolsHandler(handlers.NewSearchSymbolsHandler(projectManager)).
			Build()

		addr := fmt.Sprintf("127.0.0.1:%d", port)

		server := &http.Server{
			Handler:      router,
			Addr:         addr,
			WriteTimeout: 120 * time.Second,
			ReadTimeout:  120 * time.Second,
		}

		log.Info().Msgf("Server started on %s\n", addr)

		go func() {
			sigChan := make(chan os.Signal, 1)
			signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
			<-sigChan

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()

			log.Info().Msg("Server shutting down ...")
			if err := projectManager.Cleanup(ctx); err != nil {
				log.Warn().Err(err).Msgf("Failed to cleanup projects")
			}

			if err := server.Shutdown(ctx); err != nil {
				log.Warn().Err(err).Msgf("HTTP shutdown error: %v", err)
			}
		}()

		if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Fatal().Err(err).Msgf("HTTP server error: %v", err)
		}

		fmt.Println("👋 Goodbye!")
	},
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
