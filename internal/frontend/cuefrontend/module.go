// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cuefrontend

import (
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/format"
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/tasks"
)

const (
	WorkspaceFile       = "ns-workspace.cue"
	LegacyWorkspaceFile = "fn-workspace.cue"
)

var ModuleLoader moduleLoader

type moduleLoader struct{}

func (moduleLoader) FindModuleRoot(dir string) (string, error) {
	return parsing.RawFindModuleRoot(dir, WorkspaceFile, LegacyWorkspaceFile)
}

func (moduleLoader) ModuleAt(ctx context.Context, dir string) (pkggraph.WorkspaceData, error) {
	return tasks.Return(ctx, tasks.Action("workspace.load-workspace").Arg("dir", dir), func(ctx context.Context) (pkggraph.WorkspaceData, error) {
		wfile := WorkspaceFile
		data, err := os.ReadFile(filepath.Join(dir, WorkspaceFile))
		if err != nil {
			if os.IsNotExist(err) {
				wfile = LegacyWorkspaceFile
				data, err = os.ReadFile(filepath.Join(dir, LegacyWorkspaceFile))
			}
		}

		if err != nil {
			if os.IsNotExist(err) {
				return nil, fnerrors.New("a workspace definition (ns-workspace.cue) is missing. You can use 'ns mod init' to create a default one.")
			}

			return nil, err
		}

		return moduleFrom(ctx, dir, wfile, data)
	})
}

func moduleFrom(ctx context.Context, dir, workspaceFile string, data []byte) (pkggraph.WorkspaceData, error) {
	var memfs memfs.FS
	memfs.Add(workspaceFile, data)

	p, err := fncue.EvalWorkspace(ctx, &memfs, dir, []string{workspaceFile})
	if err != nil {
		return nil, err
	}

	w, err := parseWorkspaceValue(p.Val)
	if err != nil {
		return nil, err
	}

	return workspaceData{
		absPath:        dir,
		definitionFile: workspaceFile,
		data:           data,
		parsed:         w,
		source:         p.Val,
	}, nil
}

func (moduleLoader) NewModule(ctx context.Context, dir string, w *schema.Workspace) (pkggraph.WorkspaceData, error) {
	val, err := decodeWorkspace(w)
	if err != nil {
		return nil, err
	}
	return workspaceData{
		absPath:        dir,
		definitionFile: WorkspaceFile,
		data:           nil,
		parsed:         w,
		source:         val,
	}, nil
}

func decodeWorkspace(w *schema.Workspace) (cue.Value, error) {
	m := cueModule{
		ModuleName: w.GetModuleName(),
	}

	if req := w.GetFoundation(); req != nil {
		m.Foundation = &cueModuleFoundation{
			MinimumAPI:   int(req.MinimumApi),
			ToolsVersion: int(req.ToolsVersion),
		}
	}

	if len(w.GetReplace()) > 0 {
		m.Replaces = make(map[string]string)

		for _, r := range w.GetReplace() {
			m.Replaces[r.ModuleName] = r.Path
		}
	}

	if len(w.GetDep()) > 0 {
		m.Dependencies = make(map[string]cueModuleVersion)

		for _, d := range w.GetDep() {
			m.Dependencies[d.ModuleName] = cueModuleVersion{
				Version: d.Version,
			}
		}
	}

	if w.GetPrebuiltBaseRepository() != "" {
		m.Prebuilts = &cueWorkspacePrebuilts{
			BaseRepository: w.GetPrebuiltBaseRepository(),
		}

		for _, pb := range w.GetPrebuiltBinary() {
			if pb.Repository != "" && pb.Repository != w.GetPrebuiltBaseRepository() {
				return cue.Value{}, fnerrors.InternalError("prebuilt %q: custom prebuilt repository not supported", pb.PackageName)
			}

			m.Prebuilts.Digests[pb.PackageName] = pb.Digest
		}
	}

	if len(w.GetEnvSpec()) > 0 {
		m.Environments = make(map[string]cueEnvironment)

		for _, e := range w.GetEnvSpec() {
			m.Environments[e.Name] = cueEnvironment{
				Runtime: e.Runtime,
				Purpose: e.Purpose.String(),
			}
		}
	}

	ctx := cuecontext.New()
	val := ctx.Encode(m)
	return val, nil
}

