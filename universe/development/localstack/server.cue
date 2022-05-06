import (
	"namespacelabs.dev/foundation/std/fn"
)

server: fn.#OpaqueServer & {
	id:   "une8l4h8muitei49gcn0"
	name: "localstack"

	isStateful: true

	// TODO pin the localstack version.
	binary: image: "localstack/localstack"

	import: [
		"namespacelabs.dev/foundation/universe/development/localstack/configure",
	]

	// Export the service so it its endpoint is discoverable by clients.
	service: "api": {
		containerPort: 4566
		metadata: {
			// TODO this is required but does not seem to be used
			protocol: "dummy string"
		}
	}
}
