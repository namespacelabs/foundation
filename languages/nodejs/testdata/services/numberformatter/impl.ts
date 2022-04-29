// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import { sendUnaryData, Server, ServerUnaryCall } from "@grpc/grpc-js";
import { ServiceDeps, WireService } from "./deps.fn";
import { IFormatServiceServer, FormatServiceService } from "./service_grpc_pb";
import { FormatRequest, FormatResponse } from "./service_pb";

export const wireService: WireService = (deps: ServiceDeps, server: Server): void => {
	const service: IFormatServiceServer = {
		format: function (
			call: ServerUnaryCall<FormatRequest, FormatResponse>,
			callback: sendUnaryData<FormatResponse>
		): void {
			const response: FormatResponse = new FormatResponse();
			response.setOutput(deps.fmt.formatNumber(call.request.getInput()));

			callback(null, response);
		},
	};

	server.addService(FormatServiceService, service);
};
