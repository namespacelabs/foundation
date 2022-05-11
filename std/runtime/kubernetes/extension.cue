import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
)

$typesProto: inputs.#Proto & {
	source: "types.proto"
}

extension: fn.#Extension & {
	provides: {
		ServerExtension: {
			input: $typesProto.types.ServerExtensionArgs
		}
	}

	on: {
		prepare: {
			invokeInternal: "namespacelabs.dev/foundation/std/runtime/kubernetes.ApplyServerExtensions"
		}
	}

	packageData: [
		"defaults/container.securitycontext.yaml",
		"defaults/pod.podsecuritycontext.yaml",
	]
}

$env:     inputs.#Environment
$ephCtrl: inputs.#Server & {
	packageName: "namespacelabs.dev/foundation/std/monitoring/grafana/server"
}

configure: fn.#Configure & {
	stack: {
		if $env.ephemeral {
			append: [$ephCtrl]
		}
	}
}
