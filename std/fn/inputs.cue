package inputs

import fntypes "namespacelabs.dev/foundation/std/fn:types"

#Port: {
	@fn(alloc=port)
	name: string
	// Set by the runtime.
	port?: int
}

#Package: {
	@fn(input=package)
	string
}

#Server: {
	@fn(input=server_dep)

	packageName: #Package

	// Set by the runtime.
	id?:   string
	name?: string
	endpoints: [...#Endpoint]

	$endpointMap: {
		for endpoint in endpoints {
			"\(endpoint.serviceName)": endpoint
		}
	}

	$addressMap: {
		for endpoint in endpoints {
			"\(endpoint.serviceName)": "\(endpoint.allocatedName):\(endpoint.containerPort)"
		}
	}

	#Endpoint: {
		type:          "PRIVATE" | "INTERNET_FACING"
		serviceName:   string
		allocatedName: string
		containerPort: int
	}
}

#Proto: {
	@fn(input=protoload)

	*{sources: [string, ...string]} | {source: string, sources: [source]}

	// Set by the runtime.
	types: [string]:    fntypes.#Proto
	services: [string]: fntypes.#Proto
}

#Workspace: {
	@fn(input=workspace)
	serverPath: string
}

#Environment: {
	@fn(input=env)
	name:      string
	runtime:   string
	purpose:   "DEVELOPMENT" | "TESTING" | "PRODUCTION"
	ephemeral: bool
	labels: [string]: string
}

#FromFile: {
	@fn(input=resource)
	fntypes.#Resource
}

// XXX To be removed.
#VCS: {
	@fn(input=vcs)
}
