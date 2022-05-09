// XXX this package should be a web node instead.
binary: {
	name:       "webui"
	repository: "us-docker.pkg.dev/foundation-344819/prebuilts/foundation/devworkflow/web"

	from: web_build: "."
}
