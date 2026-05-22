// Package carrier provides the runtime registry of carrier adapters.
//
// Adapters register themselves at application startup. The registry
// exposes them by role, by zone, and by versioned identifier so the
// routing engine and webhook listener can find them without knowing
// any specific carrier.
package carrier

import (
	"errors"
	"fmt"
	"sync"

	"dashfetchr/internal/ports"
)

// Registry holds the set of carrier adapters live in this process.
// Multiple versions of the same carrier can coexist; routing is driven
// by feature flags persisted in the database.
type Registry struct {
	mu       sync.RWMutex
	byID     map[ports.CarrierID]map[string]ports.CarrierPort // id -> version -> adapter
	primary  map[ports.CarrierID]string                       // id -> primary version
	enabled  map[key]bool                                     // (id, version) -> enabled
}

type key struct {
	ID      ports.CarrierID
	Version string
}

func NewRegistry() *Registry {
	return &Registry{
		byID:    make(map[ports.CarrierID]map[string]ports.CarrierPort),
		primary: make(map[ports.CarrierID]string),
		enabled: make(map[key]bool),
	}
}

// Register adds a versioned adapter. The first version registered for a
// given carrier ID becomes the primary; SetPrimary changes it.
func (r *Registry) Register(version string, adapter ports.CarrierPort) error {
	if adapter == nil {
		return errors.New("carrier: nil adapter")
	}
	id := adapter.Capabilities().ID
	if id == "" {
		return errors.New("carrier: missing capabilities.ID")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.byID[id]; !ok {
		r.byID[id] = make(map[string]ports.CarrierPort)
	}
	if _, dup := r.byID[id][version]; dup {
		return fmt.Errorf("carrier: %s %s already registered", id, version)
	}
	r.byID[id][version] = adapter
	r.enabled[key{id, version}] = true
	if _, set := r.primary[id]; !set {
		r.primary[id] = version
	}
	return nil
}

// SetPrimary chooses which version is returned by Get when no explicit
// version is requested. Used by feature-flag-driven rollouts.
func (r *Registry) SetPrimary(id ports.CarrierID, version string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.byID[id][version]; !ok {
		return fmt.Errorf("carrier: %s %s not registered", id, version)
	}
	r.primary[id] = version
	return nil
}

// SetEnabled toggles availability of a specific version.
func (r *Registry) SetEnabled(id ports.CarrierID, version string, enabled bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.enabled[key{id, version}] = enabled
}

// Get returns the primary adapter for a carrier ID.
func (r *Registry) Get(id ports.CarrierID) (ports.CarrierPort, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.primary[id]
	if !ok {
		return nil, fmt.Errorf("carrier: %s not registered", id)
	}
	if !r.enabled[key{id, v}] {
		return nil, fmt.Errorf("carrier: %s %s disabled", id, v)
	}
	return r.byID[id][v], nil
}

// GetVersion returns an explicit version.
func (r *Registry) GetVersion(id ports.CarrierID, version string) (ports.CarrierPort, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.byID[id][version]
	if !ok {
		return nil, fmt.Errorf("carrier: %s %s not registered", id, version)
	}
	if !r.enabled[key{id, version}] {
		return nil, fmt.Errorf("carrier: %s %s disabled", id, version)
	}
	return a, nil
}

// ListByRole returns all enabled primary adapters that advertise the role.
func (r *Registry) ListByRole(role ports.CarrierRole) []ports.CarrierPort {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ports.CarrierPort, 0, len(r.byID))
	for id, version := range r.primary {
		if !r.enabled[key{id, version}] {
			continue
		}
		a := r.byID[id][version]
		for _, role2 := range a.Capabilities().Roles {
			if role2 == role {
				out = append(out, a)
				break
			}
		}
	}
	return out
}

// All returns every enabled adapter (primary versions only). Mainly for
// admin dashboards.
func (r *Registry) All() []ports.CarrierPort {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ports.CarrierPort, 0, len(r.byID))
	for id, version := range r.primary {
		if r.enabled[key{id, version}] {
			out = append(out, r.byID[id][version])
		}
	}
	return out
}
