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
	"namespacelabs.dev/foundation/internal/findroot"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/versions"
	"namespacelabs.dev/foundation/schema"
)

const workspaceFilename = "workspace.ns.textpb"

func FindModuleRoot(dir string) (string, error) {
	return findroot.Find("workspace", dir, findroot.LookForFile(workspaceFilename))
}

func ModuleAt(path string) (*schema.Workspace, string, error) {
	file := filepath.Join(path, workspaceFilename)
	moduleBytes, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, "", err
	}

	// So we do a first-pass at the module definition, with loose parsing on, to
	// make sure that we meet the version requirements set by the module owners.

	firstPass := &schema.Workspace{}
	if err := (prototext.UnmarshalOptions{AllowPartial: true, DiscardUnknown: true}).Unmarshal(moduleBytes, firstPass); err != nil {
		return nil, "", fnerrors.Wrapf(nil, err, "failed to parse %s for validation", file)
	}

	if firstPass.GetFoundation().GetMinimumApi() > versions.APIVersion {
		return nil, "", fnerrors.DoesNotMeetVersionRequirements(firstPass.ModuleName, firstPass.GetFoundation().GetMinimumApi(), versions.APIVersion)
	}

	if firstPass.GetFoundation().GetMinimumApi() > 0 &&
		firstPass.GetFoundation().GetMinimumApi() < versions.MinimumAPIVersion {
		return nil, "", fnerrors.UserError(nil, `Unfortunately, this version of Foundation is too recent to be used with the
current repository. If you're testing out an existing repository that uses
Foundation, try fetching a newer version of the repository. If this is your
own codebase, then you'll need to either revert to a previous version of
"fn", or update your dependency versions with "fn mod tidy".

This version check will be removed in future non-alpha versions of
Foundation, which establish a stable longer term supported API surface.`)
	}

	w := &schema.Workspace{}
	if err := prototext.Unmarshal(moduleBytes, w); err != nil {
		return nil, "", fnerrors.Wrapf(nil, err, "failed to parse %s", file)
	}

	return w, workspaceFilename, nil
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

	if len(ws.Env) > 0 {
		writeTextMessage(&buf, &schema.Workspace{Env: ws.Env})
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
