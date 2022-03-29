// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package workspace

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"

	"github.com/protocolbuffers/txtpbfmt/parser"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/schema"
)

const WorkspaceFilename = "workspace.ns.textpb"

func ModuleAt(path string) (*schema.Workspace, error) {
	// XXX we need our own module definition.
	moduleBytes, err := ioutil.ReadFile(filepath.Join(path, WorkspaceFilename))
	if err != nil {
		return nil, err
	}

	w := &schema.Workspace{}
	if err := prototext.Unmarshal(moduleBytes, w); err != nil {
		return nil, err
	}

	return w, nil
}

func FormatWorkspace(w io.Writer, ws *schema.Workspace) error {
	// We force a particular structure by controlling which messages are emited when.

	var buf bytes.Buffer

	writeTextMessage(&buf, &schema.Workspace{ModuleName: ws.ModuleName})

	if len(ws.Dep) > 0 {
		fmt.Fprintln(&buf)
		writeTextMessage(&buf, &schema.Workspace{Dep: ws.Dep})
	}

	if len(ws.Replace) > 0 {
		fmt.Fprintln(&buf)
		writeTextMessage(&buf, &schema.Workspace{Replace: ws.Replace})
	}

	stableFmt, err := parser.Format(buf.Bytes())
	if err != nil {
		return err
	}

	fmt.Fprintf(w, "%s", stableFmt)
	return nil
}

func writeTextMessage(w io.Writer, msg proto.Message) {
	fmt.Fprint(w, prototext.MarshalOptions{Multiline: true}.Format(msg))
}