import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/testdata/datastore"
	"namespacelabs.dev/foundation/std/grpc"
	"namespacelabs.dev/foundation/std/grpc/deadlines"
)

$proto: inputs.#Proto & {
	source: "../proto/post.proto"
}

service: fn.#Service & {
	framework: "GO"

	instantiate: {
		main: datastore.#Exports.Database & {
			name:       "main"
			schemaFile: inputs.#FromFile & {
				path: "schema.txt"
			}
		}

		dl: deadlines.#Exports.Deadlines & {
			configuration: [
				{serviceName: "PostService", methodName: "*", maximumDeadline: 5.0},
			]
		}

		simple: grpc.#Exports.Backend & {
			packageName: "namespacelabs.dev/foundation/std/testdata/service/simple"
		}
	}

	exportMethods: {
		service: $proto.services.PostService
		methods: ["Post"]
	}
	exportServicesAsHttp: true
	ingress:              "INTERNET_FACING"
}
