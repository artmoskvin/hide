package lsp

import "fmt"

type LanguageNotSupportedError struct {
	LanguageId LanguageId
}

func (e LanguageNotSupportedError) Error() string {
	return fmt.Sprintf("Language %s is not supported", e.LanguageId)
}

type LanguageServerAlreadyExistsError struct {
	ProjectId  ProjectId
	LanguageId LanguageId
}

func (e LanguageServerAlreadyExistsError) Error() string {
	return fmt.Sprintf("Language server already exists for project %s and language %s", e.ProjectId, e.LanguageId)
}

type LanguageServerNotFoundError struct {
	ProjectId  ProjectId
	LanguageId LanguageId
}

func (e LanguageServerNotFoundError) Error() string {
	return fmt.Sprintf("Language server not found for project %s and language %s", e.ProjectId, e.LanguageId)
}
