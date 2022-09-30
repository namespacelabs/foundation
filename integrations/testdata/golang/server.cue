server: {
	name: "myserver"

	integration: "go"

	env: {
		NAME: "\($env.name)-Bob"
	}

	services: {
		webapi: {
			port: 4000
			kind: "http"

			ingress: internetFacing: true
		}
	}
}
