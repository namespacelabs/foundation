// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package kubetool

import (
	"log"

	"google.golang.org/protobuf/proto"
)

type perNode interface {
	UnpackInput(proto.Message) error
}

// FromRequest is meant to be used by provisioning tools (and thus can os.Exit).
func FromRequest(r perNode) *KubernetesEnv {
	env := &KubernetesEnv{}

	if err := r.UnpackInput(env); err != nil {
		log.Fatal(err)
	}

	return env
}