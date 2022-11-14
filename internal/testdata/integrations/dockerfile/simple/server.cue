server: {
	name: "mysimpleserver"

	command: "npm"
	args: ["start"]

	integration: "dockerfile"

	services: {
		webapi: {
			port: 4000
			kind: "http"

			ingress: internetFacing: true
		}
	}

	mounts: "/app/src": {
		syncWorkspace: fromDir: "src"
	}
}
