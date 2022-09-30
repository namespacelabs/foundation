server: {
	name: "myserver"

	integration: "docker"

	env: {
		NAME: "\($env.name)-Bob"
	}

	services: {
		webapi: {
			port: 4000
			kind: "http"

			ingress: {
				internetFacing: true
				httpRoutes: "*": ["/mypath"]
			}
		}
	}

	// XXX work in progress.
	// resources: [
	//  "namespacelabs.dev/foundation/integrations/testdata/resources/instances:withInput",
	// ]
}

tests: {
	// TODO: fix a k8s error when a test name is too long.
	hello: {
		build: docker: dockerfile: "test/Dockerfile"
	}
}