func parseWorkspaceValue(val cue.Value) (*schema.Workspace, error) {
	var m cueModule
	if err := val.Decode(&m); err != nil {
		return nil, fnerrors.New("failed to decode workspace contents: %w", err)
	}

	w := &schema.Workspace{
		ModuleName: m.ModuleName,
	}

	if m.Foundation != nil {
		w.Foundation = &schema.Workspace_FoundationRequirements{
			MinimumApi:   int32(m.Foundation.MinimumAPI),
			ToolsVersion: int32(m.Foundation.ToolsVersion),
		}
	}

	for moduleName, relPath := range m.Replaces {
		w.Replace = append(w.Replace, &schema.Workspace_Replace{
			ModuleName: moduleName,
			Path:       relPath,
		})
	}

	slices.SortFunc(w.Replace, func(a, b *schema.Workspace_Replace) bool {
		if a.ModuleName == b.ModuleName {
			return strings.Compare(a.Path, b.Path) < 0
		}
		return strings.Compare(a.ModuleName, b.ModuleName) < 0
	})

	for moduleName, dep := range m.Dependencies {
		w.Dep = append(w.Dep, &schema.Workspace_Dependency{
			ModuleName: moduleName,
			Version:    dep.Version,
		})
	}

	slices.SortFunc(w.Dep, func(a, b *schema.Workspace_Dependency) bool {
		if a.ModuleName == b.ModuleName {
			return strings.Compare(a.Version, b.Version) < 0
		}
		return strings.Compare(a.ModuleName, b.ModuleName) < 0
	})

	if m.Prebuilts != nil {
		w.PrebuiltBaseRepository = m.Prebuilts.BaseRepository

		for packageName, digest := range m.Prebuilts.Digests {
			w.PrebuiltBinary = append(w.PrebuiltBinary, &schema.Workspace_BinaryDigest{
				PackageName: packageName,
				Digest:      digest,
			})
		}

		slices.SortFunc(w.PrebuiltBinary, func(a, b *schema.Workspace_BinaryDigest) bool {
			if a.PackageName == b.PackageName {
				return strings.Compare(a.Digest, b.Digest) < 0
			}
			return strings.Compare(a.PackageName, b.PackageName) < 0
		})
	}

	for name, env := range m.Environments {
		purpose, ok := schema.Environment_Purpose_value[strings.ToUpper(env.Purpose)]
		if !ok || purpose == 0 {
			return nil, fnerrors.New("%s: no such environment purpose %q", name, env.Purpose)
		}

		out := &schema.Workspace_EnvironmentSpec{
			Name:    name,
			Runtime: env.Runtime,
			Purpose: schema.Environment_Purpose(purpose),
		}

		for k, v := range env.Labels {
			out.Labels = append(out.Labels, &schema.Label{Name: k, Value: v})
		}

		slices.SortFunc(out.Labels, func(a, b *schema.Label) bool {
			if a.GetName() == b.GetName() {
				return strings.Compare(a.GetValue(), b.GetValue()) < 0
			}
			return strings.Compare(a.GetName(), b.GetName()) < 0
		})

		w.EnvSpec = append(w.EnvSpec, out)
	}

	slices.SortFunc(w.EnvSpec, func(a, b *schema.Workspace_EnvironmentSpec) bool {
		return strings.Compare(a.Name, b.Name) < 0
	})

	w.ExperimentalProtoModuleImports = m.ProtoModuleImports

	return w, nil
}

type workspaceData struct {
	absPath, definitionFile string
	data                    []byte
	parsed                  *schema.Workspace
	source                  cue.Value
}

func (r workspaceData) ErrorLocation() string    { return r.absPath }
func (r workspaceData) ModuleName() string       { return r.parsed.ModuleName }
func (r workspaceData) Proto() *schema.Workspace { return r.parsed }

