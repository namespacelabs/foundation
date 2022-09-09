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
