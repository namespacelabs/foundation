package buildkit

var ForceEstargz bool

func MaybeForceEstargz(src map[string]string) map[string]string {
	if !ForceEstargz {
		return src
	}

	src["compression"] = "estargz"
	src["oci-mediatypes"] = "true"
	src["force-compress"] = "true"
	return src
}
