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
$tool: inputs.#Package & "namespacelabs.dev/foundation/std/monitoring/prometheus/tool"

configure: fn.#Configure & {
	if $env.runtime == "kubernetes" {
		stack: {
			append: [$promServer]
		}

		with: {
			binary: $tool
			args: {
				mode: "client"
			}
		}
	}
}
