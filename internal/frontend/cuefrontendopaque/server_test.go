// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cuefrontendopaque

import (
	"context"
	"testing"

	"cuelang.org/go/cue/cuecontext"
	"github.com/stretchr/testify/assert"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func TestExtensionFSGroupChangePolicy(t *testing.T) {
	for _, tt := range []struct {
		name, fields string
		enabled      bool
		want         schema.ServerFragment_FSGroupChangePolicy
		wantErr      string
	}{
		{name: "default"},
		{name: "always", fields: `fsGroupChangePolicy: "Always"`, enabled: true, want: schema.ServerFragment_ALWAYS},
		{name: "root mismatch", fields: `fsGroupChangePolicy: "OnRootMismatch"`, enabled: true, want: schema.ServerFragment_ON_ROOT_MISMATCH},
		{name: "requires feature", fields: `fsGroupChangePolicy: "OnRootMismatch"`, wantErr: `feature "experimental/container/fsGroupChangePolicy" is not enabled`},
		{name: "invalid policy", fields: `fsGroupChangePolicy: "OnRootMismtach"`, enabled: true, wantErr: `unsupported fsGroupChangePolicy`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			workspace := &schema.Workspace{ModuleName: "example.com"}
			if tt.enabled {
				workspace.EnabledFeatures = []string{"experimental/container/fsGroupChangePolicy"}
			}
			mod := pkggraph.NewModule(workspace, &schema.Workspace_LoadedFrom{AbsPath: t.TempDir()}, "")
			partial := &fncue.Partial{
				CueV:    fncue.CueV{Val: cuecontext.New().CompileString("extension: {" + tt.fields + "}")},
				Package: fncue.CuePackage{PackageContents: &fncue.PackageContents{}},
			}
			pkg, err := NewFrontend(nil, nil).ParsePackage(context.Background(), partial, mod.MakeLocation("storage"))
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			if assert.NoError(t, err) {
				assert.Equal(t, tt.want, pkg.ServerFragment.FsGroupChangePolicy)
			}
		})
	}
}
