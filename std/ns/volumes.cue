package ns

#Volume: {
	[string]: _#VolumeSpec
}

_#VolumeSpec: {
	// Simplified syntax: if only a string is provided, it represents kind
	*string | {
		kind: string

		_#Ephemeral | _#Persistent | _#Configurable | _#PackageSync
	}
}

_#VolumeRef: {
	kind:   "namespace.so/volume/ref"
	volume: string
}

_#Ephemeral: {
	kind: "namespace.so/volume/ephemeral"
}

_#Persistent: {
	kind: "namespace.so/volume/persistent"
	id:   string
	size: string
}

_#Configurable: {
	kind: "namespace.so/volume/configurable"

	// Keyed by target path
	contents: *{[string]: _#Content} | _#Content
}

_#Content: {
	{fromDir: string} | _#FileContent
}

_#FileContent: {
	// TODO consider adding fromInvocation & fromConfiguration

	// string is used for inline content
	string | {fromFile: string} | {fromSecret: string}
}

_#PackageSync: {
	kind: "namespace.so/package-sync"
	fileset: {
		// Hidden files are excluded by default, unless explicitly included.
		include: [...string]
		exclude: [...string]
	}
}
