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
}

$env:  inputs.#Environment
$ctrl: inputs.#Server & {
	packageName: "namespacelabs.dev/foundation/std/runtime/kubernetes/controller"
}

configure: fn.#Configure & {
	stack: {
		if $env.ephemeral || $env.purpose == "DEVELOPMENT" {
			append: [$ctrl]
		}
	}
}
