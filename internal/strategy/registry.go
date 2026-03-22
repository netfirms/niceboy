package strategy

import (
	"fmt"
	"sync"
)

var (
	registry = make(map[string]func(params map[string]interface{}) (Strategy, error))
	mu       sync.RWMutex
)

// Register adds a strategy factory to the registry
func Register(name string, factory func(params map[string]interface{}) (Strategy, error)) {
	mu.Lock()
	defer mu.Unlock()
	registry[name] = factory
}

// New creates a new instance of a registered strategy
func New(name string, params map[string]interface{}) (Strategy, error) {
	mu.RLock()
	defer mu.RUnlock()
	factory, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("strategy %s not found in registry", name)
	}
	return factory(params)
}

// List returns a list of all registered strategy names
func List() []string {
	mu.RLock()
	defer mu.RUnlock()
	list := make([]string, 0, len(registry))
	for name := range registry {
		list = append(list, name)
	}
	return list
}
