import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
  "namespacelabs.dev/foundation/languages/nodejs/testdata/extensions/numberformatter"
)

$proto: inputs.#Proto & {
	source: "service.proto"
}

// A service that uses the "numberformatter" extension.

service: fn.#Service & {
  framework: "NODEJS"
  
  instantiate: {
    fmt: numberformatter.#Exports.fmt & {
      precision: 3
    }
  }

	exportService:        $proto.services.FormatService
}
