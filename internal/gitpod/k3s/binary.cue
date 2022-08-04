binary: {
	name: "nsgitpod-k3s"

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
