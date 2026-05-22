package carrier

import (
	"dashfetchr/internal/adapters/carriers/bolt"
)

// Config aggregates per-carrier settings. New carriers add a field here.
type Config struct {
	Bolt bolt.Config
	// Glovo   glovo.Config   // v2
	// Sameday sameday.Config // v2
	// FAN     fan.Config     // v2
	// DHL     dhl.Config     // v2
	// Wolt    wolt.Config    // v2
}

// Wire constructs the registry with all enabled carrier adapters.
//
// Adding a new carrier to the runtime is a one-line change here, plus
// the adapter package and the Config field above.
func Wire(cfg Config) (*Registry, error) {
	r := NewRegistry()

	if err := r.Register("v1", bolt.New(cfg.Bolt)); err != nil {
		return nil, err
	}
	// if err := r.Register("v1", glovo.New(cfg.Glovo)); err != nil { return nil, err }
	// if err := r.Register("v1", sameday.New(cfg.Sameday)); err != nil { return nil, err }
	// if err := r.Register("v1", fan.New(cfg.FAN)); err != nil { return nil, err }
	// if err := r.Register("v1", dhl.New(cfg.DHL)); err != nil { return nil, err }
	// if err := r.Register("v1", wolt.New(cfg.Wolt)); err != nil { return nil, err }

	return r, nil
}
