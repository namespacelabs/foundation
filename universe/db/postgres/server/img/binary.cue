binary: {
	name: "ns-postgres-image"
	config: {
		command: ["/ns-postgres-entrypoint.sh"]
	}

	from: llb_plan: {
		output_of: {
			name: "llbgen"
			from: go_package: "."
		}
	}
}
