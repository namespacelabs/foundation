binary: {
	name: "with-parent-context"

	build_plan: {
		layer_build_plan: [
			{docker_build: {context_dir: "../..", dockerfile: "binaries/docker/Dockerfile"}},
		]
	}
}
