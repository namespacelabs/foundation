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

#PackageRef: {
	@fn(input=package_ref)
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
		type:          string
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

#FromFile: {
	@fn(input=resource)
	fntypes.#Resource
}

// XXX To be removed.
#VCS: {
	@fn(input=vcs)
}
