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
$proto: inputs.#Proto & {
	source: "service.proto"
}
{{end}}

service: fn.#Service & {
	framework: "{{.Framework}}"

	{{if .ExportedServiceName}}
	exportService: $proto.services.{{.ExportedServiceName}}
	exportServicesAsHttp: true
	ingress:              "INTERNET_FACING"
	{{end}}

	{{if .HttpBackendPkg}}
	instantiate: {
		apiBackend: http.#Exports.Backend & {
			endpointOwner: "{{.HttpBackendPkg}}"
			manager:       "namespacelabs.dev/foundation/std/grpc/httptranscoding"
		}
	}
	{{end}}
}
`))
