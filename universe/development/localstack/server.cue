import (
	"namespacelabs.dev/foundation/std/fn"
)

server: fn.#OpaqueServer & {
	id:   "une8l4h8muitei49gcn0"
	name: "localstack"

	isStateful: true

	binary: image: "localstack/localstack"

	// Export the service so it its endpoint is discoverable by clients.
	service: "api": {
		containerPort: 4566
		metadata: {
			// TODO this is required but does not seem to be used
			protocol: "dummy string"
		}
	}
}
