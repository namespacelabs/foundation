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

	requirePersistentStorage: {
		persistentId: "ns-orchestration-state"
		byteCount:    "1GiB"
		mountPath:    "/namespace/orchestration/data"
	}
}

configure: fn.#Configure & {
	startup: {
		env: {
			"ORCH_VERSION": "2"
			"NSDATA":       "/namespace/orchestration/data"
			"HOME":         "/namespace/orchestration/data/tmphome" // TODO Move to empty dir.
		}
	}
}
