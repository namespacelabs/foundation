binary: {
	name: "envoy"
	from: dockerfile: "Dockerfile"
	config: {
		command: ["/entrypoint.sh"]
	}
}
