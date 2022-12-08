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

$workspace: inputs.#Workspace
$tool:      inputs.#Package & "namespacelabs.dev/foundation/std/secrets/kubernetes"

configure: fn.#Configure & {
	if $env.runtime == "kubernetes" {
		// Corresponding secrets are instantiated in the target Kubernetes cluster,
		// driven by the local configuration.
		with: {
			binary: $tool
			snapshot: {
				serverSecrets: {
					// XXX we need a validation step that is more understandable to users.
					fromWorkspace: path.Join([$workspace.serverPath, "server.secrets"])
					optional:      true
					requireFile:   true
				}
			}
			noCache:      true // We don't want secret values to end up in the cache.
			requiresKeys: true // This is temporary while we don't pipe a keys service to tools.
		}
	}
}
