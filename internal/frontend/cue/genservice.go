// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package cue

import (
	"context"
	"text/template"

	"namespacelabs.dev/foundation/internal/fnfs"
	"namespacelabs.dev/foundation/schema"
)

const (
	serviceFileName = "service.cue"
)

type GenServiceOpts struct {
	ExportedServiceName string
	Framework           schema.Framework
	HttpBackendPkg      string
}

func CreateServiceScaffold(ctx context.Context, fsfs fnfs.ReadWriteFS, loc fnfs.Location, opts GenServiceOpts) error {
	return generateCueSource(ctx, fsfs, loc.Rel(serviceFileName), serviceTmpl, opts)
}

var serviceTmpl = template.Must(template.New(serviceFileName).Parse(`
import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	{{if .HttpBackendPkg -}}
	"namespacelabs.dev/foundation/std/web/http"
	{{end -}}
)

{{if .ExportedServiceName}}
// Load the protobuf definition so its contents are available to Namespace.
$proto: inputs.#Proto & {
	source: "service.proto"
}
{{end}}

// Declare a new service, see also https://docs.namespacelabs.com/concepts/service
service: fn.#Service & {
	framework: "{{.Framework}}"

	{{if .ExportedServiceName}}
	// Export a grpc-based API, defined within service.proto.
	exportService: $proto.services.{{.ExportedServiceName}}

	// Make each of the service methods also available as HTTP endpoints.
	exportServicesAsHttp: true

	// Make this service available to the public Internet.
	// See also https://docs.namespacelabs.com/guides/internet-facing/
	ingress:              "INTERNET_FACING"
	{{end}}

	{{if .HttpBackendPkg}}
	instantiate: {
		// Wire the API backend's configuration (e.g. public address) automatically.
		apiBackend: http.#Exports.Backend & {
			endpointOwner: "{{.HttpBackendPkg}}"
			manager:       "namespacelabs.dev/foundation/std/grpc/httptranscoding"
		}
	}
	{{end}}
}
`))
