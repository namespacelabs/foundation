server: {
	name: "myserver"

	integration: docker: dockerfile: "Dockerfile"

	env: {
		NAME: "\($env.name)-Bob"
	}

	services: {
		webapi: {
			port: 4000
			kind: "http"

			ingress: {
				internetFacing: true
				httpRoutes: "*": ["/"]
			}
		}
	}
}

tests: {
	// TODO: fix a k8s error when a test name is too long.
	hello: {
		build: docker: dockerfile: "test/Dockerfile"
	}
}
