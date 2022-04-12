// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package deadlines

import (
	"context"
)

type DeadlineRegistration struct {
	conf *Deadline
}

func (dl *DeadlineRegistration) Add(conf *Deadline_Configuration) {
	dl.conf.Configuration = append(dl.conf.Configuration, conf)
}

func ProvideDeadlines(ctx context.Context, conf *Deadline, deps ExtensionDeps) (*DeadlineRegistration, error) {
	// XXX validate isolation, i.e. caller is only registering deadlines for itself.

	reg := &DeadlineRegistration{conf: conf}

	mu.Lock()
	registrations = append(registrations, reg)
	mu.Unlock()

	return reg, nil
}
