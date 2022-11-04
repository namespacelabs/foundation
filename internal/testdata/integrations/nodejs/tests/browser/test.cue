// Separate server for development since `ns dev`, ENV/args, services and filesync are not supported for tests.
server: {
	name: "cypress-dev-server"

	// "open" wouldn't work without x11 so we use "run --no-exit" instead.
	args: ["run", "--no-exit", "--browser=chrome"]

	env: {
		CYPRESS_REMOTE_DEBUGGING_PORT: "5001"
	}

	integration: dockerfile: {
		command: "cypress"
	}

	services: {
		webapi: {
			// Exposing the Chrome remote debugging port
			port: 5001
			kind: "http"

			ingress: internetFacing: true
		}
	}

	requires: [
		"namespacelabs.dev/foundation/internal/testdata/integrations/nodejs/npm",
	]
}

tests: {
	cypress: {
		integration: "dockerfile"
		serversUnderTest: [
			"namespacelabs.dev/foundation/internal/testdata/integrations/nodejs/npm",
		]
	}
}
