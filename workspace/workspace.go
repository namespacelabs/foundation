// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package workspace

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"sort"
	"strings"

	"github.com/protocolbuffers/txtpbfmt/parser"
	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/proto"
	"namespacelabs.dev/foundation/internal/findroot"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/internal/versions"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/workspace/tasks"
)

const (
	originalWorkspaceFilename = "workspace.ns.textpb"
	foundationModule          = "namespacelabs.dev/foundation"
)

var ModuleLoader interface {
	FindModuleRoot(string) (string, error)
	ModuleAt(context.Context, string) (pkggraph.WorkspaceData, error)
}

func FindModuleRoot(dir string) (string, error) {
	return ModuleLoader.FindModuleRoot(dir)
}

type ModuleAtArgs struct {
	SkipAPIRequirements bool
}

// Loads and validates a module at a given path.
func ModuleAt(ctx context.Context, path string, args ModuleAtArgs) (pkggraph.WorkspaceData, error) {
	ws, err := ModuleLoader.ModuleAt(ctx, path)
	if err != nil {
		return ws, err
	}

	if !args.SkipAPIRequirements {
		if err := validateAPIRequirements(ws.ModuleName(), ws.Proto().Foundation); err != nil {
			return ws, err
		}
	}

	return ws, nil
}

func RawFindModuleRoot(dir string, names ...string) (string, error) {
	return findroot.Find("workspace", dir, findroot.LookForFile(append(names, originalWorkspaceFilename)...))
}

// RawModuleAt returns a schema.WorkspaceData with a reference to the workspace filename tried, even when errors are returned.
func RawModuleAt(ctx context.Context, path string) (pkggraph.WorkspaceData, error) {
	return tasks.Return(ctx, tasks.Action("workspace.load-workspace-textpb").Arg("dir", path), func(ctx context.Context) (pkggraph.WorkspaceData, error) {
		data := rawWorkspaceData{absPath: path, definitionFile: originalWorkspaceFilename}

		file := filepath.Join(path, originalWorkspaceFilename)
		moduleBytes, err := ioutil.ReadFile(file)
		if err != nil {
			return data, err
		}

		// So we do a first-pass at the module definition, with loose parsing on, to
		// make sure that we meet the version requirements set by the module owners.

		firstPass := &schema.Workspace{}
		if err := (prototext.UnmarshalOptions{AllowPartial: true, DiscardUnknown: true}).Unmarshal(moduleBytes, firstPass); err != nil {
			return data, fnerrors.Wrapf(nil, err, "failed to parse %s for validation", file)
		}

		w := &schema.Workspace{}
		if err := prototext.Unmarshal(moduleBytes, w); err != nil {
			return data, fnerrors.Wrapf(nil, err, "failed to parse %s", file)
		}

		data.data = moduleBytes
		data.parsed = w
		return data, nil
	})
}

func validateAPIRequirements(moduleName string, w *schema.Workspace_FoundationRequirements) error {
	if w.GetMinimumApi() > versions.APIVersion {
		return fnerrors.DoesNotMeetVersionRequirements(moduleName, w.GetMinimumApi(), versions.APIVersion)
	}

	// Check that the foundation repo dep uses an API compatible with the current CLI.
	if moduleName == foundationModule && w.GetMinimumApi() > 0 && w.GetMinimumApi() < versions.MinimumAPIVersion {
		return fnerrors.UserError(nil, fmt.Sprintf(`Unfortunately, this version of Foundation is too recent to be used with the
current repository. If you're testing out an existing repository that uses
Foundation, try fetching a newer version of the repository. If this is your
own codebase, then you'll need to either revert to a previous version of
"ns", or update your dependency versions with "ns mod get %s".

This version check will be removed in future non-alpha versions of
Foundation, which establish a stable longer term supported API surface.`, foundationModule))
	}

	return nil
}

func writeTextMessage(w io.Writer, msg proto.Message) {
	fmt.Fprint(w, prototext.MarshalOptions{Multiline: true}.Format(msg))
}

type rawWorkspaceData struct {
	absPath, definitionFile string
	data                    []byte
	parsed                  *schema.Workspace
}

func (r rawWorkspaceData) ErrorLocation() string    { return r.absPath }
func (r rawWorkspaceData) ModuleName() string       { return r.parsed.ModuleName }
func (r rawWorkspaceData) Proto() *schema.Workspace { return r.parsed }

func (r rawWorkspaceData) AbsPath() string        { return r.absPath }
func (r rawWorkspaceData) DefinitionFile() string { return r.definitionFile }
func (r rawWorkspaceData) RawData() []byte        { return r.data }

func (r rawWorkspaceData) FormatTo(w io.Writer) error {
	// We force a particular structure by controlling which messages are emited when.

	var buf bytes.Buffer

	ws := r.parsed
	writeTextMessage(&buf, &schema.Workspace{ModuleName: ws.ModuleName, Foundation: ws.Foundation})

	if len(ws.Dep) > 0 {
		sort.Slice(ws.Dep, func(i, j int) bool {
			return strings.Compare(ws.Dep[i].ModuleName, ws.Dep[j].ModuleName) < 0
		})

		fmt.Fprintln(&buf)
		writeTextMessage(&buf, &schema.Workspace{Dep: ws.Dep})
	}

	if len(ws.Replace) > 0 {
		fmt.Fprintln(&buf)
		writeTextMessage(&buf, &schema.Workspace{Replace: ws.Replace})
	}

	if len(ws.EnvSpec) > 0 {
		writeTextMessage(&buf, &schema.Workspace{EnvSpec: ws.EnvSpec})
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

func (r rawWorkspaceData) WithSetDependency(deps ...*schema.Workspace_Dependency) pkggraph.WorkspaceData {
	cloned := protos.Clone(r.parsed)

	var mods, changes int
	var newDeps []*schema.Workspace_Dependency

	for _, dep := range deps {
		for _, existing := range cloned.Dep {
			if existing.ModuleName == dep.ModuleName {
				mods++
				if existing.Version != dep.Version {
					existing.Version = dep.Version
					changes++
				}
			}
		}
	}

	if mods == 0 {
		cloned.Dep = append(cloned.Dep, newDeps...)
		copy := r
		copy.parsed = cloned
		return copy
	}

	return nil
}

func (r rawWorkspaceData) WithReplacedDependencies(deps []*schema.Workspace_Dependency) pkggraph.WorkspaceData {
	cloned := protos.Clone(r.parsed)
	cloned.Dep = deps
	copy := r
	copy.parsed = cloned
	return copy
}

func (r rawWorkspaceData) LoadedFrom() *schema.Workspace_LoadedFrom {
	return &schema.Workspace_LoadedFrom{
		AbsPath:        r.AbsPath(),
		DefinitionFile: r.DefinitionFile(),
		Contents:       r.RawData(),
	}
}
