server: {
	name: "redis-server"

	image: "redis:6.2.6-alpine@sha256:132337b9d7744ffee4fae83f51de53c3530935ad3ba528b7110f2d805f55cbf5"

	class: "stateful"

	services: "redis": {
		port: 6379
		kind: "tcp"
	}

	// TODO configure a generated password with --requirepass
	args: ["--save", "60", "1"]

	mounts: {
		"/data": ":data"
	}
}

volumes: {
	"data": persistent: {
		id:   "redis-data"
		size: "10GiB"
	}
}
