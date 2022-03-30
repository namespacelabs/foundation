import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/testdata/go/datastore"
)

$proto: inputs.#Proto & {
	source: "service.proto"
}

service: fn.#Service & {
  framework: "GO"

	instantiate: {
		main: datastore.#Exports.Database & {
			with: {
				name:       "main"
				schemaFile: inputs.#FromFile & {
					path: "schema.txt"
				}
			}
		}
	}

	exportService:        $proto.services.PostService
	exportServicesAsHttp: true
	ingress:              "INTERNET_FACING"

	requirePersistentStorage: {
		persistentId: "test-data"
		byteCount:    "1GiB"
		mountPath:    "/testdata"
	}
}
