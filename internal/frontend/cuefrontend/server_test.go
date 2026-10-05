// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package cuefrontend

import (
	"context"
	"os"
	"testing"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/build"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/token"
	"github.com/stretchr/testify/assert"
	"namespacelabs.dev/foundation/internal/frontend/fncue"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/std/pkggraph"
)

func TestTypedServerResources(t *testing.T) {
	buildCtx := build.NewContext()
	imports := map[string]*build.Instance{}
	for pkg, file := range map[string]string{"fn": "foundation", "inputs": "inputs", "types": "types"} {
		instance := buildCtx.NewInstance("", func(_ token.Pos, path string) *build.Instance {
			return imports[path]
		})
		filename := "../../../std/fn/" + file + ".cue"
		data, err := os.ReadFile(filename)
		if !assert.NoError(t, err) || !assert.NoError(t, instance.AddFile(filename, data)) {
			return
		}
		imports["namespacelabs.dev/foundation/std/fn:"+pkg] = instance
	}
	ctx := cuecontext.New()
	definitions := ctx.BuildInstance(imports["namespacelabs.dev/foundation/std/fn:fn"])
	if !assert.NoError(t, definitions.Err()) {
		return
	}

	for _, tt := range []struct {
		name     string
		fields   string
		features []string
		wantErr  string
		requests *schema.Container_ResourceLimits
		selector []*schema.NodeSelectorItem
	}{
		{name: "omitted"},
		{
			name: "requests without feature gate", fields: `resourceRequests: {cpu: "8", memory: "48Gi"}`,
			requests: &schema.Container_ResourceLimits{Cpu: "8", Memory: "48Gi"},
		},
		{
			name: "requests and selectors", features: []string{"experimental/container/nodeSelector"},
			fields: `resourceRequests: {cpu: "8", memory: "48Gi"}
				nodeSelector: {"namespace.systems/network-bandwidth-gbps": "200", "kubernetes.io/arch": "amd64"}`,
			requests: &schema.Container_ResourceLimits{Cpu: "8", Memory: "48Gi"},
			selector: []*schema.NodeSelectorItem{
				{Key: "kubernetes.io/arch", Value: "amd64"},
				{Key: "namespace.systems/network-bandwidth-gbps", Value: "200"},
			},
		},
		{
			name: "selector requires feature", fields: `nodeSelector: {"namespace.systems/network-bandwidth-gbps": "200"}`,
			wantErr: `feature "experimental/container/nodeSelector" is not enabled`,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			v := &fncue.CueV{Val: ctx.CompileString(`server: #Server & {
				id: "q7mi8f5h2ps4j6u9k0dg"
				name: "storage"
				framework: "GO"
				`+tt.fields+`
			}`, cue.Scope(definitions))}
			if !assert.NoError(t, v.Val.Err()) {
				return
			}
			mod := pkggraph.NewModule(&schema.Workspace{
				ModuleName: "example.com", EnabledFeatures: tt.features,
			}, &schema.Workspace_LoadedFrom{AbsPath: t.TempDir()}, "")
			server, _, err := parseCueServer(context.Background(), nil, mod.MakeLocation("storage"), v, v.LookupPath("server"))
			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, tt.requests, server.Self.MainContainer.Requests)
			assert.Equal(t, tt.selector, server.Self.NodeSelector)
		})
	}
}
