binary: {
	name: "fn-postgres-image"
	config: {
		command: ["/fn-postgres-entrypoint.sh"]
	}

	from: llb_go_binary: "."
}
