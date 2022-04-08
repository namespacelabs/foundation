package types

#Proto: {
	typename: string
	source: [...string]
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
