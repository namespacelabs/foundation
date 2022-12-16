// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package secrets

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"namespacelabs.dev/foundation/internal/fnerrors"
	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/internal/keys"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/cfg"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type localSecrets struct {
	keyDir          fnfs.LocalFS
	workspaceModule string
	env             *schema.Environment

	mu    sync.Mutex
	cache map[string]*Bundle
}

func NewLocalSecrets(env cfg.Context) (runtime.SecretSource, error) {
	keyDir, err := keys.KeysDir()
	if err != nil {
		if errors.Is(err, keys.ErrKeyGen) {
			keyDir = nil
		} else {
			return nil, err
		}
	}

	return &localSecrets{keyDir: keyDir, workspaceModule: env.Workspace().ModuleName(), env: env.Environment(), cache: map[string]*Bundle{}}, nil
}

func (l *localSecrets) Load(ctx context.Context, modules pkggraph.Modules, req runtime.SecretRequest) (*schema.FileContents, error) {
	// Ordered by lookup order.
	var bundles []*Bundle

	if userSecrets, err := l.loadSecretsFor(ctx, modules, l.workspaceModule, UserBundleName); err != nil {
		return nil, err
	} else if userSecrets != nil {
		bundles = append(bundles, userSecrets)
	}

	lookup := []string{l.workspaceModule}

	if req.Server != nil && req.Server.ModuleName != "" {
		if serverSecrets, err := l.loadSecretsFor(ctx, modules, req.Server.ModuleName, filepath.Join(req.Server.RelPath, ServerBundleName)); err != nil {
			return nil, err
		} else if serverSecrets != nil {
			bundles = append(bundles, serverSecrets)
		}
		lookup = append(lookup, req.Server.ModuleName)
	}

	for _, moduleName := range lookup {
		if bundle, err := l.loadSecretsFor(ctx, modules, moduleName, WorkspaceBundleName); err != nil {
			return nil, err
		} else if bundle != nil {
			bundles = append(bundles, bundle)
		}
	}

	return lookupSecret(ctx, l.env, req.SecretRef, bundles...)
}

func (l *localSecrets) MissingError(missing *schema.PackageRef, missingSpec *schema.SecretSpec, missingServer schema.PackageName) error {
	label := fmt.Sprintf("\n  # Description: %s\n  # Server: %s\n  ns secrets set --secret %s", missingSpec.Description, missingServer, missing.Canonical())

	return fnerrors.UsageError(
		fmt.Sprintf("Please run:\n%s", label),
		"There are secrets required which have not been specified")
}

func (l *localSecrets) loadSecretsFor(ctx context.Context, modules pkggraph.Modules, moduleName, secretFile string) (*Bundle, error) {
	if strings.Contains(moduleName, ":") {
		return nil, fnerrors.InternalError("module names can't contain colons")
	}

	key := fmt.Sprintf("%s:%s", moduleName, secretFile)

	l.mu.Lock()
	defer l.mu.Unlock()

	if existing, ok := l.cache[key]; ok {
		return existing, nil
	}

	fsys := l.moduleFS(modules, moduleName)
	if fsys == nil {
		return nil, fnerrors.InternalError("%s: module is not loaded", moduleName)
	}

	loaded, err := loadSecretsFile(ctx, l.keyDir, moduleName, fsys, secretFile)
	if err != nil {
		return nil, err
	}

	l.cache[key] = loaded
	return loaded, nil
}

func (l *localSecrets) moduleFS(modules pkggraph.Modules, moduleName string) fs.FS {
	for _, mod := range modules.Modules() {
		if mod.ModuleName() == moduleName {
			return mod.ReadOnlyFS()
		}
	}

	return nil
}

func loadSecretsFile(ctx context.Context, keyDir fs.FS, name string, fsys fs.FS, sourceFile string) (*Bundle, error) {
	contents, err := fs.ReadFile(fsys, sourceFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fnerrors.InternalError("%s: failed to read %q: %w", name, sourceFile, err)
	}

	return LoadBundle(ctx, keyDir, contents)
}

func lookupSecret(ctx context.Context, env *schema.Environment, secretRef *schema.PackageRef, lookup ...*Bundle) (*schema.FileContents, error) {
	key := &ValueKey{PackageName: secretRef.PackageName, Key: secretRef.Name, EnvironmentName: env.Name}

	for _, src := range lookup {
		value, err := src.Lookup(ctx, key)
		if err != nil {
			return nil, err
		}

		if value != nil {
			return &schema.FileContents{Contents: value, Utf8: true}, nil
		}
	}

	return nil, nil
}
