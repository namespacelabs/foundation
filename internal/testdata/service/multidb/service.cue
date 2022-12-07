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

	resources: {
		postgres: {
			class:    "namespacelabs.dev/foundation/library/database/postgres:Database"
			provider: "namespacelabs.dev/foundation/library/oss/postgres"

			intent: {
				name: "postgreslist"
				schema: ["schema_postgres.sql"]
			}

			resources: {
				"cluster": ":postgresCluster"
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

resources: {
	postgresCluster: {
		class:    "namespacelabs.dev/foundation/library/database/postgres:Cluster"
		provider: "namespacelabs.dev/foundation/library/oss/postgres"

		intent: {}
	}
}
