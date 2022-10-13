server: {
	name: "myserver"

	integration: "docker"

	env: {
		NAME: "\($env.name)-Bob"
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
		"namespacelabs.dev/foundation/integrations/testdata/resources/instances:test1",
	]
}

tests: {
	// TODO: fix a k8s error when a test name is too long.
	hello: {
		builder: docker: dockerfile: "test/Dockerfile"
	}
}
