server: {
	name: "nats-server"

	image: "nats:2.9.14-alpine3.17@sha256:563571e1ce1bf17367bf80f61526381dac15e299ffb2e54f14dc242b1b8a8e70"

	args: [
		"/usr/local/bin/nats-server",
		"-m", "9000",
		"--auth", "$AUTH_KEY",
	]

	env: {
		AUTH_KEY: fromSecret: ":authKey"
	}

	services: {
		nats: {
			port: 4222
		}
		console: {
			port: 9000
			kind: "http"
			probe: http: "/healthz"
		}
	}
}

secrets: {
	authKey: {
		description: "Generated auth key"
		generate: {
			uniqueId:        "nats-simple-auth-key"
			randomByteCount: 32
			format:          "FORMAT_BASE32"
		}
	}
}
