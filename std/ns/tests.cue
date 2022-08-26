package ns

import (
	"namespacelabs.dev/foundation/std/fn:inputs"
)

#Tests: {
	[string]: {
		source: string

		// Defaults to current package, if empty.
		serversUnderTest: [...inputs.#Package]

		_#TestIntegration
	}
}

_#TestIntegration: {
	// TODO introduce types when integrations require parameters
	integration: "namespace.so/pkg/testing/go" | "namespace.so/testing/shell-script"
}
