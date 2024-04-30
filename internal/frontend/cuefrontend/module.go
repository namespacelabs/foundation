// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cuefrontend

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/format"
	"golang.org/x/exp/slices"
	"google.golang.org/protobuf/types/known/anypb"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/internal/parsing"
	"namespacelabs.dev/foundation/internal/protos"
	"namespacelabs.dev/foundation/internal/workspace"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
	"namespacelabs.dev/foundation/std/tasks"
)

var ModuleLoader moduleLoader

type moduleLoader struct{}

func (moduleLoader) FindModuleRoot(dir string) (string, error) {
	return parsing.RawFindModuleRoot(dir, workspace.WorkspaceFile, workspace.LegacyWorkspaceFile)
}

func (moduleLoader) ModuleAt(ctx context.Context, dir string, files ...string) (pkggraph.WorkspaceData, error) {
	return tasks.Return(ctx, tasks.Action("workspace.load-workspace").Arg("dir", dir), func(ctx context.Context) (pkggraph.WorkspaceData, error) {
		if len(files) == 0 {
			matched, err := filepath.Glob(filepath.Join(dir, "*.cue"))
			if err != nil {
				return nil, err
			}

			for _, path := range matched {
				rel, err := filepath.Rel(dir, path)
				if err != nil {
					return nil, err
				}

				files = append(files, rel)
			}
		}

		if len(files) == 0 {
			return nil, fnerrors.New("a workspace definition (ns-workspace.cue) is missing. You can use 'ns mod init' to create a default one.")
		}

		var loaded [][]byte
		for _, name := range files {
			data, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				return nil, err
			}

			loaded = append(loaded, data)
		}

		return moduleFrom(ctx, dir, files, loaded)
	})
}

func moduleFrom(ctx context.Context, dir string, files []string, loaded [][]byte) (pkggraph.WorkspaceData, error) {
	var memfs memfs.FS

	// we generate new filenames to place all files in one directory when evaluating the workspace.
	var tmpNames []string
	var count int
	for _, data := range loaded {
		name := fmt.Sprintf("fn-workspace-%d.cue", count)
		memfs.Add(name, data)

		tmpNames = append(tmpNames, name)
		count++
	}

	p, err := fncue.EvalWorkspace(ctx, &memfs, dir, tmpNames)
	if err != nil {
		return nil, err
	}

	parsed, err := parseWorkspaceValue(ctx, p.Val)
	if err != nil {
		return nil, err
	}

	wd := workspaceData{
		absPath:         dir,
		definitionFiles: files,
		parsed:          parsed,
		source:          p.Val,
	}

	if err := validateWorkspace(wd); err != nil {
		return nil, err
	}

	return wd, nil
}

func (moduleLoader) NewModule(ctx context.Context, dir string, w *schema.Workspace) (pkggraph.WorkspaceData, error) {
	val, err := decodeWorkspace(w)
	if err != nil {
		return nil, err
	}
	wd := workspaceData{
		absPath:         dir,
		definitionFiles: []string{workspace.WorkspaceFile},
		parsed:          w,
		source:          val,
	}

	if err := validateWorkspace(wd); err != nil {
		return nil, err
	}

	return wd, nil
}

