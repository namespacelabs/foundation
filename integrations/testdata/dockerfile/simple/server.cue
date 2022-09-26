server: {
	name: "myserver"

	integration: "docker"

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
