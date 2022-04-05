import (
	"namespacelabs.dev/foundation/std/fn"
)

server: fn.#OpaqueServer & {
	id:   "3sdnqjp3kguuq8dogvkg"
	name: "mariadb"

	isStateful: true

	// TODO wrap image to speed up startup and skip initialization from file
	// https://github.com/MariaDB/mariadb-docker/blob/master/10.7/docker-entrypoint.sh#L493
	binary: image: "mariadb:10.7.3"

	import: [
		"namespacelabs.dev/foundation/universe/db/maria/server/creds",
		"namespacelabs.dev/foundation/universe/db/maria/server/data",
	]

	service: "mariadb": {
		containerPort: 3306
		metadata: protocol: "tcp"
	}
}
