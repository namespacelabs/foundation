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
		prometheus: {
			port: 7777
			kind: "http"
		}
	}

	annotations: {
		"prometheus.io/scrape": "true"
		"prometheus.io/path":   "/metrics"
		"prometheus.io/port":   "7777"
	}

	sidecars: {
		metrics: {
			image: "natsio/prometheus-nats-exporter@sha256:bce728062c4f2bcdb2cfa8f22266efa70122ff6c51c02e04f3a56eafd1bea001"

			args: [
				"-connz", "-routez", "-subz", "-varz", "-prefix=nats", "-use_internal_server_id", "http://localhost:9000",
			]
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
