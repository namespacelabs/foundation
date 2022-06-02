// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import { JaegerExporter } from "@opentelemetry/exporter-jaeger";
import { BatchSpanProcessor } from "@opentelemetry/sdk-trace-base";
import yargs from "yargs";
import { ExtensionDeps } from "./deps.fn";

const argv = yargs(process.argv.slice(2))
	.options({
		jaeger_collector_endpoint: { type: "string" },
	})
	.parse();

export const initialize = (deps: ExtensionDeps) => {
	deps.openTelemetry.addSpanProcessor(
		new BatchSpanProcessor(
			new JaegerExporter({
				endpoint: argv.jaeger_collector_endpoint,
			})
		)
	);
	console.warn(`Jaeger extension is initialized. Server: ${argv.jaeger_collector_endpoint}.`);
};
