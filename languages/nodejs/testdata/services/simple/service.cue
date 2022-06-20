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

	instantiate: {
		cert: secrets.#Exports.Secret & {
			name: "cert"
		}
  }

	exportService:        $proto.services.PostService
	exportServicesAsHttp: true
}
