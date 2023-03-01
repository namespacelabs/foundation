server: {
	name: "nats-server"

	image: "nats:2.9.14@sha256:f14772ef64c223208b81b1e8ce213f3adc2260dd30517a35a3c0a3534074ac9a"

	args: [
		"-m", "9000",
		"--auth", "$(AUTH_KEY)",
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
