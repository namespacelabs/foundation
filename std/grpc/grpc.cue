package grpc

import (
	"path"
	"namespacelabs.dev/foundation/std/fn:inputs"
	"namespacelabs.dev/foundation/std/go/grpc"
)

#Backend: {
	packageName: string
	service:     inputs.#Service & {
		"packageName": packageName
	}

	instanceName:     "\(path.Base(service.packageName))"
	connInstanceName: "\(path.Base(service.packageName))_conn"

	instances: {
		"\(connInstanceName)": grpc.#Exports.Conn & {
			packageName:   service.packageName
			protoTypename: service.protoTypename
		}

		"\(instanceName)": {
			"package":  service.goPackage
			"typename": "\(service.protoTypename)Client"
			"method":   "New\(service.protoTypename)Client"
			"arguments": [{ref: connInstanceName}]

			#Definition: {
				typeDefinition: {
					"typename": "foundation.languages.golang.Instantiate"
				}
			}
		}
	}
}
