server: {
	name:        "bazel"
	integration: "go"

	services: http: {
		port: 4000
		kind: "http"

		ingress: true
	}
}
