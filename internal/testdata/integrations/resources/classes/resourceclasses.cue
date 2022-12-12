resourceClasses: {
	"Database": {
		// XXX remove when provider-based intents are working.
		intent: {
			type:   "foundation.internal.testdata.integrations.resources.classes.DatabaseIntent"
			source: "../testgenprovider/proto1.proto"
		}
		produces: {
			type:   "foundation.internal.testdata.integrations.resources.classes.protos.DatabaseInstance"
			source: "./protos/proto2.proto"
		}
	}
}
