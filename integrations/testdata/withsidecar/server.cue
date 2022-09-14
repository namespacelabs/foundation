server: {
	name: "myserver"

	integration: docker: dockerfile: "Dockerfile"

	services: {
		webapi: {
			port: 4000
			kind: "http"

			ingress: {
				internetFacing: true
				httpRoutes: "*": ["/"]
			}
		}
		mysidecar: {
			port: 4001
			kind: "http"
		}
	}
}

sidecars: {
	sc1: {
		build: {
			with:       "namespace.so/from-dockerfile"
			dockerfile: "mysidecar/Dockerfile"
		}

		env: {
			NAME: "\($env.name)-Mary"
		}
	}
}
