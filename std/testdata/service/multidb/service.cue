import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	postgres "namespacelabs.dev/foundation/universe/db/postgres/incluster"
	maria "namespacelabs.dev/foundation/universe/db/maria/incluster"
)

$proto: inputs.#Proto & {
	source: "service.proto"
}

service: fn.#Service & {
  framework: "GO_GRPC"

	instantiate: {
		postgres: postgres.#Exports.Database & {
			with: {
				name:       "list"
				schemaFile: inputs.#FromFile & {
					path: "schema.sql"
				}
			}
		}
		maria: maria.#Exports.Database & {
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
