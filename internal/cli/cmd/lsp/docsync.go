// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package lsp

import (
	"io/fs"
	"sync"

	"go.lsp.dev/protocol"
)

// Implements LSP document sync protocol to maintain a view into currently open files.
// For the open files the authoritative content is stored in the editor memory instead of the filesystem.
// TODO: Implement delta sync.
type OpenFiles struct {
	m     sync.RWMutex
	index map[protocol.URI]Snapshot
}

type Snapshot struct {
	Version int32
	Text    string
}

func NewOpenFiles() *OpenFiles {
	return &OpenFiles{
		index: make(map[protocol.URI]Snapshot),
	}
}

func (of *OpenFiles) Read(uri protocol.URI) (Snapshot, error) {
	of.m.RLock()
	defer of.m.RUnlock()

	snapshot, ok := of.index[uri]
	if !ok {
		return Snapshot{}, fs.ErrNotExist
	}
	return snapshot, nil
}

func (of *OpenFiles) DidOpen(params *protocol.DidOpenTextDocumentParams) (err error) {
	of.m.Lock()
	defer of.m.Unlock()

	of.index[params.TextDocument.URI] = Snapshot{
		Version: params.TextDocument.Version,
		Text:    params.TextDocument.Text,
	}
	return nil
}

func (of *OpenFiles) DidClose(params *protocol.DidCloseTextDocumentParams) (err error) {
	of.m.Lock()
	defer of.m.Unlock()

	delete(of.index, params.TextDocument.URI)
	return nil
}

func (of *OpenFiles) DidChange(params *protocol.DidChangeTextDocumentParams) (err error) {
	of.m.Lock()
	defer of.m.Unlock()

	newSnapshot := of.index[params.TextDocument.URI]
	newSnapshot.Version = params.TextDocument.Version

	for _, change := range params.ContentChanges {
		// We ignore ranges since we asked for TextDocumentSyncKindFull.
		// We also can't verify that a full update is provided:
		// https://github.com/go-language-server/protocol/issues/29#issue-966899747
		// TODO: Implement delta sync.
		newSnapshot.Text = change.Text
	}

	of.index[params.TextDocument.URI] = newSnapshot
	return nil
}
