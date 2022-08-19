import (
	"namespacelabs.dev/foundation/universe/db/postgres/server:template"
)

server: template.#PostgresServer & {
	id:   "422eajpp5jt8pjwfp2vrq5f0ce"
	name: "postgresql"

	import: [
		"namespacelabs.dev/foundation/universe/db/postgres/server/creds",
		"namespacelabs.dev/foundation/universe/db/postgres/server/data",
	]
}
