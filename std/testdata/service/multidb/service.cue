import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	postgresdb "namespacelabs.dev/foundation/universe/db/postgres/incluster"
	mariadb "namespacelabs.dev/foundation/universe/db/maria/incluster"
)

$proto: inputs.#Proto & {
	source: "../proto/multidb.proto"
}

service: fn.#Service & {
	framework: "GO_GRPC"

	instantiate: {
		postgres: postgresdb.#Exports.Database & {
			name:       "postgreslist"
			schemaFile: inputs.#FromFile & {
				path: "schema_postgres.sql"
			}
		}
		maria: mariadb.#Exports.Database & {
			name:       "mariadblist"
			schemaFile: inputs.#FromFile & {
				path: "schema_maria.sql"
			}
		}
	}

	exportService:        $proto.services.MultiDbListService
	exportServicesAsHttp: true
	ingress:              "INTERNET_FACING"
}
