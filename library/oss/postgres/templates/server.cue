package templates

#Server: {
	spec: {
		image:          *"postgres:14.0@sha256:db927beee892dd02fbe963559f29a7867708747934812a80f83bff406a0d54fd" | string
		dataVolumeSize: *"10GiB" | string
		dataVolume:     *{
			id:   "postgres-server-data"
			size: dataVolumeSize
		} | {
			id:   string
			size: string
		}
		passwordSecret: *"namespacelabs.dev/foundation/library/oss/postgres/server:password" | string
	}

	name: "postgres-server"

	image: spec.image

	// Postgres mounts a persistent volume which requires a stateful deployment (more conservative update strategy).
	class: "stateful"

	env: {
		// PGDATA may not be a mount point but only a subdirectory.
		PGDATA:                 "/postgres/data/pgdata"
		POSTGRES_PASSWORD_FILE: "/postgres/secrets/password"
		POSTGRES_INITDB_ARGS:   "--auth-local=trust --auth-host=password" // Trust local-socket connections, require password for local TCP/IP
	}

	services: "postgres": {
		port: 5432
		kind: "tcp"
	}

	mounts: {
		"/postgres/data": persistent: spec.dataVolume

		"/postgres/secrets": configurable: {
			contents: {
				"password": fromSecret: spec.passwordSecret
			}
		}
	}
}
