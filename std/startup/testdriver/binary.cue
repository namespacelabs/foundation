binary: {
	name: "startup-test"
	build_plan: {
		layer_build_plan: [
			{alpine_build: {package: ["bash"]}},
			{snapshot_files: ["test.sh"]},
		]
	}
	config: {
		command: ["/bin/bash", "/test.sh"]
	}
}
