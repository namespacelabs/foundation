import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	postgresdb "namespacelabs.dev/foundation/universe/db/postgres/incluster"
	mariadb "namespacelabs.dev/foundation/universe/db/maria/incluster"
)

$proto: inputs.#Proto & {
	source: "service.proto"
}

service: fn.#Service & {
  framework: "GO_GRPC"

	instantiate: {
		postgres: postgresdb.#Exports.Database & {
			with: {
				name:       "list"
				schemaFile: inputs.#FromFile & {
					path: "schema.sql"
				}
			}
		}
		maria: mariadb.#Exports.Database & {
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
