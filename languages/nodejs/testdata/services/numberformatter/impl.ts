// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import { GrpcRegistrar } from "@namespacelabs.dev-foundation/std-nodejs-grpc";
import { ServiceDeps, WireService } from "./deps.fn";
import { FormatServiceServer, bindFormatServiceServer } from "./service_grpc.fn";
import { FormatRequest, FormatResponse } from "./service_pb";

export const wireService: WireService = async (deps: ServiceDeps, registrar: GrpcRegistrar) => {
	const bf1 = await deps.batch1;
	const bf2 = await deps.batch2;

	const service: FormatServiceServer = {
		format: async (request: FormatRequest): Promise<FormatResponse> => {
			const response: FormatResponse = new FormatResponse();

			const formatResult1 = bf1.getFormatResult(request.getInput());
			const formatResult2 = bf2.getFormatResult(request.getInput());

			const output = `First instance of the "batchformatter" extension:
  Singleton formatter output: ${formatResult1.singleton}
  Scoped formatter output: ${formatResult1.scoped}
Second instance of the "batchformatter" extension:
  Singleton formatter output: ${formatResult2.singleton}
  Scoped formatter output: ${formatResult2.scoped}`;
			response.setOutputList(output.split("\n"));

			return response;
		},
	};

	registrar.registerGrpcService(bindFormatServiceServer(service));
};
