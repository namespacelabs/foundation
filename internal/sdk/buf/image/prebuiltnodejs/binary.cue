binary: {
	name: "bufbuild"
	from: llb_plan: {
		output_of: {
			name: "llbgen"
			from: go_package: "."
		}
	}
}
