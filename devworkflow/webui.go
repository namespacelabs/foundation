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
	prebuilt       = "sha256:ad4b46877bbbdfd95c1a11c06417495afe30e8271edd73acd5fc9d43aedeaedf"
)

func PrebuiltWebUI(ctx context.Context) (*mux.Router, error) {
	image := oci.ImageP(fmt.Sprintf("%s/%s@%s", baseRepository, WebPackage, prebuilt), nil, false)

	return compute.GetValue(ctx, web.ServeFS(image, true))
}
