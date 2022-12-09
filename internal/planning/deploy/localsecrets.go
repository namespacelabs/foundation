// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package deploy

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
	"namespacelabs.dev/foundation/internal/secrets"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

type localSecrets struct {
	keyDir  fnfs.LocalFS
	modules pkggraph.Modules

	mu    sync.Mutex
	cache map[string]*secrets.Bundle
}

func newLocalSecrets(modules pkggraph.Modules) (SecretSource, error) {
	keyDir, err := keys.KeysDir()
	if err != nil {
		if errors.Is(err, keys.ErrKeyGen) {
			keyDir = nil
		} else {
			return nil, err
		}
	}

	return &localSecrets{keyDir: keyDir, modules: modules, cache: map[string]*secrets.Bundle{}}, nil
}

func (l *localSecrets) Load(ctx context.Context, req SecretRequest) (*schema.FileContents, error) {
	// Ordered by lookup order.
	var bundles []*secrets.Bundle

	if userSecrets, err := l.loadSecretsFor(ctx, req.SecretsContext.WorkspaceModuleName, secrets.UserBundleName); err != nil {
		return nil, err
	} else if userSecrets != nil {
		bundles = append(bundles, userSecrets)
	}

	if serverSecrets, err := l.loadSecretsFor(ctx, req.Server.ModuleName, filepath.Join(req.Server.RelPath, secrets.ServerBundleName)); err != nil {
		return nil, err
	} else if serverSecrets != nil {
		bundles = append(bundles, serverSecrets)
	}

	for _, moduleName := range []string{
		req.SecretsContext.WorkspaceModuleName,
		req.Server.ModuleName,
	} {
		if bundle, err := l.loadSecretsFor(ctx, moduleName, secrets.WorkspaceBundleName); err != nil {
			return nil, err
		} else if bundle != nil {
			bundles = append(bundles, bundle)
		}
	}

	return lookupSecret(ctx, req.SecretsContext.Environment, req.SecretRef, bundles...)
}

func (l *localSecrets) MissingError(missing []*schema.PackageRef, missingSpecs []*schema.SecretSpec, missingServer []schema.PackageName) error {
	labels := make([]string, len(missing))

	for k, secretRef := range missing {
		labels[k] = fmt.Sprintf("\n  # Description: %s\n  # Server: %s\n  ns secrets set --secret %s", missingSpecs[k].Description, missingServer[k], secretRef.Canonical())
	}

	return fnerrors.UsageError(
		fmt.Sprintf("Please run:\n%s", strings.Join(labels, "\n")),
		"There are secrets required which have not been specified")
}

func (l *localSecrets) loadSecretsFor(ctx context.Context, moduleName, secretFile string) (*secrets.Bundle, error) {
	if strings.Contains(moduleName, ":") {
		return nil, fnerrors.InternalError("module names can't contain colons")
	}

	key := fmt.Sprintf("%s:%s", moduleName, secretFile)

	l.mu.Lock()
	defer l.mu.Unlock()

	if existing, ok := l.cache[key]; ok {
		return existing, nil
	}

	fsys := l.moduleFS(moduleName)
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

func (l *localSecrets) moduleFS(moduleName string) fs.FS {
	for _, mod := range l.modules.Modules() {
		if mod.ModuleName() == moduleName {
			return mod.ReadOnlyFS()
		}
	}

	return nil
}

func loadSecretsFile(ctx context.Context, keyDir fs.FS, name string, fsys fs.FS, sourceFile string) (*secrets.Bundle, error) {
	contents, err := fs.ReadFile(fsys, sourceFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fnerrors.InternalError("%s: failed to read %q: %w", name, sourceFile, err)
	}

	return secrets.LoadBundle(ctx, keyDir, contents)
}

func lookupSecret(ctx context.Context, env *schema.Environment, secretRef *schema.PackageRef, lookup ...*secrets.Bundle) (*schema.FileContents, error) {
	key := &secrets.ValueKey{PackageName: secretRef.PackageName, Key: secretRef.Name, EnvironmentName: env.Name}

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
