// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package parsing

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/exp/slices"
	"golang.org/x/net/html"
	"namespacelabs.dev/foundation/internal/artifacts/download"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/internal/console"
	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/fnfs/zipfs"
	"namespacelabs.dev/foundation/internal/git"
	"namespacelabs.dev/foundation/internal/localexec"
	"namespacelabs.dev/foundation/internal/workspace/dirs"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/tasks"
	"namespacelabs.dev/go-ids"
)

var publicRepos = []string{
	"https://github.com/namespacelabs/foundation",
}

// LocalModule represents a module that is present in the specified LocalPath.
type LocalModule struct {
	ModuleName string
	LocalPath  string
	Version    string
}

type ResolvedPackage struct {
	ModuleName string
	Type       string
	Repository string
	RelPath    string
}

func ResolveModuleVersion(ctx context.Context, packageName string) (*schema.Workspace_Dependency, error) {
	resolved, err := ResolveModule(ctx, packageName)
	if err != nil {
		return nil, err
	}

	return ModuleHead(ctx, resolved)
}

func ModuleHead(ctx context.Context, resolved *ResolvedPackage) (*schema.Workspace_Dependency, error) {
	return tasks.Return(ctx, tasks.Action("module.resolve-head").Arg("name", resolved.ModuleName), func(ctx context.Context) (*schema.Workspace_Dependency, error) {
		var out bytes.Buffer
		cmd := exec.CommandContext(ctx, "git", "ls-remote", "-q", resolved.Repository, "HEAD")
		cmd.Env = append(os.Environ(), git.NoPromptEnv().Serialize()...)
		cmd.Stdout = &out
		cmd.Stderr = console.Output(ctx, "git")

		if err := cmd.Run(); err != nil {
			return nil, fnerrors.InvocationError("%s: failed to `git ls-remote`: %w", resolved.Repository, err)
		}

		gitout := strings.TrimSpace(out.String())
		if p := strings.TrimSuffix(gitout, "\tHEAD"); p != gitout {
			dep := &schema.Workspace_Dependency{}
			dep.ModuleName = resolved.ModuleName
			dep.Version = strings.TrimSpace(p)
			return dep, nil
		}

		return nil, fnerrors.InvocationError("%s: failed to resolve HEAD", resolved.Repository)
	})
}

func ResolveModule(ctx context.Context, packageName string) (*ResolvedPackage, error) {
	// Check if there's a foundation redirect.
	var r ResolvedPackage
	if err := resolvePackageTo(ctx, packageName, &r); err != nil {
		return nil, err
	}

	if r.Type != "git" {
		return nil, fnerrors.New("%s: %s: unsupported type", packageName, r.Type)
	}

	return &r, nil
}

func resolvePackageTo(ctx context.Context, packageName string, resolved *ResolvedPackage) error {
	return tasks.Action("module.resolve").Arg("name", packageName).Run(ctx, func(ctx context.Context) error {
		contents, err := http.Get(fmt.Sprintf("https://%s?foundation-get=1", packageName))
		if err != nil {
			return err
		}

		defer contents.Body.Close()

		doc, err := html.Parse(contents.Body)
		if err != nil {
			return err
		}

		if v := recurse(doc); v != "" {
			parts := strings.Split(v, " ")

			switch len(parts) {
			case 3:
				moduleName := parts[0]
				var rel string
				if moduleName != packageName {
					rel = strings.TrimPrefix(packageName, moduleName+"/")
					if rel == packageName {
						return fnerrors.BadInputError("%s: invalid format, resolved package claimed it was module %q", packageName, moduleName)
					}
				}

				*resolved = ResolvedPackage{
					ModuleName: moduleName,
					Type:       parts[1],
					Repository: parts[2],
					RelPath:    rel,
				}
				return nil

			case 4:
				*resolved = ResolvedPackage{
					ModuleName: parts[0],
					Type:       parts[1],
					Repository: parts[2],
					RelPath:    parts[3],
				}
				return nil

			default:
				fmt.Fprintf(console.Warnings(ctx), "Ignored foundation-import definition, wrong number of parts: %d (got %q)\n", len(parts), v)
			}
		}

		if strings.HasPrefix(packageName, "github.com/") {
			r, err := parseGithubPackage(packageName)
			if err != nil {
				return err
			}
			*resolved = *r
			return nil
		}

		return fnerrors.InternalError("%s: don't know how to handle package", packageName)
	})
}

