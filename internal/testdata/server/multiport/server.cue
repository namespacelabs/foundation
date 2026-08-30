server: {
	name: "multiport"

	integration: "go"

	services: {
		multi: {
			ports: [
				{
					port:         50000
					exportedPort: 5000
				},
				{
					port:     4000
					protocol: "udp"
				},
			]
		}
	}

	ports: {
		main: {
			containerPort: 3000
			// hostPort:      30000
		}
	}
}
