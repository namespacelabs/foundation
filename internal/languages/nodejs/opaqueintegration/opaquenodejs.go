// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package opaque

import (
	"context"
	"strings"

	"namespacelabs.dev/foundation/internal/fnfs/workspace/wsremote"
	"namespacelabs.dev/foundation/internal/hotreload"
	"namespacelabs.dev/foundation/internal/languages"
	"namespacelabs.dev/foundation/internal/languages/nodejs/binary"
	"namespacelabs.dev/foundation/internal/languages/opaque"
	"namespacelabs.dev/foundation/internal/planning"
	"namespacelabs.dev/foundation/internal/runtime"
	"namespacelabs.dev/foundation/internal/wscontents"
	"namespacelabs.dev/foundation/schema"
)

func Register() {
	languages.Register(schema.Framework_OPAQUE_NODEJS, impl{})
}

type impl struct {
	opaque.OpaqueIntegration
}

func (impl) PrepareDev(ctx context.Context, cluster runtime.ClusterNamespace, srv planning.Server) (context.Context, languages.DevObserver, error) {
	if opaque.UseDevBuild(srv.SealedContext().Environment()) {
		return hotreload.ConfigureFileSyncDevObserver(ctx, cluster, srv)
	}

	return ctx, nil, nil
}

func (impl) PrepareHotReload(ctx context.Context, remote *wsremote.SinkRegistrar, srv planning.Server) *languages.HotReloadOpts {
	if opaque.UseDevBuild(srv.SealedContext().Environment()) {
		return &languages.HotReloadOpts{
			// "ModuleName" and "Rel" are empty because we have only one module in the image and
			// we put the package content directly under the root "/app" directory.
			Sink: remote.For(&wsremote.Signature{ModuleName: "", Rel: ""}),
			EventProcessor: func(ev *wscontents.FileEvent) *wscontents.FileEvent {
				for _, p := range binary.PackageManagerSources {
					if strings.HasPrefix(ev.Path, p) {
						return nil
					}
				}
				return ev
			},
		}
	}

	return nil
}
