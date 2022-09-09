server: {
	name: "myserver"

	integration: {
		kind:       "namespace.so/from-dockerfile"
		dockerfile: "Dockerfile"
	}

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
