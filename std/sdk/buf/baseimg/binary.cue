binary: {
	name:       "baseimg"
	repository: "us-docker.pkg.dev/foundation-344819/prebuilts/foundation/std/sdk/buf/baseimg"

	from: llb_go_binary: "."
}
