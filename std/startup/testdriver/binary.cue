binary: {
	name: "startup-test"
	build_plan: {
		layer_build_plan: [
			{alpine_build: {
				version: "sha256:e1c082e3d3c45cccac829840a25941e679c25d438cc8412c2fa221cf1a824e6a"
				package: ["bash"]
			}},
			{snapshot_files: ["test.sh"]}, // XXX update to `files`.
		]
	}
	config: {
		command: ["/bin/bash", "/test.sh"]
	}
}
