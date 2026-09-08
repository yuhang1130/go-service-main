package lifecycle

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"
)

var ErrNotReady = errors.New("service is not ready")

type Options struct {
	StartTimeout time.Duration
	StopTimeout  time.Duration
}

type serviceState uint8

const (
	stateStopped serviceState = iota
	stateStarting
	stateReady
	stateFailed
	stateStopping
)

type Manager struct {
	logger   *slog.Logger
	options  Options
	services map[string]Service
	layers   [][]string

	mu       sync.RWMutex
	states   map[string]serviceState
	starting bool
	started  bool
}

func NewManager(logger *slog.Logger, options Options, services ...Service) (*Manager, error) {
	if logger == nil {
		logger = slog.Default()
	}
	serviceMap := make(map[string]Service, len(services))
	for _, service := range services {
		if service == nil {
			return nil, errors.New("lifecycle service is nil")
		}
		name := strings.TrimSpace(service.Name())
		if name == "" {
			return nil, errors.New("lifecycle service name is required")
		}
		if _, exists := serviceMap[name]; exists {
			return nil, fmt.Errorf("lifecycle service %q is registered more than once", name)
		}
		serviceMap[name] = service
	}
	layers, err := resolveLayers(serviceMap)
	if err != nil {
		return nil, err
	}
	states := make(map[string]serviceState, len(serviceMap))
	for name := range serviceMap {
		states[name] = stateStopped
	}
	return &Manager{
		logger:   logger,
		options:  options,
		services: serviceMap,
		layers:   layers,
		states:   states,
	}, nil
}

func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.started {
		m.mu.Unlock()
		return nil
	}
	if m.starting {
		m.mu.Unlock()
		return errors.New("lifecycle manager is already starting")
	}
	m.starting = true
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		m.starting = false
		m.mu.Unlock()
	}()

	for _, layer := range m.layers {
		if err := m.startLayer(ctx, layer); err != nil {
			rollbackErr := m.stopStarted(context.Background())
			return errors.Join(err, rollbackErr)
		}
	}
	m.mu.Lock()
	m.started = true
	m.mu.Unlock()
	return nil
}

func (m *Manager) Stop(ctx context.Context) error {
	return m.stopStarted(ctx)
}

func (m *Manager) ReadinessChecks() map[string]func(context.Context) error {
	checks := make(map[string]func(context.Context) error, len(m.services))
	for name, service := range m.services {
		name := name
		service := service
		checks[name] = func(ctx context.Context) error {
			m.mu.RLock()
			state := m.states[name]
			started := m.started
			m.mu.RUnlock()
			if !started || state != stateReady {
				return fmt.Errorf("%w: %s", ErrNotReady, name)
			}
			if err := service.Ready(ctx); err != nil {
				return fmt.Errorf("%s readiness: %w", name, err)
			}
			return nil
		}
	}
	return checks
}

func (m *Manager) startLayer(ctx context.Context, names []string) error {
	layerCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	type result struct {
		name string
		err  error
	}
	results := make(chan result, len(names))
	for _, name := range names {
		name := name
		go func() {
			m.setState(name, stateStarting)
			m.logger.Info("service starting", "service", name)
			err := callWithTimeout(layerCtx, m.options.StartTimeout, m.services[name].Start)
			results <- result{name: name, err: err}
		}()
	}

	var failures []error
	for range names {
		result := <-results
		if result.err != nil {
			cancel()
			m.setState(result.name, stateFailed)
			failures = append(failures, fmt.Errorf("start service %s: %w", result.name, result.err))
			continue
		}
		m.setState(result.name, stateReady)
		m.logger.Info("service ready", "service", result.name)
	}
	return errors.Join(failures...)
}

func (m *Manager) stopStarted(ctx context.Context) error {
	m.mu.Lock()
	m.started = false
	m.mu.Unlock()

	var failures []error
	for layerIndex := len(m.layers) - 1; layerIndex >= 0; layerIndex-- {
		names := m.layers[layerIndex]
		type result struct {
			name string
			err  error
		}
		results := make(chan result, len(names))
		pending := 0
		for _, name := range names {
			if !m.transitionToStopping(name) {
				continue
			}
			pending++
			name := name
			go func() {
				m.logger.Info("service stopping", "service", name)
				err := callWithTimeout(ctx, m.options.StopTimeout, m.services[name].Stop)
				results <- result{name: name, err: err}
			}()
		}
		for range pending {
			result := <-results
			m.setState(result.name, stateStopped)
			if result.err != nil {
				failures = append(failures, fmt.Errorf("stop service %s: %w", result.name, result.err))
				continue
			}
			m.logger.Info("service stopped", "service", result.name)
		}
	}
	return errors.Join(failures...)
}

func (m *Manager) transitionToStopping(name string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.states[name] != stateReady && m.states[name] != stateFailed {
		return false
	}
	m.states[name] = stateStopping
	return true
}

func (m *Manager) setState(name string, state serviceState) {
	m.mu.Lock()
	m.states[name] = state
	m.mu.Unlock()
}

func callWithTimeout(ctx context.Context, timeout time.Duration, call func(context.Context) error) error {
	if timeout <= 0 {
		return call(ctx)
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return call(callCtx)
}

func resolveLayers(services map[string]Service) ([][]string, error) {
	indegree := make(map[string]int, len(services))
	dependents := make(map[string][]string, len(services))
	for name := range services {
		indegree[name] = 0
	}
	for name, service := range services {
		seen := make(map[string]struct{})
		for _, dependency := range service.Dependencies() {
			dependency = strings.TrimSpace(dependency)
			if dependency == "" {
				return nil, fmt.Errorf("service %q has an empty dependency", name)
			}
			if dependency == name {
				return nil, fmt.Errorf("service %q depends on itself", name)
			}
			if _, exists := services[dependency]; !exists {
				return nil, fmt.Errorf("service %q depends on unknown service %q", name, dependency)
			}
			if _, duplicate := seen[dependency]; duplicate {
				continue
			}
			seen[dependency] = struct{}{}
			indegree[name]++
			dependents[dependency] = append(dependents[dependency], name)
		}
	}

	layers := make([][]string, 0)
	processed := 0
	for processed < len(services) {
		layer := make([]string, 0)
		for name, degree := range indegree {
			if degree == 0 {
				layer = append(layer, name)
			}
		}
		if len(layer) == 0 {
			return nil, errors.New("lifecycle dependency graph contains a cycle")
		}
		sort.Strings(layer)
		layers = append(layers, layer)
		processed += len(layer)
		for _, name := range layer {
			delete(indegree, name)
			for _, dependent := range dependents[name] {
				indegree[dependent]--
			}
		}
	}
	return layers, nil
}
