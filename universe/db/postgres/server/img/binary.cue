binary: {
	name: "fn-postgres-image"
	config: {
		command: ["/fn-postgres-entrypoint.sh"]
	}
	repository: "us-docker.pkg.dev/foundation-344819/prebuilts/foundation/universe/db/postgres/server/img"

	from: llb_go_binary: "."
}
