// Copyright 2022 Namespace Labs Inc; All rights reserved.
// Licensed under the EARLY ACCESS SOFTWARE LICENSE AGREEMENT
// available at http://github.com/namespacelabs/foundation

package observers

import sync "sync"

type SessionProvider interface {
	NewStackClient() (StackSession, error)
}

type StackSession interface {
	StackEvents() chan *StackUpdateEvent
	Close()
}

func Static() *StaticProvider {
	return &StaticProvider{
		clients: []chan *StackUpdateEvent{},
		mu:      sync.Mutex{},
	}
}

type StaticProvider struct {
	update  *StackUpdateEvent
	clients []chan *StackUpdateEvent
	mu      sync.Mutex
}

func (p *StaticProvider) PushUpdate(update *StackUpdateEvent) {
	p.mu.Lock()
	clients := p.clients
	p.update = update
	p.mu.Unlock()

	for _, ch := range clients {
		ch <- update
	}
}

func (p *StaticProvider) NewStackClient() (StackSession, error) {
	ch := make(chan *StackUpdateEvent, 1)

	p.mu.Lock()
	p.clients = append(p.clients, ch)
	update := p.update
	p.mu.Unlock()

	if update != nil {
		ch <- update
	}

	return staticSession{ch, p}, nil
}

func (p *StaticProvider) RemoveClient(ch chan *StackUpdateEvent) {
	p.mu.Lock()
	defer p.mu.Unlock()

	newClients := []chan *StackUpdateEvent{}
	for _, client := range p.clients {
		if client != ch {
			newClients = append(newClients, client)
		}
	}
	p.clients = newClients

	close(ch)
}

type staticSession struct {
	ch     chan *StackUpdateEvent
	parent *StaticProvider
}

func (s staticSession) StackEvents() chan *StackUpdateEvent { return s.ch }

func (s staticSession) Close() { s.parent.RemoveClient(s.ch) }
