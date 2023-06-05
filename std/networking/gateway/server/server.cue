import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
)

server: fn.#OpaqueServer & {
	id:   "sun4qtee50l61888bdj0"
	name: "gateway"

	binary: image: "envoyproxy/envoy:v1.24.2@sha256:a64ee326eebcaed29118ce15bccd7753e61623f8c42c5ce2905bcb2d0dea47c8"

	service: {
		"admin": {
			label:         "Envoy (admin)"
			containerPort: 19000
			metadata: protocol: "http"
		}

		// Must be consistent with controller's configuration.
		"grpc-http-transcoder": {
			containerPort: 10000
			metadata: protocol: "http"
			internal: true // Not used for development.
		}
	}
}

$collectorServer: inputs.#Server & {
	packageName: "namespacelabs.dev/foundation/std/monitoring/otelcollector"
}

configure: fn.#Configure & {
	stack: {
		append: [$collectorServer]
	}

	with: binary: "namespacelabs.dev/foundation/std/networking/gateway/server/configure"

	sidecar: controller: {
		binary: "namespacelabs.dev/foundation/std/networking/gateway/controller"
	}
}