func validateWorkspace(w workspaceData) error {
	module := w.ModuleName()
	if strings.ToLower(module) != module {
		return fnerrors.NewWithLocation(w, "invalid module name %q: may not contain uppercase letters", module)
	}

	u, err := url.Parse("https://" + module)
	if err != nil {
		return fnerrors.NewWithLocation(w, "invalid module name %q: %w", module, err)
	}
	if h := u.Hostname(); !strings.Contains(h, ".") {
		return fnerrors.NewWithLocation(w, "invalid module name %q: host %q does not contain `.`", module, h)
	}

	return nil
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

func parseWorkspaceValue(ctx context.Context, val cue.Value) (*schema.Workspace, error) {
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

	slices.SortFunc(w.Replace, func(a, b *schema.Workspace_Replace) int {
		if a.ModuleName == b.ModuleName {
			return strings.Compare(a.Path, b.Path)
		}
		return strings.Compare(a.ModuleName, b.ModuleName)
	})

	for moduleName, dep := range m.Dependencies {
		w.Dep = append(w.Dep, &schema.Workspace_Dependency{
			ModuleName: moduleName,
			Version:    dep.Version,
		})
	}

	slices.SortFunc(w.Dep, func(a, b *schema.Workspace_Dependency) int {
		if a.ModuleName == b.ModuleName {
			return strings.Compare(a.Version, b.Version)
		}
		return strings.Compare(a.ModuleName, b.ModuleName)
	})

	if m.Prebuilts != nil {
		w.PrebuiltBaseRepository = m.Prebuilts.BaseRepository

		for packageName, digest := range m.Prebuilts.Digests {
			w.PrebuiltBinary = append(w.PrebuiltBinary, &schema.Workspace_BinaryDigest{
				PackageName: packageName,
				Digest:      digest,
			})
		}

		slices.SortFunc(w.PrebuiltBinary, func(a, b *schema.Workspace_BinaryDigest) int {
			if a.PackageName == b.PackageName {
				return strings.Compare(a.Digest, b.Digest)
			}
			return strings.Compare(a.PackageName, b.PackageName)
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

		slices.SortFunc(out.Labels, func(a, b *schema.Label) int {
			if a.GetName() == b.GetName() {
				return strings.Compare(a.GetValue(), b.GetValue())
			}
			return strings.Compare(a.GetName(), b.GetName())
		})

		for _, kv := range env.Configuration {
			msg, err := protos.AllocateFrom(ctx, protos.ParseContext{}, kv)
			if err != nil {
				return nil, err
			}

			// This is a bit sad, but alas.
			packed, err := anypb.New(msg)
			if err != nil {
				return nil, fnerrors.New("failed to repack message: %w", err)
			}
			out.Configuration = append(out.Configuration, packed)
		}

		w.EnvSpec = append(w.EnvSpec, out)
	}

	for _, sb := range m.SecretBindings {
		ref, err := schema.ParsePackageRef(schema.PackageName(m.ModuleName), sb.Ref)
		if err != nil {
			return nil, err
		}

		msg, err := protos.AllocateFrom(ctx, protos.ParseContext{}, sb.Configuration)
		if err != nil {
			return nil, err
		}

		// This is a bit sad, but alas.
		packed, err := anypb.New(msg)
		if err != nil {
			return nil, fnerrors.New("failed to repack message: %w", err)
		}

		w.SecretBinding = append(w.SecretBinding, &schema.Workspace_SecretBinding{
			PackageRef:    ref,
			Environment:   sb.Environment,
			Configuration: packed,
		})
	}

	slices.SortFunc(w.EnvSpec, func(a, b *schema.Workspace_EnvironmentSpec) int {
		return strings.Compare(a.Name, b.Name)
	})

	w.ExperimentalProtoModuleImports = m.ProtoModuleImports
	w.EnabledFeatures = m.EnabledFeatures

	return w, nil
}

type workspaceData struct {
	absPath         string
	definitionFiles []string
	parsed          *schema.Workspace
	source          cue.Value
}

func (r workspaceData) ErrorLocation() string    { return r.absPath }
func (r workspaceData) ModuleName() string       { return r.parsed.ModuleName }
func (r workspaceData) Proto() *schema.Workspace { return r.parsed }

func (r workspaceData) AbsPath() string                { return r.absPath }
func (r workspaceData) ReadOnlyFS(rel ...string) fs.FS { return fnfs.Local(r.absPath, rel...) }
func (r workspaceData) DefinitionFiles() []string      { return r.definitionFiles }
func (r workspaceData) structLit() *ast.StructLit      { return r.source.Syntax().(*ast.StructLit) }

func (r workspaceData) LoadedFrom() *schema.Workspace_LoadedFrom {
	return &schema.Workspace_LoadedFrom{
		AbsPath:         r.absPath,
		DefinitionFiles: r.definitionFiles,
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

func (r workspaceData) WithModuleName(name string) pkggraph.WorkspaceData {
	syntax := r.structLit()

	for _, decl := range syntax.Elts {
		switch z := decl.(type) {
		case *ast.Field:
			if lbl, _, _ := ast.LabelName(z.Label); lbl == "module" {
				z.Value = ast.NewString(name)
			}
		}
	}

	copy := r
	copy.source = r.source.Context().BuildExpr(syntax)

	return copy
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

// Consider using https://pkg.go.dev/reflect#StructTag to infer this from cueModule
var ModuleFields = []string{
	"module", "requirements", "replace", "dependency", "prebuilts", "environment",
	"experimentalProtoModuleImports", "enabledFeatures",
}

type cueModule struct {
	ModuleName         string                                 `json:"module"`
	Foundation         *cueModuleFoundation                   `json:"requirements"`
	Replaces           map[string]string                      `json:"replace"` // Map: module name -> relative path.
	Dependencies       map[string]cueModuleVersion            `json:"dependency"`
	Prebuilts          *cueWorkspacePrebuilts                 `json:"prebuilts"`
	Environments       map[string]cueEnvironment              `json:"environment"`
	ProtoModuleImports []*schema.Workspace_ProtoModuleImports `json:"experimentalProtoModuleImports"`
	EnabledFeatures    []string                               `json:"enabledFeatures"`
	SecretBindings     []cueSecretBinding                     `json:"secretBinding"`
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
	Runtime       string            `json:"runtime"`
	Purpose       string            `json:"purpose"`
	Labels        map[string]string `json:"labels"`
	Configuration []map[string]any  `json:"configuration"`
	Policy        cuePolicy         `json:"policy"`
}

type cuePolicy struct {
	RequireDeploymentReason  bool   `json:"require_deployment_reason"`
	DeployUpdateSlackChannel string `json:"deploy_update_slack_channel"`
}

type cueSecretBinding struct {
	Ref           string         `json:"ref"`
	Environment   string         `json:"environment"`
	Configuration map[string]any `json:"configuration"`
}
