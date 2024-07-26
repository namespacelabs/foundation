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
	// ns build-binary --base_repository=registry.eu-services.namespace.systems internal/webui/devui --env prod
	WebPackage schema.PackageName = "namespacelabs.dev/foundation/internal/webui/devui"

	baseRepository = "registry.eu-services.namespace.systems"
	prebuilt       = "sha256:c2dd09514559903b54bc4f79cf018e21bc8e697bcdd50194bc60a633a17abf95"
)

func PrebuiltWebUI(ctx context.Context) (*mux.Router, error) {
	image := oci.ImageP(fmt.Sprintf("%s/%s@%s", baseRepository, WebPackage, prebuilt), nil, oci.RegistryAccess{PublicImage: true})

	return compute.GetValue(ctx, serveFS(image, "app/dist/" /* pathPrefix */, true /* spa */))
}
