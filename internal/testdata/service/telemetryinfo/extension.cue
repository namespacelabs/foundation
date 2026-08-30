import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/core/info"
)

$proto: inputs.#Proto & {
	source: "service.proto"
}

service: fn.#Service & {
	framework:     "GO"
	exportService: $proto.services.TelemetryInfoService
	instantiate: {
		serverInfo: info.#Exports.ServerInfo
	}
}
