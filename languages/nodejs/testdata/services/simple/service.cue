import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/secrets"
)

$proto: inputs.#Proto & {
	source: "service.proto"
}

service: fn.#Service & {
	framework: "NODEJS"

	exportService:        $proto.services.PostService
	exportServicesAsHttp: true

	instantiate: testSecrets: secrets.#Exports.Secret & {
		name: "test-name"
		generate: {
			randomByteCount: 32
			format:          "FORMAT_BASE64"
		}
	}
}
