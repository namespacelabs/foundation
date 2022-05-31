// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import { sendUnaryData, ServerUnaryCall } from "@grpc/grpc-js";
import { Registrar } from "@namespacelabs/foundation";
import { ServiceDeps, WireService } from "./deps.fn";
import { FormatServiceService, IFormatServiceServer } from "./service_grpc_pb";
import { FormatRequest, FormatResponse } from "./service_pb";

export const wireService: WireService = async (deps: ServiceDeps, registrar: Registrar) => {
	const service: IFormatServiceServer = {
		format: function (
			call: ServerUnaryCall<FormatRequest, FormatResponse>,
			callback: sendUnaryData<FormatResponse>
		): void {
			const response: FormatResponse = new FormatResponse();

			const formatResult1 = deps.batch1.getFormatResult(call.request.getInput());
			const formatResult2 = deps.batch2.getFormatResult(call.request.getInput());

			const output = `First instance of the "batchformatter" extension:
  Singleton formatter output: ${formatResult1.singleton}
  Scoped formatter output: ${formatResult1.scoped}
Second instance of the "batchformatter" extension:
  Singleton formatter output: ${formatResult2.singleton}
  Scoped formatter output: ${formatResult2.scoped}`;
			response.setOutputList(output.split("\n"));

			callback(null, response);
		},
	};

	registrar.registerGrpcService(FormatServiceService, service);
};
