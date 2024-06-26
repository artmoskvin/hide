package devcontainer

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strconv"

	"log"
	"os"

	"github.com/artmoskvin/hide/pkg/util"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/go-connections/nat"

	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
)

const DefaultShell = "/bin/sh"

var DefaultContainerCommand = []string{DefaultShell, "-c", "while sleep 1000; do :; done"}

type DockerRunnerConfig struct {
	Username string
	Password string
}

type ExecResult struct {
	StdOut   string
	StdErr   string
	ExitCode int
}

type Runner interface {
	Run(projectPath string, config Config) (string, error)
	Stop(containerId string) error
	Exec(containerId string, command []string) (ExecResult, error)
}

type DockerRunner struct {
	dockerClient    *client.Client
	commandExecutor util.Executor
	context         context.Context
	config          DockerRunnerConfig
}

func NewDockerRunner(client *client.Client, commandExecutor util.Executor, context context.Context, config DockerRunnerConfig) Runner {
	return &DockerRunner{
		dockerClient:    client,
		commandExecutor: commandExecutor,
		context:         context,
		config:          config,
	}
}

func (r *DockerRunner) Run(projectPath string, config Config) (string, error) {
	// Run initialize commands
	if command := config.LifecycleProps.InitializeCommand; command != nil {
		if err := r.executeLifecycleCommand(command, projectPath); err != nil {
			return "", fmt.Errorf("Failed to run initialize command %s: %w", command, err)
		}
	}

	// Build docker compose
	if len(config.DockerComposeFile) > 0 {
		// TODO: build docker-compose file
		return "", fmt.Errorf("Docker Compose is not supported yet")
	}

	// Pull or build image
	imageId, err := r.pullOrBuildImage(projectPath, config)

	if err != nil {
		return "", fmt.Errorf("Failed to pull or build image: %w", err)
	}

	// Create container
	containerId, err := r.createContainer(imageId, projectPath, config)

	if err != nil {
		return "", fmt.Errorf("Failed to create container: %w", err)
	}

	// Start container
	if err := r.startContainer(containerId); err != nil {
		return "", fmt.Errorf("Failed to start container: %w", err)
	}

	// Run onCreate commands
	if command := config.LifecycleProps.OnCreateCommand; command != nil {
		if err := r.executeLifecycleCommandInContainer(command, containerId); err != nil {
			return "", fmt.Errorf("Failed to run onCreate command %s: %w", command, err)
		}
	}

	// Run updateContent commands
	if command := config.LifecycleProps.UpdateContentCommand; command != nil {
		if err := r.executeLifecycleCommandInContainer(command, containerId); err != nil {
			return "", fmt.Errorf("Failed to run updateContent command %s: %w", command, err)
		}
	}

	// Run postCreate commands
	if command := config.LifecycleProps.PostCreateCommand; command != nil {
		if err := r.executeLifecycleCommandInContainer(command, containerId); err != nil {
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

func (r *DockerRunner) Stop(containerId string) error {
	if err := r.dockerClient.ContainerStop(context.Background(), containerId, container.StopOptions{}); err != nil {
		return fmt.Errorf("Failed to stop container %s: %w", containerId, err)
	}

	return nil
}

func (r *DockerRunner) Exec(containerID string, command []string) (ExecResult, error) {
	execConfig := types.ExecConfig{
		Cmd:          command,
		AttachStdout: true,
		AttachStderr: true,
	}
	execIDResp, err := r.dockerClient.ContainerExecCreate(r.context, containerID, execConfig)

	if err != nil {
		return ExecResult{}, fmt.Errorf("Failed to create execute configuration for command %s in container %s: %w", command, containerID, err)
	}

	execID := execIDResp.ID

	resp, err := r.dockerClient.ContainerExecAttach(r.context, execID, types.ExecStartCheck{})

	if err != nil {
		return ExecResult{}, fmt.Errorf("Failed to attach to exec process %s in container %s: %w", execID, containerID, err)
	}

	defer resp.Close()

	var stdOut, stdErr bytes.Buffer

	stdOutWriter := io.MultiWriter(os.Stdout, &stdOut)
	stdErrWriter := io.MultiWriter(os.Stderr, &stdErr)

	if err := ReadOutputFromContainer(resp.Reader, stdOutWriter, stdErrWriter); err != nil {
		return ExecResult{}, fmt.Errorf("Error reading output from container: %w", err)
	}

	inspectResp, err := r.dockerClient.ContainerExecInspect(context.Background(), execID)

	if err != nil {
		return ExecResult{}, fmt.Errorf("Failed to inspect exec process %s in container %s: %w", execID, containerID, err)
	}

	return ExecResult{StdOut: stdOut.String(), StdErr: stdErr.String(), ExitCode: inspectResp.ExitCode}, nil
}

func (r *DockerRunner) executeLifecycleCommand(lifecycleCommand LifecycleCommand, workingDir string) error {
	for _, command := range lifecycleCommand {
		log.Printf("Running command '%s'", command)

		if err := r.commandExecutor.Run(command, workingDir, os.Stdout, os.Stderr); err != nil {
			return err
		}
	}

	return nil
}

func (r *DockerRunner) executeLifecycleCommandInContainer(lifecycleCommand LifecycleCommand, containerId string) error {
	for _, command := range lifecycleCommand {
		log.Printf("Running command %s in container %s", command, containerId)

		result, err := r.Exec(containerId, command)

		if err != nil {
			return err
		}

		if result.ExitCode != 0 {
			return fmt.Errorf("Exit code %d. Stdout: %s, Stderr: %s", result.ExitCode, result.StdOut, result.StdErr)
		}

	}

	return nil
}

func (r *DockerRunner) pullOrBuildImage(workingDir string, config Config) (string, error) {
	if config.Image != "" {
		if err := r.pullImage(config.Image); err != nil {
			return "", fmt.Errorf("Failed to pull image %s: %w", config.Image, err)
		}

		return config.Image, nil
	}

	var dockerFile, context string

	if config.Dockerfile != "" {
		dockerFile = config.Dockerfile
	} else if config.Build != nil && config.Build.Dockerfile != "" {
		dockerFile = config.Build.Dockerfile
	} else {
		return "", fmt.Errorf("Dockerfile not found")
	}

	if config.Context != "" {
		context = config.Context
	} else if config.Build != nil && config.Build.Context != "" {
		context = config.Build.Context
	} else {
		// default value
		// NOTE: this is bad; default values should be set during parsing
		context = "."
	}

	dockerFilePath := filepath.Join(workingDir, config.Path, dockerFile)
	contextPath := filepath.Join(workingDir, config.Path, context)
	imageId, err := r.buildImage(contextPath, dockerFilePath, config.Build, config.Name)

	if err != nil {
		return "", fmt.Errorf("Failed to build image: %w", err)
	}

	return imageId, nil

}

func (r *DockerRunner) pullImage(_image string) error {
	log.Println("Pulling image", _image)

	authStr, err := r.encodeRegistryAuth(r.config.Username, r.config.Password)

	if err != nil {
		log.Printf("Failed to encode registry auth: %s", err)
		return fmt.Errorf("Failed to encode registry auth: %w", err)
	}

	output, err := r.dockerClient.ImagePull(r.context, _image, image.PullOptions{RegistryAuth: authStr})

	if err != nil {
		return err
	}

	defer output.Close()

	if err := util.ReadOutput(output, os.Stdout); err != nil {
		log.Printf("Error streaming output: %v\n", err)
	}

	log.Println("Pulled image", _image)

	return nil
}

func (r *DockerRunner) buildImage(buildContextPath string, dockerFilePath string, buildProps *BuildProps, containerName string) (string, error) {
	log.Println("Building image from", buildContextPath)

	log.Println("Warning: building images is not stable yet")

	buildContext, err := archive.TarWithOptions(buildContextPath, &archive.TarOptions{})

	if err != nil {
		return "", fmt.Errorf("Failed to create tar archive from %s for Docker build context: %w", buildContextPath, err)
	}

	var tag string

	if containerName != "" {
		tag = fmt.Sprintf("%s-%s:%s", containerName, util.RandomString(6), "latest")
	} else {
		tag = fmt.Sprintf("%s:%s", util.RandomString(6), "latest")
	}

	imageBuildResponse, err := r.dockerClient.ImageBuild(r.context, buildContext, types.ImageBuildOptions{
		Tags:       []string{tag},
		Dockerfile: dockerFilePath,
		BuildArgs:  buildProps.Args,
		Context:    buildContext,
		CacheFrom:  buildProps.CacheFrom,
		Target:     buildProps.Target,
		// NOTE: other options are ignored because in the devcontainer spec they are defined as []string and it's too cumbersome to parse them into types.ImageBuildOptions{}
	})

	if err != nil {
		return "", fmt.Errorf("Failed to build Docker image: %w", err)
	}

	defer imageBuildResponse.Body.Close()

	if err := util.ReadOutput(imageBuildResponse.Body, os.Stdout); err != nil {
		log.Printf("Error streaming output: %v\n", err)
	}

	log.Println("Built image with tag", tag)

	return tag, nil
}

func (r *DockerRunner) createContainer(image string, projectPath string, config Config) (string, error) {
	log.Println("Creating container...")

	env := []string{}

	for envKey, envValue := range config.ContainerEnv {
		env = append(env, fmt.Sprintf("%s=%s", envKey, envValue))
	}

	containerConfig := &container.Config{Image: image, Cmd: DefaultContainerCommand, Env: env}

	if config.ContainerUser != "" {
		containerConfig.User = config.ContainerUser
	}

	portBindings := make(nat.PortMap)

	for _, port := range config.AppPort {
		port_str := strconv.Itoa(port)
		port, err := nat.NewPort("tcp", port_str)

		if err != nil {
			return "", fmt.Errorf("Failed to create new TCP port from port %s: %w", port_str, err)
		}

		portBindings[port] = []nat.PortBinding{{HostIP: "127.0.0.1", HostPort: port_str}}
	}

	mounts := []mount.Mount{}

	if config.WorkspaceMount != nil && config.WorkspaceFolder != "" {
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeBind,
			Source: config.WorkspaceMount.Source,
			Target: config.WorkspaceMount.Destination,
		})

		containerConfig.WorkingDir = config.WorkspaceFolder
	} else {
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeBind,
			Source: projectPath,
			Target: "/workspace",
		})

		containerConfig.WorkingDir = "/workspace"
	}

	if len(config.Mounts) > 0 {
		for _, _mount := range config.Mounts {
			mountType, err := stringToType(_mount.Type)

			if err != nil {
				return "", fmt.Errorf("Failed to convert mount type %s to type.Type: %w", _mount.Type, err)
			}

			mounts = append(mounts, mount.Mount{
				Type:   mountType,
				Source: _mount.Source,
				Target: _mount.Destination,
			})
		}
	}

	hostConfig := container.HostConfig{
		PortBindings: portBindings,
		Mounts:       mounts,
		Init:         &config.Init,
		Privileged:   config.Privileged,
		CapAdd:       config.CapAdd,
		SecurityOpt:  config.SecurityOpt,
	}

	createResponse, err := r.dockerClient.ContainerCreate(r.context, containerConfig, &hostConfig, nil, nil, "")

	if err != nil {
		return "", err
	}

	containerId := createResponse.ID

	log.Println("Created container", containerId)

	return containerId, nil
}

func (r *DockerRunner) startContainer(containerId string) error {
	log.Println("Starting container...")

	err := r.dockerClient.ContainerStart(r.context, containerId, container.StartOptions{})

	if err != nil {
		return err
	}

	log.Println("Started container", containerId)

	return nil
}

func (r *DockerRunner) encodeRegistryAuth(username, password string) (string, error) {
	authConfig := registry.AuthConfig{
		Username: username,
		Password: password,
	}

	encodedJSON, err := json.Marshal(authConfig)

	if err != nil {
		return "", err
	}

	authStr := base64.URLEncoding.EncodeToString(encodedJSON)

	return authStr, nil
}

func stringToType(s string) (mount.Type, error) {
	switch s {
	case string(mount.TypeBind):
		return mount.TypeBind, nil
	case string(mount.TypeVolume):
		return mount.TypeVolume, nil
	case string(mount.TypeTmpfs):
		return mount.TypeTmpfs, nil
	case string(mount.TypeNamedPipe):
		return mount.TypeNamedPipe, nil
	case string(mount.TypeCluster):
		return mount.TypeCluster, nil
	default:
		return "", fmt.Errorf("Unsupported mount type: %s", s)
	}
}
