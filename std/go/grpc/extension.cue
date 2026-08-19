import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
)

extension: fn.#Extension & {
	import: [
		"namespacelabs.dev/foundation/std/core",
		"namespacelabs.dev/foundation/std/go/grpc/metrics",
		"namespacelabs.dev/foundation/std/go/http",
	]
}

$inputs: {
	serverPort: inputs.#Port & {
		name: "server-port"
	}
	grpcPort: inputs.#Port & {
		name: "grpc-port"
	}
}

configure: fn.#Configure & {
	startup: {
		args: {
			listen_hostname:           "0.0.0.0"
			grpcserver_port:           "\($inputs.serverPort.port)"
			grpcserver_plaintext_port: "\($inputs.grpcPort.port)"
		}

		env: {
			// go-grpc started requiring ALPN in TLS handshakes with go-grpc 1.67.0
			// (https://github.com/grpc/grpc-go/pull/7535), but not all external
			// clients send it (notably many buildkit clients don't).
			// This instructs go-grpc to not enforce it for now.
			GRPC_ENFORCE_ALPN_ENABLED: "false"
		}
	}
}
