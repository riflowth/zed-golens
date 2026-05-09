package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Parse parses the content of a JSON-RPC message from r following the LSP specification
//
// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification/#baseProtocol
func Parse(r *bufio.Reader) ([]byte, error) {
	var contentLength int
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		if !strings.HasSuffix(line, "\r\n") {
			return nil, fmt.Errorf(`line ending must be \r\n`)
		}
		line = strings.TrimSuffix(line, "\r\n")
		if line == "" {
			break
		}
		if value, ok := strings.CutPrefix(line, "Content-Length: "); ok {
			contentLength, err = strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				return nil, fmt.Errorf("invalid Content-Length value: %w", err)
			}
		}
	}

	if contentLength == 0 {
		return nil, fmt.Errorf("no Content-Length header found")
	}

	buf := make([]byte, contentLength)
	_, err := io.ReadFull(r, buf)
	return buf, err
}

// Respond writes JSON data to w following the LSP specification
//
// See https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification/#baseProtocol
func Respond(w io.Writer, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Content-Length: %d\r\n\r\n", len(data)); err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	return nil
}
