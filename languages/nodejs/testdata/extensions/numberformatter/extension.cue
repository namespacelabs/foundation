import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
)

$inputProto: inputs.#Proto & {
	source: "input.proto"
}

// Simple extension that allows to format numbers.

extension: fn.#Extension & {  
	provides: {
		fmt: {
			input: $inputProto.types.FormattingSettings
			availableIn: {
				nodejs: {
          import: "@namespacelabs.dev/foundation_languages_nodejs_testdata_extensions_numberformatter/formatter"
					type:   "NumberFormatter"
				}
			}
		}
	}
}
