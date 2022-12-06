server: {
	name: "myserver"

	integration: "nodejs"

	env: {
		NAME: "\($env.name)-Bob"
	}

	services: {
		webapi: {
			port: 4000
			kind: "http"

			ingress: true

			probe: http: "/"
		}
	}
}
