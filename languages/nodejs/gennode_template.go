// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package nodejs

import (
	"text/template"
)

type nodeTmplOptions struct {
	NeedsDepsType bool
}

var (
	serviceTmpl = template.Must(template.New("template").Parse(
		`// This file was automatically generated.
import { Server } from "@grpc/grpc-js";
import * as wire from "./wire";

{{if .NeedsDepsType}}
export interface Deps {
}
{{end}}

export type WireService = (deps: Deps, server: Server) => void;
export const wireService: WireService = wire.wireService;
`))
)
