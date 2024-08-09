package model

import protocol "github.com/tliron/glsp/protocol_3_16"

type File struct {
	Path        string                `json:"path"`
	Content     string                `json:"content"`
	Diagnostics []protocol.Diagnostic `json:"diagnostics,omitempty"`
}

func (f *File) Equals(other *File) bool {
	if f == nil && other == nil {
		return true
	}

	if f == nil || other == nil {
		return false
	}

	// TODO: compare diagnostics?
	return f.Path == other.Path && f.Content == other.Content
}
