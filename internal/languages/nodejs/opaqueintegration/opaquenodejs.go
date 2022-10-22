// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package opaque

import (
	"context"

	"namespacelabs.dev/foundation/internal/hotreload"
	"namespacelabs.dev/foundation/internal/languages"
	"namespacelabs.dev/foundation/internal/languages/opaque"
	"namespacelabs.dev/foundation/internal/planning"
	"namespacelabs.dev/foundation/internal/runtime"
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
