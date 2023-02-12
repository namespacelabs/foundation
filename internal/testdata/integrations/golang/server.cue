server: {
	name: "myserver"

	integration: "go"

	env: {
		NAME: "\($env.name)-Bob"
	}

	annotations: {
		"foobar.com": fromField: {
			instance: fromService: ":webapi"
			fieldRef: "owner"
		}
	}

	services: {
		webapi: {
			port: 4000
			kind: "http"

			ingress: true
		}
	}
}
