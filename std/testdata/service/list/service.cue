import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/universe/db/postgres/incluster"
)

$proto: inputs.#Proto & {
	source: "service.proto"
}

service: fn.#Service & {
	instantiate: {
		db: incluster.#Exports.Database & {
			with: {
				name:       "list"
				schemaFile: inputs.#FromFile & {
					path: "schema.sql"
				}
			}
		}
	}

	exportService:        $proto.services.ListService
	exportServicesAsHttp: true
	ingress:              "INTERNET_FACING"
}
