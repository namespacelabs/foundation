package templates

#Server: {
	_spec: {
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
		authLocal:      *"trust" | string    // Trust local-socket connections by default
		authHost:       *"password" | string // Require password for local TCP/IP by default
	}

	name: "postgres-server"

	image: _spec.image

	// Postgres mounts a persistent volume which requires a stateful deployment (more conservative update strategy).
	class: "stateful"

	env: {
		// PGDATA may not be a mount point but only a subdirectory.
		PGDATA:                 "/postgres/data/pgdata"
		POSTGRES_PASSWORD_FILE: "/postgres/secrets/password"
		POSTGRES_INITDB_ARGS:   "--auth-local=\(_spec.authLocal) --auth-host=\(_spec.authHost)"
	}

	services: "postgres": {
		port: 5432
		kind: "tcp"
	}

	mounts: {
		"/postgres/data": persistent: _spec.dataVolume

		"/postgres/secrets": configurable: {
			contents: {
				"password": fromSecret: _spec.passwordSecret
			}
		}
	}
}
