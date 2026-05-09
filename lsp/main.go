package main

import (
	"bufio"
	"encoding/json"
	"log"
	"os"

	"github.com/riflowth/zed-golens/lsp"
	"github.com/riflowth/zed-golens/rpc"
)

func main() {
	log.SetOutput(os.Stderr)
	r, w := bufio.NewReader(os.Stdin), os.Stdout

	for {
		body, err := lsp.Parse(r)
		if err != nil {
			log.Fatalf("failed to read jsonrpc2 request: %v", err)
		}

		var req rpc.Request
		if err := json.Unmarshal(body, &req); err != nil {
			log.Printf("failed to unmarshal jsonrpc2 request: %s\n", err)
			continue
		}

		switch req.Method {
		case "initialize":
			lsp.Respond(w, rpc.Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: map[string]any{
					"capabilities": map[string]any{
						// https://microsoft.github.io/language-server-protocol/specifications/lsp/3.17/specification/#textDocument_codeLens
						"codeLensProvider": map[string]any{
							"resolveProvider": false,
						},
					},
				},
			})
		case "textDocument/codeLens":
			var params lsp.CodeLensParams
			if err := json.Unmarshal(*req.Params, &params); err != nil {
				lsp.Respond(w, rpc.Response{
					JSONRPC: "2.0",
					ID:      req.ID,
					Error: &rpc.Error{
						Code:    rpc.CodeParseError,
						Message: err.Error(),
					},
				})
				continue
			}

			lenses, err := lsp.ListInterfaceLenses(params.TextDocument.URI)
			if err != nil {
				lsp.Respond(w, rpc.Response{
					JSONRPC: "2.0",
					ID:      req.ID,
					Error: &rpc.Error{
						Code:    rpc.CodeInternalError,
						Message: err.Error(),
					},
				})
				continue
			}

			lsp.Respond(w, rpc.Response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  lenses,
			})
		case "shutdown":
			lsp.Respond(w, rpc.Response{JSONRPC: "2.0", ID: req.ID})
		case "exit":
			os.Exit(0)
		}
	}
}
