binary: {
	name: "imageutil"
	build_plan: {
		layer_build_plan: [
			{prebuilt: "ubuntu:22.04"},
			{go_build: {
				rel_path:    "."
				binary_name: "bake"
				binary_only: true
			}},
		]
	}
}
