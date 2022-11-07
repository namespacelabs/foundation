server: {
	name: "myserver"

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
