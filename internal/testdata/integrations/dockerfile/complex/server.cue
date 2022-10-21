server: {
	name: "myserver"

	integration: "docker"

	env: {
		NAME: "\($env.name)-Bob"
		SECRET: fromSecret: "namespacelabs.dev/foundation/internal/testdata/integrations/dockerfile/complex:key1"
	}

	services: {
		webapi: {
			port: 4000
			kind: "http"

			ingress: {
				internetFacing: true
				httpRoutes: "*": ["/mypath"]
			}

			probe: http: "/readyz"
		}
	}

	resources: [
		"namespacelabs.dev/foundation/internal/testdata/integrations/resources/instances:test1",
	]
}

tests: {
	// TODO: fix a k8s error when a test name is too long.
	hello: {
		builder: docker: dockerfile: "test/Dockerfile"
	}
}

secrets: {
	key1: {
		description: "A generated secret, for testing purposes."
		generate: {
			uniqueId:        "myserver-key1"
			randomByteCount: 16
		}
	}
}
