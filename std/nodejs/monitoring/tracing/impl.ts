// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

import { diag, DiagConsoleLogger, DiagLogLevel } from "@opentelemetry/api";
import { Instrumentation, registerInstrumentations } from "@opentelemetry/instrumentation";
import { Resource } from "@opentelemetry/resources";
import { SpanProcessor } from "@opentelemetry/sdk-trace-base";
import { NodeTracerProvider } from "@opentelemetry/sdk-trace-node";
import { SemanticResourceAttributes } from "@opentelemetry/semantic-conventions";
import { ExporterArgs } from "./types_pb";

const spanProcessors: {
	name: string;
	spanProcessor: SpanProcessor;
}[] = [];

export const provideExporter = (args: ExporterArgs) => {
	return {
		addSpanProcessor(spanProcessor: SpanProcessor) {
			spanProcessors.push({ spanProcessor, name: args.getName() });
		},
	};
};

const instrumentations: Instrumentation[] = [];

export const provideInstrumentationRegistrar = () => {
	return {
		addInstrumentation(instrumentation: Instrumentation) {
			instrumentations.push(instrumentation);
		},
	};
};

export const initialize = () => {
	diag.setLogger(new DiagConsoleLogger(), DiagLogLevel.INFO);

	const provider = new NodeTracerProvider({
		// TODO: set the resource attributes.
		resource: new Resource({
			[SemanticResourceAttributes.SERVICE_NAME]: "My nodejs server",
		}),
	});
	spanProcessors.forEach((p) => {
		console.log(`OpenTelemetry: adding span processor ${p.name}`);
		provider.addSpanProcessor(p.spanProcessor);
	});

	provider.register();

	registerInstrumentations({
		instrumentations: instrumentations,
	});

	console.log(`Initialized OpenTelemetry.`);
};
