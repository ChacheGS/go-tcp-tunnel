// Copyright (C) 2017 Michał Matczuk
// Copyright (C) 2022 jlandowner
// Use of this source code is governed by an AGPL-style
// license that can be found in the LICENSE file.

package tunnel

import (
	"fmt"
	"net"
	"sync"

	"github.com/jlandowner/go-tcp-tunnel/id"
	"github.com/jlandowner/go-tcp-tunnel/log"
)

// RegistryItem holds information about hosts and listeners associated with a
// client.
type RegistryItem struct {
	Hosts     []string
	Listeners []net.Listener
}

type hostInfo struct {
	identifier id.ID
}

type registry struct {
	items  map[id.ID]*RegistryItem
	hosts  map[string]*hostInfo
	mu     sync.RWMutex
	logger log.Logger
}

func newRegistry(logger log.Logger) *registry {
	if logger == nil {
		logger = log.NewNopLogger()
	}

	return &registry{
		items:  make(map[id.ID]*RegistryItem),
		hosts:  make(map[string]*hostInfo),
		logger: logger,
	}
}

var voidRegistryItem = &RegistryItem{}

// Subscribe allows to connect client with a given identifier.
func (r *registry) Subscribe(identifier id.ID) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.items[identifier]; ok {
		return
	}

	r.logger.Log(
		"level", 1,
		"action", "subscribe",
		"identifier", identifier,
	)

	r.items[identifier] = voidRegistryItem
}

// IsSubscribed returns true if client is subscribed.
func (r *registry) IsSubscribed(identifier id.ID) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.items[identifier]
	return ok
}

// Subscriber returns client identifier assigned to given host.
func (r *registry) Subscriber(hostPort string) (id.ID, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	h, ok := r.hosts[trimPort(hostPort)]
	if !ok {
		return id.ID{}, false
	}

	return h.identifier, ok
}

// Unsubscribe removes client from registry and returns it's RegistryItem.
func (r *registry) Unsubscribe(identifier id.ID) *RegistryItem {
	r.mu.Lock()
	defer r.mu.Unlock()

	i, ok := r.items[identifier]
	if !ok {
		return nil
	}

	r.logger.Log(
		"level", 1,
		"action", "unsubscribe",
		"identifier", identifier,
	)

	if i.Hosts != nil {
		for _, h := range i.Hosts {
			delete(r.hosts, h)
		}
	}

	delete(r.items, identifier)

	return i
}

func (r *registry) set(i *RegistryItem, identifier id.ID) error {
	r.logger.Log(
		"level", 2,
		"action", "set registry item",
		"identifier", identifier,
	)

	r.mu.Lock()
	defer r.mu.Unlock()

	j, ok := r.items[identifier]
	if !ok {
		return errClientNotSubscribed
	}
	if j != voidRegistryItem {
		return fmt.Errorf("attempt to overwrite registry item")
	}

	if i.Hosts != nil {
		for _, h := range i.Hosts {
			if _, ok := r.hosts[trimPort(h)]; ok {
				return fmt.Errorf("host %q is occupied", h)
			}
		}

		for _, h := range i.Hosts {
			r.hosts[trimPort(h)] = &hostInfo{
				identifier: identifier,
			}
		}
	}

	r.items[identifier] = i

	return nil
}

func (r *registry) clear(identifier id.ID) *RegistryItem {
	r.logger.Log(
		"level", 2,
		"action", "clear registry item",
		"identifier", identifier,
	)

	r.mu.Lock()
	defer r.mu.Unlock()

	i, ok := r.items[identifier]
	if !ok || i == voidRegistryItem {
		return nil
	}

	if i.Hosts != nil {
		for _, h := range i.Hosts {
			delete(r.hosts, trimPort(h))
		}
	}

	r.items[identifier] = voidRegistryItem

	return i
}

func trimPort(hostPort string) (host string) {
	host, _, _ = net.SplitHostPort(hostPort)
	if host == "" {
		host = hostPort
	}
	return
}
