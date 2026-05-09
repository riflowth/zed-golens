package lsp

import (
	"net/url"
	"strings"
)

type CodeLensParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

type CodeLens struct {
	Range   Range    `json:"range"`
	Command *Command `json:"command"`
}

type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

type Position struct {
	Line      uint `json:"line"`
	Character uint `json:"character"`
}

type Command struct {
	Title   string `json:"title"`
	Command string `json:"command"`
	Args    []any  `json:"arguments,omitempty"`
}

func URIToPath(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		return strings.TrimPrefix(uri, "file://")
	}
	return u.Path
}
