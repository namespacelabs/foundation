resourceClasses: {
	"Server": {
		intent: {
			type:   "foundation.std.runtime.ServerIntent"
			source: "./intents.proto"
		}
		produces: {
			type:   "foundation.schema.runtime.Server"
			source: "../../schema/runtime/config.proto"
		}
	}
	"Secret": {
		intent: {
			type:   "foundation.std.runtime.SecretIntent"
			source: "./intents.proto"
		}
		produces: {
			type:   "foundation.std.runtime.SecretInstance"
			source: "./instances.proto"
		}
	}
}
