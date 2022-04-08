binary: {
	name: "tailscale"
	from: dockerfile: "Dockerfile"
	config: {
		command: ["/entrypoint.sh"]
	}
}
