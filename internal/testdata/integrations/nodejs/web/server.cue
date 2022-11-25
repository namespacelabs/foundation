server: {
	name: "myserver"

	integration: web: service: "webapi"

	services: webapi: {
		// Default Vite port
		port: 5173
		kind: "http"

		ingress: true
	}
}

tests: {
	health: {
		integration: shellscript: {
			entrypoint: "test/test.sh"
			requiredPackages: ["jq"]
		}
	}
}
