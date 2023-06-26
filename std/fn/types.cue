package types

#Proto: {
	typename: string
	source: [...string]
	skip_codegen?: bool
}

#ImageID: {
	repository?: string
	tag?:        string
	digest?:     string
}

#Resource: {
	path:     string
	contents: bytes
}

#Any: {
	typeUrl?: string
	value?:   bytes
}
