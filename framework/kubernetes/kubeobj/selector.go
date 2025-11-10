// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package kubeobj

import (
	"fmt"
	"sort"
	"strings"
)

const (
	K8sServerId = "k8s.namespacelabs.dev/server-id"
)

type IDGetter interface {
	GetId() string
}

func SelectById(srv IDGetter) map[string]string {
	return map[string]string{
		K8sServerId: srv.GetId(),
	}
}

func SerializeSelector(selector map[string]string) string {
	var sels []string
	for k, v := range selector {
		sels = append(sels, fmt.Sprintf("%s=%s", k, v))
	}
	sort.Strings(sels)
	return strings.Join(sels, ",")
}
