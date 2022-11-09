// Separate server for development since `ns dev`, ENV/args, services and filesync are not supported for tests.
server: {
	name: "cypress-dev-server"

	args: ["-c", "/liveserver/supervisord.conf"]

	integration: dockerfile: {
		// Special image for development, contains noVNC, websockify, supervisord, etc.
		src: "liveserver/Dockerfile"
		// Using supervisord to run multiple processes in the same container.
		command: "supervisord"
	}

	services: {
		web: {
			// Exposing the noVNC Web frontend port.
			port: 8080
			kind: "http"

			ingress: internetFacing: true
		}
	}

	// Mounting the local directory into the container.
	mounts: "/app/cypress": {
		syncWorkspace: fromDir: "cypress"
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
