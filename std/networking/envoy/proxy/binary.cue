binary: {
	name: "envoyproxy"
	from: dockerfile: "Dockerfile"
	config: {
		command: ["/entrypoint.sh"]
	}
}
