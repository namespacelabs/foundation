import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
)

extension: fn.#Extension & {
	import: [
		"namespacelabs.dev/foundation/std/monitoring/grafana",
	]
}

$env:        inputs.#Environment
$promServer: inputs.#Server & {
	packageName: "namespacelabs.dev/foundation/std/monitoring/prometheus/server"
}

configure: fn.#Configure & {
	stack: {
		append: [$promServer]
	}

	with: {
		binary: "namespacelabs.dev/foundation/std/monitoring/prometheus/tool"
		args: {
			mode: "client"
		}
	}
}
