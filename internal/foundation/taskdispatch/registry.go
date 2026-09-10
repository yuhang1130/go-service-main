package taskdispatch

import (
	"fmt"
	"strings"
	"sync"
)

type Key struct {
	TaskType string
	Version  int
}

type Registry struct {
	mu       sync.RWMutex
	handlers map[Key]Handler
}

func NewRegistry() *Registry {
	return &Registry{handlers: make(map[Key]Handler)}
}

func (r *Registry) Register(taskType string, version int, handler Handler) error {
	taskType = strings.TrimSpace(taskType)
	if taskType == "" || version <= 0 || handler == nil {
		return fmt.Errorf("task type, positive version, and handler are required")
	}
	key := Key{TaskType: taskType, Version: version}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.handlers[key]; exists {
		return fmt.Errorf("task handler already registered for %s v%d", taskType, version)
	}
	r.handlers[key] = handler
	return nil
}

func (r *Registry) Handler(taskType string, version int) (Handler, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	handler, exists := r.handlers[Key{TaskType: taskType, Version: version}]
	return handler, exists
}

func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.handlers)
}