func (r workspaceData) AbsPath() string           { return r.absPath }
func (r workspaceData) ReadOnlyFS() fs.FS         { return fnfs.Local(r.absPath) }
func (r workspaceData) DefinitionFile() string    { return r.definitionFile }
func (r workspaceData) RawData() []byte           { return r.data }
func (r workspaceData) structLit() *ast.StructLit { return r.source.Syntax().(*ast.StructLit) }

func (r workspaceData) LoadedFrom() *schema.Workspace_LoadedFrom {
	return &schema.Workspace_LoadedFrom{
		AbsPath:        r.absPath,
		DefinitionFile: r.definitionFile,
		Contents:       r.data,
	}
}

func (r workspaceData) FormatTo(w io.Writer) error {
	formatted, err := format.Node(&ast.File{Decls: r.structLit().Elts})
	if err != nil {
		return fnerrors.New("failed to produce cue syntax: %w", err)
	}

	_, err = w.Write(formatted)
	return err
}

func (r workspaceData) WithSetEnvironment(envs ...*schema.Workspace_EnvironmentSpec) pkggraph.WorkspaceData {
	var add []*schema.Workspace_EnvironmentSpec

	for _, env := range envs {
		found := false
		for _, x := range r.parsed.EnvSpec {
			if x.Name == env.Name {
				found = true
				break
			}
		}
		if !found {
			add = append(add, env)
		}
	}
	return r.updateEnvironments(add, nil, nil)
}

func (r workspaceData) updateEnvironments(add, update []*schema.Workspace_EnvironmentSpec, remove []string) workspaceData {
	syntax := r.structLit()
	hasEnvBlock := false
	for _, decl := range syntax.Elts {
		switch z := decl.(type) {
		case *ast.Field:
			if lbl, _, _ := ast.LabelName(z.Label); lbl == "environment" {
				switch st := z.Value.(type) {
				case *ast.StructLit:
					hasEnvBlock = true
					for _, add := range add {
						st.Elts = append(st.Elts, makeEnv(add))
					}
				}
			}
		}
	}

	if !hasEnvBlock && len(add) > 0 {
		var d []interface{}
		for _, add := range add {
			d = append(d, makeEnv(add))
		}

		syntax.Elts = append(syntax.Elts, &ast.Field{
			Label: ast.NewIdent("environment"),
			Value: ast.NewStruct(d...),
		})
	}
	copy := r
	copy.source = r.source.Context().BuildExpr(syntax)

	return copy
}

func makeEnv(v *schema.Workspace_EnvironmentSpec) *ast.Field {
	values := []interface{}{
		&ast.Field{
			Label: ast.NewIdent("runtime"),
			Value: ast.NewString(v.Runtime),
		},
		&ast.Field{
			Label: ast.NewIdent("purpose"),
			Value: ast.NewString(v.Purpose.String()),
		},
	}
	if len(v.Labels) > 0 {
		var labelValues []interface{}
		for _, lv := range v.Labels {
			labelValues = append(labelValues, &ast.Field{
				Label: ast.NewIdent(lv.Name),
				Value: ast.NewString(lv.Value),
			})
		}
		labels := &ast.Field{
			Label: ast.NewIdent("labels"),
			Value: ast.NewStruct(labelValues...),
		}
		values = append(values, labels)
	}
	return &ast.Field{
		Label: ast.NewIdent(v.Name),
		Value: ast.NewStruct(values...),
	}
}

func (r workspaceData) WithSetDependency(deps ...*schema.Workspace_Dependency) pkggraph.WorkspaceData {
	var add, update []*schema.Workspace_Dependency

	for _, dep := range deps {
		updated := false
		for _, x := range r.parsed.Dep {
			if x.ModuleName == dep.ModuleName {
				update = append(update, dep)
				updated = true
			}
		}
		if !updated {
			add = append(add, dep)
		}
	}

	return r.updateDependencies(add, update, nil)
}

