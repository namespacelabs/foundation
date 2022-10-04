binary: {
	name: "nspipelines"

	build_plan: {
		layer_build_plan: [
			{dockerfile: "Dockerfile.baseimage"},
			{go_build: {
				rel_path:    "../nsboot"
				binary_name: "ns"
				binary_only: true
			}},
			{go_build: {
				rel_path:    "."
				binary_name: "nspipelines"
				binary_only: true
			}},
		]
	}
}
