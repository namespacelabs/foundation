binary: {
	name: "filesfrom"

	build_plan: {
		layer_build_plan: [
			{files_from: {
				from: image_id: "busybox"
				files: ["lib/libc.so.6"]
				target_dir: "wrapped"
			}},
		]
	}
}
