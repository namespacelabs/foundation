import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
)

extension: fn.#Extension & {}

$grafanaServer: inputs.#Server & {
	packageName: "namespacelabs.dev/foundation/std/monitoring/grafana/server"
}

configure: fn.#Configure & {
	if $env.runtime == "kubernetes" && !$env.ephemeral {
		stack: {
			append: [$grafanaServer]
		}
	}
}
