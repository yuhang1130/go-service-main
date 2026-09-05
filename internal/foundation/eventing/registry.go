package eventing

import (
	"context"
	"fmt"
	"sync"
)

type Key struct {
	Type    string
	Version int
}

type Registry struct {
	mu       sync.RWMutex
	handlers map[Key]Handler
}

func NewRegistry() *Registry { return &Registry{handlers: make(map[Key]Handler)} }

func (r *Registry) Register(eventType string, version int, handler Handler) error {
	key := Key{Type: eventType, Version: version}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.handlers[key]; exists {
		return fmt.Errorf("event handler already registered for %s v%d", eventType, version)
	}
	r.handlers[key] = handler
	return nil
}

func (r *Registry) Handle(ctx context.Context, event Envelope) (Result, error) {
	key := Key{Type: event.EventType, Version: event.EventVersion}
	r.mu.RLock()
	handler, exists := r.handlers[key]
	r.mu.RUnlock()
	if !exists {
		return PermanentRejection, fmt.Errorf("no handler for %s v%d", event.EventType, event.EventVersion)
	}
	return handler.Handle(ctx, event)
}

func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.handlers)
}
