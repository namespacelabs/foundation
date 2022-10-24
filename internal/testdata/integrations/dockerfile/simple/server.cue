server: {
	name: "myserver"

	integration: "dockerfile"

	services: {
		webapi: {
			port: 4000
			kind: "http"

			ingress: internetFacing: true
		}
	}
}
