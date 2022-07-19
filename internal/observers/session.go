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

func Static(update *StackUpdateEvent) SessionProvider {
	return staticProvider{update}
}

type staticProvider struct{ update *StackUpdateEvent }
type staticSession struct{ ch chan *StackUpdateEvent }

func (p staticProvider) NewStackClient() (StackSession, error) {
	ch := make(chan *StackUpdateEvent, 1)
	ch <- p.update
	return staticSession{ch}, nil
}

func (s staticSession) StackEvents() chan *StackUpdateEvent { return s.ch }
func (s staticSession) Close()                              { close(s.ch) }
