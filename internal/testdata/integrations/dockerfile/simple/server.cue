server: {
	name: "mysimpleserver"

	args: ["start"]

	integration: dockerfile: {
		command: "npm"
	}

	services: {
		webapi: {
			port: 4000
			kind: "http"

			ingress: internetFacing: true
		}
	}
}
