import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
)

extension: fn.#Extension

$tool: inputs.#Package & "namespacelabs.dev/foundation/std/monitoring/grafana/tool"

configure: fn.#Configure & {
	with: binary: $tool
}
