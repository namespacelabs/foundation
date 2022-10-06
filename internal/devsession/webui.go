// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package devsession

import (
	"context"
	"fmt"

	"github.com/gorilla/mux"
	"namespacelabs.dev/foundation/engine/compute"
	"namespacelabs.dev/foundation/internal/artifacts/oci"
	"namespacelabs.dev/foundation/languages/web"
	"namespacelabs.dev/foundation/schema"
)

const (
	WebPackage schema.PackageName = "namespacelabs.dev/foundation/internal/webui/devsession"

	baseRepository = "us-docker.pkg.dev/foundation-344819/prebuilts"
	prebuilt       = "sha256:13d961041080faed5cebac4ccfe2ccfa9302de115ea9303ea768bf580982bf90"
)

func PrebuiltWebUI(ctx context.Context) (*mux.Router, error) {
	image := oci.ImageP(fmt.Sprintf("%s/%s@%s", baseRepository, WebPackage, prebuilt), nil, oci.ResolveOpts{PublicImage: true})

	return compute.GetValue(ctx, web.ServeFS(image, true))
}
