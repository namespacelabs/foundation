// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package devworkflow

import (
	"context"
	"fmt"

	"github.com/gorilla/mux"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/languages/web"
	"namespacelabs.dev/foundation/schema"
	"namespacelabs.dev/foundation/workspace/compute"
)

const (
	WebPackage schema.PackageName = "namespacelabs.dev/foundation/devworkflow/web"

	baseRepository = "us-docker.pkg.dev/foundation-344819/prebuilts"
	prebuilt       = "sha256:0fe87b9abe9e86b7513120dad83504d98c449175ace68269caf344b1edeabb68"
)

func PrebuiltWebUI(ctx context.Context) (*mux.Router, error) {
	image := oci.ImageP(fmt.Sprintf("%s/%s@%s", baseRepository, WebPackage, prebuilt), nil, oci.ResolveOpts{PublicImage: true})

	return compute.GetValue(ctx, web.ServeFS(image, true))
}
