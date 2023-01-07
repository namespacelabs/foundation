import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
)

$proto: inputs.#Proto & {
	source: "../proto/service.proto"
}

service: fn.#Service & {
	framework:     "GO"
	exportService: $proto.services.OrchestrationService

	mounts: {
		"/namespace/orchestration/data": ephemeral: {}
		"/namespace/orchestration/home": ephemeral: {}
	}
}

configure: fn.#Configure & {
	startup: {
		env: {
			"NSDATA": "/namespace/orchestration/data"
			"HOME":   "/namespace/orchestration/home"
		}
	}
}
