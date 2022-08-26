package ns

#Server: {
    name: string
    kind: *"namespace.so/stateless" | "namespace.so/stateful" | "namespace.so/daemonset"

    _#Integration
    _#ContainerArgs
    _#Mounts
    _#ServiceMap
}


_#Integration: {
    // Simplified syntax: if only a string is provided, it represents kind
    integration: *string | {
        kind:       string
        useCodeGen: *true|false

        _#GoIntegration | _#NodeIntegration | _#FromDockerfile | _#FromImage
    }
}

_#GoIntegration: {
    kind:    "namespace.so/go"
    basedir: *"." | string
}

_#NodeIntegration: {
    kind:    "namespace.so/node"
    basedir: *"." | string
}

_#FromDockerfile: {
    kind:       "namespace.so/from-dockerfile"
    useCodeGen: false
    dockerfile: string
}

_#FromImage: {
    kind:       "namespace.so/from-image"
    useCodeGen: false
    image:      string
}

_#ContainerArgs: {
    args?:    [...string]
    env?:     [string]: _#FileContent
    command?: string
}

_#Mounts: {
    // Keyed by mount path
    mounts?: [string]: {*_#VolumeSpec | _#VolumeRef}
}

_#ServiceMap: {
    services: [string]: _#Service
}

_#Service: {
    kind:     string
    port:     int
    ingress?: _#Ingress

    _#HttpService | _#GrpcService
}

_#HttpService: {
    kind: "http"
}

_#GrpcService: {
    kind:         "grpc"
    grpcServices: [string]: {
        source:          string
        transcodeToHttp: *true | false
    }
}

_#Ingress: {
    internet_facing: *false | true

    httpRoutes: [string]: (_#HttpPathList | _#HttpUrlMap)
}

_#HttpPathList: [...string]

_#HttpUrlMap: {
    [string]: _#HttpUrlMapEntry
}

_#HttpUrlMapEntry: {
    *string | {
        cors:  *true | false
        mapTo: string
    } | {
        transcodeToHttp: string
    }
}