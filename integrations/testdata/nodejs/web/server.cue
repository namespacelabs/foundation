server: {
	name: "myserver"

	integration: "nodejs"

	services: webapi: {
		// Default Vite port
		port: 5173
		kind: "http"

		ingress: internetFacing: true
	}
}
