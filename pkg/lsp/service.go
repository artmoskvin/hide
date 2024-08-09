package lsp

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/artmoskvin/hide/pkg/model"
	"github.com/rs/zerolog/log"
	protocol "github.com/tliron/glsp/protocol_3_16"
)

type ProjectId = string
type LanguageId = string
type ProjectRoot = string
type LspClientStore = map[ProjectId]map[LanguageId]Client
type LspDiagnostics = map[ProjectId]map[protocol.DocumentUri][]protocol.Diagnostic

type Service interface {
	StartServer(ctx context.Context, languageId LanguageId) error
	StopServer(ctx context.Context, languageId LanguageId) error
	NotifyDidOpen(ctx context.Context, file model.File) error
	NotifyDidClose(ctx context.Context, file model.File) error
	// TODO: check if any LSP server supports this
	// PullDiagnostics(ctx context.Context, params DocumentDiagnosticParams) (DocumentDiagnosticReport, error)
	GetDiagnostics(ctx context.Context, file model.File) ([]protocol.Diagnostic, error)
	Cleanup(ctx context.Context) error
	CleanupProject(ctx context.Context, projectId ProjectId) error
}

type ServiceImpl struct {
	languageDetector     LanguageDetector
	lspClients           LspClientStore
	lspDiagnostics       LspDiagnostics
	lspServerExecutables map[LanguageId]string
}

// StartServer implements Service.
func (s *ServiceImpl) StartServer(ctx context.Context, languageId LanguageId) error {
	project, ok := model.ProjectFromContext(ctx)
	if !ok {
		log.Error().Msg("Project not found in context")
		return fmt.Errorf("Project not found in context")
	}

	projectId := project.Id

	executable, ok := s.lspServerExecutables[languageId]
	if !ok {
		return LanguageNotSupportedError{LanguageId: languageId}
	}

	// Start the language server
	process, err := NewProcess(executable)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create language server process")
		return fmt.Errorf("Failed to create language server process: %w", err)
	}

	if err := process.Start(); err != nil {
		log.Error().Err(err).Msg("Failed to start language server")
		return fmt.Errorf("Failed to start language server: %w", err)
	}

	// Create a channel for diagnostics
	diagnosticsChannel := make(chan protocol.PublishDiagnosticsParams)

	// Create a client for the language server
	client := NewClient(process, diagnosticsChannel)

	// Initialize the language server
	root := PathToURI(project.Path)
	initResult, err := client.Initialize(ctx, protocol.InitializeParams{
		RootURI: &root,
		Capabilities: protocol.ClientCapabilities{
			TextDocument: &protocol.TextDocumentClientCapabilities{
				Synchronization: &protocol.TextDocumentSyncClientCapabilities{
					DynamicRegistration: boolPointer(true),
				},
			},
		},
		// WorkspaceFolders: []protocol.WorkspaceFolder{
		// 	{
		// 		URI:  root,
		// 		Name: "hide",
		// 	},
		// },
	})

	if err != nil {
		log.Error().Str("languageId", languageId).Str("projectId", projectId).Err(err).Msg("Failed to initialize language server")
		return fmt.Errorf("Failed to initialize language server: %w", err)
	}

	log.Debug().Str("languageId", languageId).Str("projectId", projectId).Msg("Initialized language server")

	// Check capabilities
	if opt, ok := initResult.Capabilities.TextDocumentSync.(protocol.TextDocumentSyncOptions); ok {
		log.Debug().Str("languageId", languageId).Str("projectId", projectId).Msgf("LSP server supports open/close file: %t", *opt.OpenClose)
		log.Debug().Str("languageId", languageId).Str("projectId", projectId).Msgf("LSP server supports change notifications: %v", *opt.Change)
	}

	// Notify that initialized
	if err := client.NotifyInitialized(ctx); err != nil {
		log.Error().Err(err).Str("languageId", languageId).Str("projectId", projectId).Msg("Failed to notify initialized")
		return fmt.Errorf("Failed to notify initialized: %w", err)
	}

	if clients, ok := s.lspClients[projectId]; ok {
		clients[languageId] = client
	} else {
		s.lspClients[projectId] = make(map[LanguageId]Client)
		s.lspClients[projectId][languageId] = client
	}

	go s.listenForDiagnostics(projectId, diagnosticsChannel)
	return nil
}

func (s *ServiceImpl) StopServer(ctx context.Context, languageId LanguageId) error {
	project, ok := model.ProjectFromContext(ctx)
	if !ok {
		log.Error().Msg("Project not found in context")
		return fmt.Errorf("Project not found in context")
	}

	client, ok := s.getClient(ctx, languageId)

	if !ok {
		log.Warn().Str("languageId", languageId).Str("projectId", project.Id).Msg("LSP client not found")
		return nil
	}

	if err := client.Shutdown(ctx); err != nil {
		log.Error().Err(err).Str("languageId", languageId).Str("projectId", project.Id).Msg("Failed to stop language server")
		return fmt.Errorf("Failed to stop language server: %w", err)
	}

	delete(s.lspClients[project.Id], languageId)

	return nil
}

