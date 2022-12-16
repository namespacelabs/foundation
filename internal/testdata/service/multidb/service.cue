import (
	"namespacelabs.dev/foundation/std/fn"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/universe/db/postgres/incluster"
	"namespacelabs.dev/foundation/universe/db/postgres/rds"
)

$proto: inputs.#Proto & {
	source: "../proto/multidb.proto"
}

service: fn.#Service & {
	framework: "GO"

	resources: {
		postgres: {
			class:    "namespacelabs.dev/foundation/library/database/postgres:Database"
			provider: "namespacelabs.dev/foundation/library/oss/postgres"

			intent: {
				name: "postgreslist"
				schema: ["schema_postgres.sql"]
			}

			resources: {
				"cluster": "namespacelabs.dev/foundation/library/oss/postgres:colocated"
			}
		}
	}

	instantiate: {
		"rds": rds.#Exports.Database & {
			name:       "postgreslist"
			schemaFile: inputs.#FromFile & {
				path: "schema_postgres.sql"
			}
		}
		postgres: incluster.#Exports.Database & {
			resourceRef: "namespacelabs.dev/foundation/internal/testdata/service/multidb:postgres"
		}
	}

	exportService:        $proto.services.MultiDbListService
	exportServicesAsHttp: true
	ingress:              "INTERNET_FACING"
}
