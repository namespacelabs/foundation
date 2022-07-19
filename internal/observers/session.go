// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package observers

type SessionProvider interface {
	NewStackClient() (StackSession, error)
}

type StackSession interface {
	StackEvents() chan *StackUpdateEvent
	Close()
}
