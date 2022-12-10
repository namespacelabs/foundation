resourceClasses: {
	"Server": {
		intent: {
			type:   "foundation.library.runtime.ServerIntent"
			source: "./intents.proto"
		}
		produces: {
			type:   "foundation.schema.runtime.Server"
			source: "../../schema/runtime/config.proto"
		}
	}
	"Secret": {
		intent: {
			type: "foundation.schema.PackageRef"
		}
		produces: {
			type:   "foundation.library.runtime.SecretInstance"
			source: "./instances.proto"
		}
	}
}
