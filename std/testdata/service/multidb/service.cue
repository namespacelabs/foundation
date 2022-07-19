import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/universe/db/postgres/rds"
	mariadb "namespacelabs.dev/foundation/universe/db/maria/incluster"
)

$proto: inputs.#Proto & {
	source: "../proto/multidb.proto"
}

service: fn.#Service & {
	framework: "GO"

	instantiate: {
		postgres: rds.#Exports.Database & {
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
