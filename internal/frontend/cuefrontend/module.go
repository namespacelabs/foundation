// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cuefrontend

import (
	"context"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/ast"
	"cuelang.org/go/cue/format"
	"golang.org/x/exp/slices"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs/memfs"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace"
	"namespacelabs.dev/foundation/workspace/tasks"
)

const (
	WorkspaceFile       = "ns-workspace.cue"
	LegacyWorkspaceFile = "fn-workspace.cue"
)

var ModuleLoader moduleLoader

type moduleLoader struct{}

func (moduleLoader) FindModuleRoot(dir string) (string, error) {
	return workspace.RawFindModuleRoot(dir, WorkspaceFile, LegacyWorkspaceFile)
}

func (moduleLoader) ModuleAt(ctx context.Context, dir string) (workspace.WorkspaceData, error) {
	return tasks.Return(ctx, tasks.Action("workspace.load-workspace").Arg("dir", dir), func(ctx context.Context) (workspace.WorkspaceData, error) {
		wfile := WorkspaceFile
		data, err := ioutil.ReadFile(filepath.Join(dir, WorkspaceFile))
		if err != nil {
			if os.IsNotExist(err) {
				wfile = LegacyWorkspaceFile
				data, err = ioutil.ReadFile(filepath.Join(dir, LegacyWorkspaceFile))
			}
		}

		if err != nil {
			if os.IsNotExist(err) {
				wd, werr := workspace.RawModuleAt(ctx, dir)
				if werr != nil {
					if os.IsNotExist(werr) {
						return nil, fnerrors.New("%s: failed to load workspace", dir)
					}
				}
				return wd, werr
			}

			return nil, err
		}

		return moduleFrom(ctx, dir, wfile, data)
	})
}

func moduleFrom(ctx context.Context, dir, workspaceFile string, data []byte) (workspace.WorkspaceData, error) {
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
			MinimumApi: int32(m.Foundation.MinimumAPI),
		}

		if err := workspace.ValidateAPIRequirements(m.ModuleName, w.Foundation); err != nil {
			return nil, err
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
			return nil, fnerrors.UserError(nil, "%s: no such environment purpose %q", name, env.Purpose)
		}

		w.Env = append(w.Env, &schema.Environment{
			Name:    name,
			Runtime: env.Runtime,
			Purpose: schema.Environment_Purpose(purpose),
		})
	}

	slices.SortFunc(w.Env, func(a, b *schema.Environment) bool {
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

func (r workspaceData) AbsPath() string           { return r.absPath }
func (r workspaceData) DefinitionFile() string    { return r.definitionFile }
func (r workspaceData) RawData() []byte           { return r.data }
func (r workspaceData) Parsed() *schema.Workspace { return r.parsed }
func (r workspaceData) structLit() *ast.StructLit { return r.source.Syntax().(*ast.StructLit) }

func (r workspaceData) FormatTo(w io.Writer) error {
	formatted, err := format.Node(&ast.File{Decls: r.structLit().Elts})
	if err != nil {
		return fnerrors.New("failed to produce cue syntax: %w", err)
	}

	_, err = w.Write(formatted)
	return err
}

func (r workspaceData) SetDependency(deps ...*schema.Workspace_Dependency) workspace.WorkspaceData {
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

func (r workspaceData) ReplaceDependencies(deps []*schema.Workspace_Dependency) workspace.WorkspaceData {
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
	MinimumAPI int `json:"api"`
}

type cueModuleVersion struct {
	Version string `json:"version"`
}

type cueWorkspacePrebuilts struct {
	Digests        map[string]string `json:"digest"` // Map: package name -> digest.
	BaseRepository string            `json:"baseRepository"`
}

type cueEnvironment struct {
	Runtime string `json:"runtime"`
	Purpose string `json:"purpose"`
}
