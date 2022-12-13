import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	inclusterpg "namespacelabs.dev/foundation/universe/db/postgres/incluster"
	"namespacelabs.dev/foundation/universe/db/postgres/rds"
)

$proto: inputs.#Proto & {
	source: "../proto/multidb.proto"
}

service: fn.#Service & {
	framework: "GO"

	instantiate: {
		"rds": rds.#Exports.Database & {
			name:       "postgreslist"
			schemaFile: inputs.#FromFile & {
				path: "schema_postgres.sql"
			}
		}
		postgres: inclusterpg.#Exports.Database & {
			name:       "postgreslist"
			schemaFile: inputs.#FromFile & {
				path: "schema_postgres.sql"
			}
		}
	}

	exportService:        $proto.services.MultiDbListService
	exportServicesAsHttp: true
	ingress:              "INTERNET_FACING"
}
