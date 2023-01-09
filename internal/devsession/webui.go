// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package devsession

import (
	"context"
	"fmt"

	"github.com/gorilla/mux"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/schema"
)

const (
	WebPackage schema.PackageName = "namespacelabs.dev/foundation/internal/webui/devui"

	baseRepository = "us-docker.pkg.dev/foundation-344819/prebuilts"
	prebuilt       = "sha256:448baeb723f3beab132f0917292d9687297ed94e5e75a75d041d842d539d1c14"
)

func PrebuiltWebUI(ctx context.Context) (*mux.Router, error) {
	image := oci.ImageP(fmt.Sprintf("%s/%s@%s", baseRepository, WebPackage, prebuilt), nil, oci.RegistryAccess{PublicImage: true})

	return compute.GetValue(ctx, serveFS(image, "app/dist/" /* pathPrefix */, true /* spa */))
}
