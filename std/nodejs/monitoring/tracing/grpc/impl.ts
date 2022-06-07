import {
	InstrumentationBase,
	InstrumentationModuleDefinition,
} from "@opentelemetry/instrumentation";
import { GrpcJsInstrumentation } from ".";
import { ExtensionDeps } from "./deps.fn";

export const initialize = async (deps: ExtensionDeps) => {
	// To wrap:
	// - Server.register
	// - makeGenericClientConstructor
	// - makeClientConstructor
	// - loadPackageDefinition
	const grpc = deps.grpcInterceptorRegistrar;
	const tracing = deps.tracingRegistrar;

	const instrumentation = new GrpcJsInstrumentation("grpc", "0.0.0");
	tracing.addInstrumentation(instrumentation);

	console.log(`Initialized OpenTelemetry instrumentation for the gRPC server.`);
};

// export class GrpcJsInstrumentation extends InstrumentationBase {
// 	constructor(name: string, version: string) {
// 		super(name, version, { enabled: true });
// 	}

// 	protected init():
// 		| void
// 		| InstrumentationModuleDefinition<any>
// 		| InstrumentationModuleDefinition<any>[] {
// 		throw new Error("Method not implemented.");
// 	}
// }
