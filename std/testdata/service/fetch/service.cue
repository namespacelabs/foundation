import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/grpc"
	"namespacelabs.dev/foundation/std/grpc/deadlines"
)

$proto: inputs.#Proto & {
	source: "../proto/service.proto"
}

service: fn.#Service & {
	framework: "GO_GRPC"

	instantiate: {
		dl: deadlines.#Exports.Deadlines & {
			configuration: [
				{serviceName: "PostService", methodName: "Fetch", maximumDeadline: 0.5},
			]
		}
	}

	exportMethods: {
		service: $proto.services.PostService
		methods: ["Fetch"]
	}
	exportServicesAsHttp: true
	ingress:              "INTERNET_FACING"
}
