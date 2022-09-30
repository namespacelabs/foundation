server: {
	name: "myserver"

	integration: "docker"

	services: {
		webapi: {
			port: 4000
			kind: "http"

			ingress: internetFacing: true
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

	sc2: {
		image: "redis:6.2.6-alpine@sha256:132337b9d7744ffee4fae83f51de53c3530935ad3ba528b7110f2d805f55cbf5"
	}
}
