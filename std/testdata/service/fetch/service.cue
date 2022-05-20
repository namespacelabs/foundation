import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/testdata/datastore"
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

	exportService:        $proto.services.PostService
	exportServicesAsHttp: true
	ingress:              "INTERNET_FACING"
}
