package lsp

import (
	"path/filepath"

	"github.com/artmoskvin/hide/pkg/model"
)

type LanguageDetector interface {
	DetectLanguage(file model.File) string
	DetectLanguages(files []model.File) map[string]int
	DetectMainLanguage(files []model.File) string
}

// Naive implementation that detects the language based on the file extension
type FileExtensionBasedLanguageDetector struct{}

func NewFileExtensionBasedLanguageDetector() LanguageDetector {
	return &FileExtensionBasedLanguageDetector{}
}

// Return the language id as per https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification/#textDocumentItem
func (ld FileExtensionBasedLanguageDetector) DetectLanguage(file model.File) LanguageId {
	// TODO: implement
	extension := filepath.Ext(file.Path)
	switch extension {
	case ".c":
		return "c"
	case ".cpp":
		return "cpp"
	case ".cs":
		return "csharp"
	case ".css":
		return "css"
	case ".go":
		return "go"
	case ".html":
		return "html"
	case ".java":
		return "java"
	case ".js":
		return "javascript"
	case ".jsx":
		return "javascriptreact"
	case ".json":
		return "json"
	case ".lua":
		return "lua"
	case ".php":
		return "php"
	case ".py":
		return "python"
	case ".rb":
		return "ruby"
	case ".rs":
		return "rust"
	case ".scala":
		return "scala"
	case ".sh":
		return "shellscript"
	case ".sql":
		return "sql"
	case ".swift":
		return "swift"
	case ".ts":
		return "typescript"
	case ".tsx":
		return "typescriptreact"
	case ".xml":
		return "xml"
	case ".yaml":
		return "yaml"
	default:
		return "plaintext"
	}
}

func (ld FileExtensionBasedLanguageDetector) DetectLanguages(files []model.File) map[string]int {
	languages := make(map[string]int)
	for _, file := range files {
		language := ld.DetectLanguage(file)
		languages[language]++
	}
	return languages
}

func (ld FileExtensionBasedLanguageDetector) DetectMainLanguage(files []model.File) string {
	languages := ld.DetectLanguages(files)
	var maxLanguage string
	maxCount := 0
	for language, count := range languages {
		if count > maxCount {
			maxLanguage = language
			maxCount = count
		}
	}
	return maxLanguage
}
