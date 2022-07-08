import (
	"namespacelabs.dev/foundation/std/fn"
)

server: fn.#OpaqueServer & {
	id:   "3sdnqjp3kguuq8dogvkg"
	name: "mariadb"

	isStateful: true

	binary: "namespacelabs.dev/foundation/universe/db/maria/server/img"

	import: [
		"namespacelabs.dev/foundation/universe/db/maria/server/creds",
		"namespacelabs.dev/foundation/universe/db/maria/server/data",
	]

	service: "mariadb": {
		containerPort: 3306
		metadata: protocol: "tcp"
	}
}
