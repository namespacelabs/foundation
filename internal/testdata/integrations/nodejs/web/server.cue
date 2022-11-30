server: {
	name: "myserver"

	integration: web: {
		// Default Vite port
		devPort: 5173
	}
}

tests: {
	health: {
		integration: shellscript: {
			entrypoint: "test/test.sh"
			requiredPackages: ["jq"]
		}
	}
}
