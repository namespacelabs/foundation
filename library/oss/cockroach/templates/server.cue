package templates

#Server: {
	spec: {
		image:          *"cockroachdb/cockroach:v25.3.1@sha256:06f54e54e69405c462cf9be37c480ce03bf29aa73aa6ae2fa2b5a786a358122b" | string
		dataVolumeSize: *"10GiB" | string
		dataVolume: *{
			id:   "cockroach-server-data"
			size: dataVolumeSize
		} | {
			id:   string
			size: string
		}
		passwordSecret: *"namespacelabs.dev/foundation/library/oss/cockroach/server:password" | string
	}

	name: "cockroach-server"

	image: spec.image

	// cockroach mounts a persistent volume which requires a stateful deployment (more conservative update strategy).
	class: "stateful"

	args: [
		"start-single-node",
		"--http-addr", "0.0.0.0:8080",
		"--sql-addr", "0.0.0.0:5432",
		"--log=sinks: {stderr: {channels: all}}",
		"--locality", "region=local",
	]

	env: {
		COCKROACH_DATABASE: "postgres"
		COCKROACH_USER:     "postgres"
		COCKROACH_PASSWORD: fromSecret: spec.passwordSecret
	}

	services: {
		"postgres": {
			port: 5432
			kind: "tcp"
		}
		"http": {
			port: 8080
			kind: "tcp"
		}
		"rpc": {
			port: 26257
			kind: "tcp"
		}
	}

	mounts: {
		"/cockroach/cockroach-data": persistent: spec.dataVolume
	}
}
