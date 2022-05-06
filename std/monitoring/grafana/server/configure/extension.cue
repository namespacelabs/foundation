import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
)

extension: fn.#Extension

configure: fn.#Configure & {
	with: binary: "namespacelabs.dev/foundation/std/monitoring/grafana/tool"
}
