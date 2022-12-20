server: {
	name: "mycomplexserver"

	integration: "dockerfile"

	env: {
		NAME: "\($env.name)-Bob"
		SECRET: fromSecret:            ":key1"
		ENDPOINT: fromServiceEndpoint: "namespacelabs.dev/foundation/internal/testdata/integrations/dockerfile/simple:webapi"
		XYZ: fromResourceField: {
			resource: "namespacelabs.dev/foundation/internal/testdata/integrations/resources/instances:test1"
			fieldRef: "url"
		}
		if $env.purpose != "PRODUCTION" {
			INGRESS: fromServiceIngress: ":webapi"
		}
	}

	services: {
		webapi: {
			port: 4000
			kind: "http"

			ingress: {
				httpRoutes: "*": ["/mypath"]
			}

			probe: http: "/readyz"
		}
	}

	resources: [
		"namespacelabs.dev/foundation/internal/testdata/integrations/resources/instances:test1",
		"namespacelabs.dev/foundation/internal/testdata/integrations/resources/instances:test2",
	]

	requires: [
		"namespacelabs.dev/foundation/internal/testdata/integrations/dockerfile/simple",
	]
}

tests: {
	// TODO: fix a k8s error when a test name is too long.
	hello: {
		integration: shellscript: "test/test.sh"
		env: {
			ENDPOINT: fromServiceEndpoint: ":webapi"
			if $env.purpose != "PRODUCTION" {
				INGRESS: fromServiceIngress: ":webapi"
			}
		}
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
