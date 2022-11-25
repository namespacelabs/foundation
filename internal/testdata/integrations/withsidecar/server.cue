server: {
	name: "myserver"

	integration: "dockerfile"

	services: {
		webapi: {
			port: 4000
			kind: "http"

			ingress: true
		}
		mysidecar: {
			port: 4001
			kind: "http"
		}
	}

	requires: [
		"namespacelabs.dev/foundation/internal/testdata/integrations/dockerfile/simple",
	]
}

sidecars: {
	sc1: {
		imageFrom: {
			// Example of a full syntax form
			kind: "namespace.so/from-dockerfile"
			src:  "mysidecar/Dockerfile"
		}

		env: {
			NAME: "\($env.name)-Mary"
			MAIN_ENDPOINT: fromServiceEndpoint: "namespacelabs.dev/foundation/internal/testdata/integrations/dockerfile/simple:webapi"
		}
	}

	sc2: {
		image: "redis:6.2.6-alpine@sha256:132337b9d7744ffee4fae83f51de53c3530935ad3ba528b7110f2d805f55cbf5"
	}
}
