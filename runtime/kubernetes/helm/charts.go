// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package helm

import (
	"context"
	"fmt"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"namespacelabs.dev/foundation/internal/artifacts"
	"namespacelabs.dev/foundation/internal/artifacts/download"
	"namespacelabs.dev/foundation/internal/bytestream"
	"namespacelabs.dev/foundation/internal/compute"
	"namespacelabs.dev/foundation/schema"
)

func Chart(repo, name, version string, digest schema.Digest) compute.Computable[*chart.Chart] {
	d := download.URL(artifacts.Reference{
		URL:    fmt.Sprintf("%s/charts/%s-%s.tgz", repo, name, version),
		Digest: digest,
	})

	return compute.Transform("load chart", d, func(ctx context.Context, b bytestream.ByteStream) (*chart.Chart, error) {
		reader, err := b.Reader()
		if err != nil {
			return nil, err
		}
		defer reader.Close()
		return loader.LoadArchive(reader)
	})
}
