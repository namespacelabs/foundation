binary: {
	name: "nspinnedgitpod"

	build_plan: {
		layer_build_plan: [
			{go_build: {
				rel_path:    "../../../cmd/ns"
				binary_name: "ns"
				binary_only: true
			}},
			{dockerfile: "Dockerfile.baseimage"},
		]
	}
	config: {
		command: ["/setup.sh"]
	}
}
