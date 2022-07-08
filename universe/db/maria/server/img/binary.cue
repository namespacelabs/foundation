binary: {
	name: "ns-mariadb-image"
	config: {
		command: ["/ns-mariadb-entrypoint.sh"]
	}

	from: llb_plan: {
		output_of: {
			name: "llbgen"
			from: go_package: "."
		}
	}
}
