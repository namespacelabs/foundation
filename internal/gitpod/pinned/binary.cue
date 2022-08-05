binary: {
	name: "nspinnedgitpod"

	build_plan: {
		layer_build_plan: [
			{dockerfile: "Dockerfile.baseimage"},
			{go_build: {
				rel_path:    "../../../cmd/ns"
				binary_name: "ns"
				binary_only: true
			}},
		]
	}
}
