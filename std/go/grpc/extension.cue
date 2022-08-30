import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/secrets"
)

extension: fn.#Extension & {
	import: [
		"namespacelabs.dev/foundation/std/core",
		"namespacelabs.dev/foundation/std/go/grpc/metrics",
	]

	hasInitializerIn: "GO"

	instantiate: {
		tlsCert: secrets.#Exports.Secret & {
			name: "grpc-tls-cert"
			generate: {
				uniqueId: "jvkrj3pn" // Without a unique ID, one is generated for us that depends on the allocation tree.
			}
			selfSignedTlsCertificate: {
				organization: ["Namespace Labs, Inc"]
				commonNamePrefix: "Namespace"
			}
		}
	}
}

$inputs: {
	serverPort: inputs.#Port & {
		name: "server-port"
	}
}

configure: fn.#Configure & {
	startup: {
		args: {
			listen_hostname: "0.0.0.0"
			port:            "\($inputs.serverPort.port)"
		}
	}
}
