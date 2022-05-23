// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import { Registrar } from "@namespacelabs/foundation";
import { WireService } from "./api.fn";

export const wireService: WireService = (registrar: Registrar): void => {
	registrar.http().post("/simple/:userId", async (req) => {
		const params = req.params as any;
		return { output: `Hello world! User ID: ${params["userId"]}` };
	});
};
