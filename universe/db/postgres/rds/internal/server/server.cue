import (
	"namespacelabs.dev/foundation/universe/db/postgres/server:template"
)

// Incluster Postgres server for hermetic testing + development.
server: template.#PostgresServer & {
	id:   "4djdk432rddfl9fmpa30"
	name: "mockrds-postgresql"
}
