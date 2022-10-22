// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

// Runs a Language Server Protocol (LSP) server ready to comminicate with any editor/IDE.
//
// This command is not intended to be invoked by end-users directly. Instead respective editor plugins
// will manage the instances of LSP server as needed.
package lsp

import (
	"context"
	"io"
	"os"

	"github.com/spf13/cobra"
	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.uber.org/zap"
	"namespacelabs.dev/foundation/internal/cli/fncobra"
)

func NewLSPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "lsp",
		Short:  "Run the Language Server.",
		Hidden: true,
		Args:   cobra.NoArgs,

		RunE: fncobra.RunE(func(ctx context.Context, args []string) error {
			pipe := stdioReadWriteCloser()
			stream := jsonrpc2.NewStream(pipe)
			conn := jsonrpc2.NewConn(stream)
			client := protocol.ClientDispatcher(conn, zap.New(nil))
			srv := newServer(ctx, conn, client)
			conn.Go(ctx, goroutineHandler(protocol.ServerHandler(srv, nil)))
			<-conn.Done()
			return nil
		}),
	}
	// Passed by languageclient-vscode.
	cmd.Flags().Bool("stdio", true, "Use stdio for JSON-RPC communication (the only option as of now).")

	return cmd
}

// Reuturns a wrapped JSON-RPC handler that runs its delegate in a new goroutine.
// This works around the issue with [go.lsp.dev/protocol] where server->client requests
// are blocked indefinitely since the RPC connection is blocked waiting on the response
// to the client->server request.
// TODO: Make this the default behavior of go.lsp.dev/protocol.
func goroutineHandler(h jsonrpc2.Handler) jsonrpc2.Handler {
	return func(ctx context.Context, reply jsonrpc2.Replier, req jsonrpc2.Request) error {
		go func() {
			if err := h(ctx, reply, req); err != nil {
				_ = reply(ctx, nil, err)
			}
		}()
		return nil
	}
}

func stdioReadWriteCloser() io.ReadWriteCloser {
	return struct {
		io.Reader
		io.Writer
		io.Closer
	}{os.Stdin, os.Stdout, os.Stdin}
}
