import (
	"path"
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
)

$providerProto: inputs.#Proto & {source: "provider.proto"}

extension: fn.#Extension & {
	provides: {
		Secret: {
			input: $providerProto.types.Secret

			availableIn: {
				go: type: "*Value"
			}
		}
	}
}

$env:       inputs.#Environment
$workspace: inputs.#Workspace
$focus:     inputs.#FocusServer
$tool:      inputs.#Package & "namespacelabs.dev/foundation/std/secrets/kubernetes"

configure: fn.#Configure & {
	if $env.runtime == "kubernetes" {
		// Corresponding secrets are instantiated in the target Kubernetes cluster,
		// driven by the local configuration.
		with: {
			binary: $tool
			snapshot: secrets: {
				// XXX we need a validation step that is more understandable to users.
				fromWorkspace: path.Join([$workspace.serverPath, "secrets"])
			}
			// We don't want secret values to end up in the cache.
			noCache:      true
			requiresKeys: true
		}
	}

	// The required secrets are then mounted to /secrets, where this extension can
	// pick them up. A map.textpb is also synthesized.
	startup: {
		// Only Go/gRPC servers embed our library.
		if $focus.framework == "GO_GRPC" {
			args: {
				simple_secrets_basepath: "/secrets"
			}
		}
	}
}
