package ns

#Sidecars: {
    [string]: {
        _#Integration
        _#ContainerArgs
        _#Mounts

        init: *false | true
        if init == false {
            _#ServiceMap
        }
    }
}