server: {
	name: "myserver"

	integration: "dockerfile"

	env: {
		NAME: "\($env.name)-Bob"
		SECRET: fromSecret:            "namespacelabs.dev/foundation/internal/testdata/integrations/dockerfile/complex:key1"
		ENDPOINT: fromServiceEndpoint: "namespacelabs.dev/foundation/internal/testdata/integrations/dockerfile/simple:webapi"
		XYZ: fromResourceField: {
			resource: "namespacelabs.dev/foundation/internal/testdata/integrations/resources/instances:test1"
			fieldRef: "url"
		}
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

	requires: [
		"namespacelabs.dev/foundation/internal/testdata/integrations/dockerfile/simple",
	]
}

tests: {
	// TODO: fix a k8s error when a test name is too long.
	hello: {
		integration: dockerfile: "test/Dockerfile"
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
