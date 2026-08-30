binary: {
	name: "docker-credential-nsc"

	build_plan: [
		{go_build: {
			binary_name: "docker-credential-nsc"
			binary_only: true
		}},
	]
}
