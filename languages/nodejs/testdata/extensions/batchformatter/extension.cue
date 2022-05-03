import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/languages/nodejs/testdata/extensions/numberformatter"
)

$providerProto: inputs.#Proto & {
	source: "input.proto"
}

// This extension demonstrates initializes and scoping.
// It instantiates "numberformatter" in different scopes and outputs results.

extension: fn.#Extension & {
	hasInitializerIn: "NODEJS"

  // This is a singleton dependency, the provider function is called at most once for this instantiation.
  instantiate: {
		fmt: numberformatter.#Exports.fmt & {
			precision: 2
		}
  }

	provides: {
		BatchFormatter: {
			input: $providerProto.types.InputData
			availableIn: {
				nodejs: {
          import: "@namespacelabs.dev/foundation_languages_nodejs_testdata_extensions_batchformatter/formatter"
					type:   "BatchFormatter"
				}
			}

      // This is a scoped dependency, the provider function is called for every instantiation.
      instantiate: {
        fmt: numberformatter.#Exports.fmt & {
          precision: 5
        }
      }
		}
	}
}
