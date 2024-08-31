package devcontainer

import (
	"context"
	"fmt"
	"os"

	"github.com/artmoskvin/hide/pkg/util"
	"github.com/rs/zerolog/log"
)

type ExecResult struct {
	StdOut   string
	StdErr   string
	ExitCode int
}

type Runner interface {
	Run(ctx context.Context, projectPath string, config Config) (string, error)
	Stop(ctx context.Context, containerId string) error
	Exec(ctx context.Context, containerId string, command []string) (ExecResult, error)
}

type DockerRunner struct {
	commandExecutor  util.Executor
	imageManager     ImageManager
	containerManager ContainerManager
}

func NewDockerRunner(commandExecutor util.Executor, imageManager ImageManager, containerManager ContainerManager) Runner {
	return &DockerRunner{
		commandExecutor:  commandExecutor,
		imageManager:     imageManager,
		containerManager: containerManager,
	}
}

func (r *DockerRunner) Run(ctx context.Context, projectPath string, config Config) (string, error) {
	log.Debug().Any("config", config).Msg("Running container")
	// Run initialize commands
	if command := config.LifecycleProps.InitializeCommand; command != nil {
		if err := r.executeLifecycleCommand(command, projectPath); err != nil {
			return "", fmt.Errorf("Failed to run initialize command %s: %w", command, err)
		}
	}

	// Pull or build image
	var imageId string
	var err error

	switch {
	case config.IsImageDevContainer():
		imageId = config.DockerImageProps.Image
		err = r.imageManager.PullImage(ctx, config.DockerImageProps.Image)
		if err != nil {
			err = fmt.Errorf("Failed to pull image: %w", err)
		}
	case config.IsDockerfileDevContainer():
		imageId, err = r.imageManager.BuildImage(ctx, projectPath, config)
		if err != nil {
			err = fmt.Errorf("Failed to build image: %w", err)
		}
	case config.IsComposeDevContainer():
		// TODO: build docker-compose file
		err = fmt.Errorf("Docker Compose is not supported yet")
	default:
		err = fmt.Errorf("Invalid devcontainer configuration")
	}

	if err != nil {
		return "", fmt.Errorf("Failed to pull or build image: %w", err)
	}

	// Create container
	containerId, err := r.containerManager.CreateContainer(ctx, imageId, projectPath, config)

	if err != nil {
		return "", fmt.Errorf("Failed to create container: %w", err)
	}

	// Start container
	if err := r.containerManager.StartContainer(ctx, containerId); err != nil {
		return "", fmt.Errorf("Failed to start container: %w", err)
	}

	// Run onCreate commands
	if command := config.LifecycleProps.OnCreateCommand; command != nil {
		if err := r.executeLifecycleCommandInContainer(ctx, command, containerId); err != nil {
			return "", fmt.Errorf("Failed to run onCreate command %s: %w", command, err)
		}
	}

	// Run updateContent commands
	if command := config.LifecycleProps.UpdateContentCommand; command != nil {
		if err := r.executeLifecycleCommandInContainer(ctx, command, containerId); err != nil {
			return "", fmt.Errorf("Failed to run updateContent command %s: %w", command, err)
		}
	}

	// Run postCreate commands
	if command := config.LifecycleProps.PostCreateCommand; command != nil {
		if err := r.executeLifecycleCommandInContainer(ctx, command, containerId); err != nil {
			return "", fmt.Errorf("Failed to run postCreate command %s: %w", command, err)
		}
	}

	// Run postStart commands
	if command := config.LifecycleProps.PostStartCommand; command != nil {
		if err := r.executeLifecycleCommand(command, projectPath); err != nil {
			return "", fmt.Errorf("Failed to run postStart command %s: %w", command, err)
		}
	}

	// Run postAttach commands
	if command := config.LifecycleProps.PostAttachCommand; command != nil {
		if err := r.executeLifecycleCommand(command, projectPath); err != nil {
			return "", fmt.Errorf("Failed to run postAttach command %s: %w", command, err)
		}
	}

	return containerId, nil
}

func (r *DockerRunner) Stop(ctx context.Context, containerId string) error {
	return r.containerManager.StopContainer(ctx, containerId)
}

func (r *DockerRunner) Exec(ctx context.Context, containerID string, command []string) (ExecResult, error) {
	return r.containerManager.Exec(ctx, containerID, command)
}

func (r *DockerRunner) executeLifecycleCommand(lifecycleCommand LifecycleCommand, workingDir string) error {
	for _, command := range lifecycleCommand {
		log.Debug().Str("command", fmt.Sprintf("%s", command)).Msg("Running command")

		if err := r.commandExecutor.Run(command, workingDir, os.Stdout, os.Stderr); err != nil {
			return err
		}
	}

	return nil
}

func (r *DockerRunner) executeLifecycleCommandInContainer(ctx context.Context, lifecycleCommand LifecycleCommand, containerId string) error {
	for _, command := range lifecycleCommand {
		log.Debug().Str("command", fmt.Sprintf("%s", command)).Msg("Running command")

		result, err := r.Exec(ctx, containerId, command)

		if err != nil {
			return err
		}

		if result.ExitCode != 0 {
			return fmt.Errorf("Exit code %d. Stdout: %s, Stderr: %s", result.ExitCode, result.StdOut, result.StdErr)
		}

	}

	return nil
}