// NotifyDidClose implements Service.
func (s *ServiceImpl) NotifyDidClose(ctx context.Context, file model.File) error {
	project, ok := model.ProjectFromContext(ctx)

	if !ok {
		log.Error().Msg("Project not found in context")
		return fmt.Errorf("Project not found in context")
	}

	languageId := s.languageDetector.DetectLanguage(file)
	client, ok := s.getClient(ctx, languageId)

	if !ok {
		log.Error().Str("languageId", languageId).Str("projectId", project.Id).Msg("LSP client not found")
		return LanguageServerNotFoundError{ProjectId: project.Id, LanguageId: languageId}
	}

	fullPath := filepath.Join(project.Path, file.Path)

	err := client.NotifyDidClose(ctx, protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{
			URI: PathToURI(fullPath),
		},
	})

	return err
}

// NotifyDidOpen implements Service.
func (s *ServiceImpl) NotifyDidOpen(ctx context.Context, file model.File) error {
	project, ok := model.ProjectFromContext(ctx)

	if !ok {
		log.Error().Msg("Project not found in context")
		return fmt.Errorf("Project not found in context")
	}

	languageId := s.languageDetector.DetectLanguage(file)
	client, ok := s.getClient(ctx, languageId)

	if !ok {
		log.Error().Str("languageId", languageId).Str("projectId", project.Id).Msg("LSP client not found")
		return LanguageServerNotFoundError{ProjectId: project.Id, LanguageId: languageId}
	}

	fullPath := filepath.Join(project.Path, file.Path)

	err := client.NotifyDidOpen(ctx, protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        PathToURI(fullPath),
			LanguageID: languageId,
			Version:    1,
			Text:       file.Content,
		},
	})

	return err
}

func (s *ServiceImpl) GetDiagnostics(ctx context.Context, file model.File) ([]protocol.Diagnostic, error) {
	project, ok := model.ProjectFromContext(ctx)

	if !ok {
		log.Error().Msg("Project not found in context")
		return nil, fmt.Errorf("Project not found in context")
	}

	uri := PathToURI(filepath.Join(project.Path, file.Path))
	if diagnostics, ok := s.lspDiagnostics[project.Id]; ok {
		return diagnostics[uri], nil
	}

	return nil, nil
}

func (s *ServiceImpl) Cleanup(ctx context.Context) error {
	for projectId := range s.lspClients {
		if err := s.CleanupProject(ctx, projectId); err != nil {
			return err
		}
	}

	return nil
}

func (s *ServiceImpl) CleanupProject(ctx context.Context, projectId ProjectId) error {
	clients, ok := s.lspClients[projectId]
	if !ok {
		return nil
	}

	for _, client := range clients {
		if err := client.Shutdown(ctx); err != nil {
			return err
		}
	}

	delete(s.lspClients, projectId)
	return nil
}

func (s *ServiceImpl) getClient(ctx context.Context, languageId LanguageId) (Client, bool) {
	project, ok := model.ProjectFromContext(ctx)
	if !ok {
		log.Error().Msg("Project not found in context")
		return nil, false
	}

	clients, ok := s.lspClients[project.Id]
	if !ok {
		return nil, false
	}

	client, ok := clients[languageId]
	if !ok {
		return nil, false
	}

	return client, true
}

func (s *ServiceImpl) listenForDiagnostics(projectId ProjectId, channel chan protocol.PublishDiagnosticsParams) {
	for {
		select {
		case diagnostics := <-channel:
			log.Debug().Str("projectId", projectId).Str("uri", diagnostics.URI).Msg("Received diagnostics")
			log.Debug().Str("projectId", projectId).Str("uri", diagnostics.URI).Msgf("Diagnostics: %+v", diagnostics.Diagnostics)

			s.updateDiagnostics(projectId, diagnostics)
		}
	}
}

func (s *ServiceImpl) updateDiagnostics(projectId ProjectId, diagnostics protocol.PublishDiagnosticsParams) {
	if _, ok := s.lspDiagnostics[projectId]; !ok {
		s.lspDiagnostics[projectId] = make(map[protocol.DocumentUri][]protocol.Diagnostic)
	}

	s.lspDiagnostics[projectId][diagnostics.URI] = diagnostics.Diagnostics
}

func PathToURI(path string) protocol.DocumentUri {
	return protocol.DocumentUri("file://" + path)
}

func NewService(languageDetector LanguageDetector, lspServerExecutables map[LanguageId]string) Service {
	return &ServiceImpl{
		languageDetector:     languageDetector,
		lspClients:           make(LspClientStore),
		lspDiagnostics:       make(LspDiagnostics),
		lspServerExecutables: lspServerExecutables,
	}
}

func boolPointer(b bool) *bool {
	return &b
}