func parseGithubPackage(packageName string) (*ResolvedPackage, error) {
	// github.com/org/repo/rel
	parts := strings.SplitN(packageName, "/", 4)
	if len(parts) < 3 {
		return nil, fnerrors.New("%s: invalid github package name", packageName)
	}

	var rel string
	if len(parts) > 3 {
		rel = strings.Join(parts[3:], "/")
	}

	moduleName := fmt.Sprintf("github.com/%s/%s", parts[1], parts[2])
	return &ResolvedPackage{
		ModuleName: moduleName,
		Type:       "git",
		Repository: fmt.Sprintf("https://%s", moduleName),
		RelPath:    rel,
	}, nil
}

func recurse(n *html.Node) string {
	if n.Type == html.ElementNode && n.Data == "meta" {
		name := getAttr(n.Attr, "name")
		if name != nil && name.Val == "foundation-import" {
			content := getAttr(n.Attr, "content")
			if content != nil {
				return content.Val
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if x := recurse(c); x != "" {
			return x
		}
	}
	return ""
}

func getAttr(attrs []html.Attribute, key string) *html.Attribute {
	for _, attr := range attrs {
		if attr.Key == key {
			return &attr
		}
	}
	return nil
}

func DownloadModule(ctx context.Context, dep *schema.Workspace_Dependency, force bool) (*LocalModule, error) {
	return tasks.Return(ctx, tasks.Action("module.download").Arg("name", dep.ModuleName).Arg("version", dep.Version), func(ctx context.Context) (*LocalModule, error) {
		modDir, err := dirs.ModuleCache(dep.ModuleName, dep.Version)
		if err != nil {
			return nil, err
		}

		// XXX more thorough check of the contents?
		if !force {
			if _, err := os.Stat(modDir); err == nil {
				// Already exists.
				return &LocalModule{ModuleName: dep.ModuleName, LocalPath: modDir, Version: dep.Version}, nil
			}
		}

		mod, err := ResolveModule(ctx, dep.ModuleName)
		if err != nil {
			return nil, err
		}

		tmpModDir, err := dirs.ModuleCache(dep.ModuleName, fmt.Sprintf("tmp-%s", ids.NewRandomBase32ID(8)))
		if err != nil {
			return nil, err
		}

		srcDir := tmpModDir

		defer func() {
			if tmpModDir != "" {
				os.RemoveAll(tmpModDir)
			}
		}()

		if slices.Contains(publicRepos, mod.Repository) {
			contents, err := compute.GetValue(ctx, download.UnverifiedURL(fmt.Sprintf("%s/archive/%s.zip", mod.Repository, dep.Version)))
			if err != nil {
				return nil, err
			}

			if err := tasks.Action("module.extract").Arg("module", mod.Repository).Arg("version", dep.Version).
				Run(ctx, func(ctx context.Context) error {
					return zipfs.UnzipContents(ctx, fnfs.ReadWriteLocalFS(tmpModDir), contents)
				}); err != nil {
				return nil, err
			}

			srcDir = filepath.Join(tmpModDir, fmt.Sprintf("%s-%s", filepath.Base(mod.Repository), dep.Version))
		} else {
			var cmd localexec.Command
			cmd.Command = "git"
			cmd.Args = []string{"clone", "-q", mod.Repository, tmpModDir}
			cmd.AdditionalEnv = git.NoPromptEnv().Serialize()
			cmd.Label = "git clone"
			if err := cmd.Run(ctx); err != nil {
				return nil, err
			}

			cmd.Args = []string{"reset", "-q", "--hard", dep.Version}
			cmd.Label = "git reset"
			cmd.Dir = tmpModDir
			if err := cmd.Run(ctx); err != nil {
				return nil, err
			}

			tmpModDir = "" // Inhibit the os.RemoveAll() above.
		}

		if force {
			// Errors are ignored as the module directory may not exist, and if it doesn't
			// and this fails, then Rename below will fail.
			_ = os.RemoveAll(modDir)
		}

		if err := os.Rename(srcDir, modDir); err != nil {
			return nil, err
		}

		return &LocalModule{ModuleName: dep.ModuleName, LocalPath: modDir, Version: dep.Version}, nil
	})
}

type MissingModuleResolver interface {
	Resolve(context.Context, schema.PackageName) (*schema.Workspace_Dependency, error)
}

type defaultMissingModuleResolver struct {
	workspace cfg.Workspace
}

func (r *defaultMissingModuleResolver) Resolve(ctx context.Context, pkg schema.PackageName) (*schema.Workspace_Dependency, error) {
	return nil, fnerrors.UsageError("Run `ns tidy`.", "%s: missing entry in %s: run:\n  ns tidy", pkg, r.workspace.LoadedFrom().DefinitionFile)
}
