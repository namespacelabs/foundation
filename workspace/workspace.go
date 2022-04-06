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
	"strings"

	"github.com/protocolbuffers/txtpbfmt/parser"
	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/schema"
)

const WorkspaceFilename = "workspace.ns.textpb"

func ModuleAt(path string) (*schema.Workspace, error) {
	moduleBytes, err := ioutil.ReadFile(filepath.Join(path, WorkspaceFilename))
	if err != nil {
		return nil, err
	}

	// So we do a first-pass at the module definition, with loose parsing on, to
	// make sure that we meet the version requirements set by the module owners.

	firstPass := &schema.Workspace{}
	if err := (prototext.UnmarshalOptions{AllowPartial: true, DiscardUnknown: true}).Unmarshal(moduleBytes, firstPass); err != nil {
		return nil, fnerrors.Wrapf(nil, err, "failed to parse workspace definition")
	}

	if firstPass.GetFoundation().GetMinimumApi() > APIVersion {
		return nil, fnerrors.DoesNotMeetVersionRequirements(firstPass.ModuleName, firstPass.GetFoundation().GetMinimumApi(), APIVersion)
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

	writeTextMessage(&buf, &schema.Workspace{ModuleName: ws.ModuleName, Foundation: ws.Foundation})

	if len(ws.Dep) > 0 {
		fmt.Fprintln(&buf)
		writeTextMessage(&buf, &schema.Workspace{Dep: ws.Dep})
	}

	if len(ws.Replace) > 0 {
		fmt.Fprintln(&buf)
		writeTextMessage(&buf, &schema.Workspace{Replace: ws.Replace})
	}

	if len(ws.PrebuiltBinary) > 0 {
		sorted := slices.Clone(ws.PrebuiltBinary)
		slices.SortFunc(sorted, func(a, b *schema.Workspace_BinaryDigest) bool {
			return strings.Compare(a.PackageName, b.PackageName) < 0
		})
		fmt.Fprintln(&buf)
		writeTextMessage(&buf, &schema.Workspace{PrebuiltBinary: sorted})
	}

	if ws.PrebuiltBaseRepository != "" {
		writeTextMessage(&buf, &schema.Workspace{PrebuiltBaseRepository: ws.PrebuiltBaseRepository})
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
