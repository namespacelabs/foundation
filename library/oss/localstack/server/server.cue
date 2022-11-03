server: {
	name: "localstack-server"

	image: "localstack/localstack@sha256:4ebb75b927bcfc9a79c77075734e51ce6031054b776eed2defcb3c0dfa4cf699"

	// Localstack requires a stateful deployment (more conservative update strategy).
	class: "stateful"

	services: {
		"api": {
			port: 4566
			kind: "http"
		}
	}
}
