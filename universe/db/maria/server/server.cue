import (
	"namespacelabs.dev/foundation/std/fn"
)

server: fn.#OpaqueServer & {
	id:   "3sdnqjp3kguuq8dogvkg"
	name: "mariadb"

	isStateful: true

	// TODO wrap image to speed up startup and skip initialization from file
	// https://github.com/MariaDB/mariadb-docker/blob/master/10.7/docker-entrypoint.sh#L493
	binary: image: "mariadb:10.7.3@sha256:07e06f2e7ae9dfc63707a83130a62e00167c827f08fcac7a9aa33f4b6dc34e0e"

	import: [
		"namespacelabs.dev/foundation/universe/db/maria/server/creds",
		"namespacelabs.dev/foundation/universe/db/maria/server/data",
	]

	service: "mariadb": {
		containerPort: 3306
		metadata: protocol: "tcp"
	}
}
