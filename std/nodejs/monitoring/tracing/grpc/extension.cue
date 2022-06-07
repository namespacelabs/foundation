import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/nodejs/grpc"
  "namespacelabs.dev/foundation/std/nodejs/monitoring/tracing"
)

// Registers Fastify as an Open Telemetry data source.

extension: fn.#Extension & {
	hasInitializerIn: "NODEJS"
	initializeBefore: ["namespacelabs.dev/foundation/std/nodejs/monitoring/tracing"]

	instantiate: {
		grpcInterceptorRegistrar: grpc.#Exports.GrpcInterceptorRegistrar
		tracingRegistrar: tracing.#Exports.InstrumentationRegistrar
	}
}
