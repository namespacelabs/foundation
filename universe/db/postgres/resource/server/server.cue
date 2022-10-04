server: {
	name: "postgres-server"

	// TODO replicate namespacelabs.dev/foundation/universe/db/postgres/server/img via Docker integration
	image: "postgres:14.0@sha256:db927beee892dd02fbe963559f29a7867708747934812a80f83bff406a0d54fd"

	class: "stateful"

	env: {
		// PGDATA may not be a mount point but only a subdirectory.
		PGDATA:                 "/postgres/data/pgdata"
		POSTGRES_PASSWORD_FILE: "/postgres/password"
	}

	services: "postgres": {
		port: 5432
		kind: "tcp"
	}

	mounts: {
		"/postgres/data":     "data"
		"/postgres/password": "password"
	}
}

volumes: {
	"data": persistent: {
		id:   "postgres-server-data"
		size: "10GiB"
	}
	"password": configurable: {
		fromSecret: "password"
	}
}

secrets: {
	// TODO this should be generated
	"password": {
		description: "Postgres server password"
	}
}