func (r workspaceData) WithReplacedDependencies(deps []*schema.Workspace_Dependency) pkggraph.WorkspaceData {
	var add, update []*schema.Workspace_Dependency
	var toremove []string

	observed := map[string]struct{}{}

	for _, dep := range deps {
		updated := false
		for _, x := range r.parsed.Dep {
			if x.ModuleName == dep.ModuleName {
				update = append(update, dep)
				updated = true
			}
		}
		if !updated {
			add = append(add, dep)
		}

		observed[dep.ModuleName] = struct{}{}
	}

	for _, dep := range r.parsed.Dep {
		if _, ok := observed[dep.ModuleName]; !ok {
			toremove = append(toremove, dep.ModuleName)
		}
	}

	return r.updateDependencies(add, update, toremove)
}

func (r workspaceData) updateDependencies(add, update []*schema.Workspace_Dependency, remove []string) workspaceData {
	syntax := r.structLit()
	hasDependencyBlock := false
	for _, decl := range syntax.Elts {
		switch z := decl.(type) {
		case *ast.Field:
			if lbl, _, _ := ast.LabelName(z.Label); lbl == "dependency" {
				switch st := z.Value.(type) {
				case *ast.StructLit:
					hasDependencyBlock = true

					for _, update := range update {
						// XXX O(n^2)
						for _, stdecl := range st.Elts {
							switch z := stdecl.(type) {
							case *ast.Field:
								name, _, _ := ast.LabelName(z.Label)
								if update.ModuleName == name {
									z.Value = makeVersion(update.Version)
								}
							}
						}
					}

					for _, add := range add {
						st.Elts = append(st.Elts, &ast.Field{
							Label: ast.NewIdent(add.ModuleName),
							Value: makeVersion(add.Version),
						})
					}

					for _, name := range remove {
						index := slices.IndexFunc(st.Elts, func(decl ast.Decl) bool {
							switch x := decl.(type) {
							case *ast.Field:
								lbl, _, _ := ast.LabelName(x.Label)
								if lbl == name {
									return true
								}
							}
							return false
						})

						st.Elts = slices.Delete(st.Elts, index, index+1)
					}
				}
			}
		}
	}

	if !hasDependencyBlock && len(add) > 0 {
		var d []interface{}
		for _, add := range add {
			d = append(d, &ast.Field{
				Label: ast.NewIdent(add.ModuleName),
				Value: makeVersion(add.Version),
			})
		}

		syntax.Elts = append(syntax.Elts, &ast.Field{
			Label: ast.NewIdent("dependency"),
			Value: ast.NewStruct(d...),
		})
	}

	copy := r
	copy.source = r.source.Context().BuildExpr(syntax)

	return copy
}

func makeVersion(v string) *ast.StructLit {
	return ast.NewStruct(&ast.Field{
		Label: ast.NewIdent("version"),
		Value: ast.NewString(v),
	})
}

type cueModule struct {
	ModuleName         string                                 `json:"module"`
	Foundation         *cueModuleFoundation                   `json:"requirements"`
	Replaces           map[string]string                      `json:"replace"` // Map: module name -> relative path.
	Dependencies       map[string]cueModuleVersion            `json:"dependency"`
	Prebuilts          *cueWorkspacePrebuilts                 `json:"prebuilts"`
	Environments       map[string]cueEnvironment              `json:"environment"`
	ProtoModuleImports []*schema.Workspace_ProtoModuleImports `json:"experimentalProtoModuleImports"`
}

type cueModuleFoundation struct {
	MinimumAPI   int `json:"api"`
	ToolsVersion int `json:"toolsVersion"`
}

type cueModuleVersion struct {
	Version string `json:"version"`
}

type cueWorkspacePrebuilts struct {
	Digests        map[string]string `json:"digest"` // Map: package name -> digest.
	BaseRepository string            `json:"baseRepository"`
}

type cueEnvironment struct {
	Runtime string            `json:"runtime"`
	Purpose string            `json:"purpose"`
	Labels  map[string]string `json:"labels"`
}
