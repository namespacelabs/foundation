import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation-nodejs-testdata/extensions/batchformatter"
)

$proto: inputs.#Proto & {
	source: "service.proto"
}

// A service that uses the "batchformatter" extension.

service: fn.#Service & {
	framework: "NODEJS"

	// Instantiating the "batchformatter" extension twice to show the scopes of its dependencies.
	instantiate: {
		batch1: batchformatter.#Exports.BatchFormatter
		batch2: batchformatter.#Exports.BatchFormatter
	}

	exportService: $proto.services.FormatService
}
