import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
)

extension: fn.#Extension & {}

$env:           inputs.#Environment
$grafanaServer: inputs.#Server & {
	packageName: "namespacelabs.dev/foundation/std/monitoring/grafana/server"
}
$tool: inputs.#Package & "namespacelabs.dev/foundation/std/monitoring/grafana/tool"

configure: fn.#Configure & {
	if $env.runtime == "kubernetes" && !$env.ephemeral {
		stack: {
			append: [$grafanaServer]
		}

		with: binary: $tool
	}
}
