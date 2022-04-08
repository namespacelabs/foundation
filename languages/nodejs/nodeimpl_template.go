// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package nodejs

import (
	"text/template"
)

type nodeimplTmplOptions struct {
	ServiceServerName, ServiceName, ServiceFileName string
}

var (
	nodeimplTmpl = template.Must(template.New("template").Parse(
		`import { Server } from "@grpc/grpc-js";
import { Deps, WireService } from "./deps.fn";
import { {{.ServiceServerName}}, {{.ServiceName}} } from "./{{.ServiceFileName}}_grpc_pb";

export const wireService: WireService = (_: Deps, server: Server): void => {
	const service: {{.ServiceServerName}} = {
	  // TODO: implement
	};

	server.addService({{.ServiceName}}, service);
};`))
)
