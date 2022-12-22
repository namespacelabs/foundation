package templates

#Server: {
	spec: {
		image:          *"localstack/localstack@sha256:4ebb75b927bcfc9a79c77075734e51ce6031054b776eed2defcb3c0dfa4cf699" | string
		ingress:        *false | bool
		dataVolumeSize: *"10GiB" | string
		dataVolume:     *{
			id:   "localstack-server-data"
			size: dataVolumeSize
		} | {
			id:   string
			size: string
		}
	}

	name: "localstack-server"

	image: spec.image

	// Localstack requires a stateful deployment (more conservative update strategy).
	class: "stateful"

	services: {
		"api": {
			port: 4566
			kind: "http"
		}
	}
	if spec.ingress {
		services: "api": ingress: true
	}

	env: {
		DATA_DIR: "/localstack/data/data"
	}

	mounts: {
		"/localstack/data": persistent: spec.dataVolume
	}
}
