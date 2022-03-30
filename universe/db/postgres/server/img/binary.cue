binary: {
	name:       "postgres"
	repository: "us-docker.pkg.dev/foundation-344819/prebuilts/foundation/universe/db/postgres/server/img"

	from: llb_go_binary: "."
}
