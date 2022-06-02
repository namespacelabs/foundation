binary: {
	name: "fn-postgres-image"
	config: {
		command: ["/fn-postgres-entrypoint.sh"]
	}

	from: llb_plan: {
		output_of: {
			name: "llbgen"
			from: go_package: "."
		}
	}
}
