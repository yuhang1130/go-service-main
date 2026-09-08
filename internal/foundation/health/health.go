package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/yuhang1130/go-service-main/internal/foundation/buildinfo"
)

type Check func(context.Context) error

var managementRoutes = []string{
	"GET /livez",
	"GET /readyz",
	"GET /buildinfo",
}

type managementHandler struct {
	mux *http.ServeMux
}

func (h *managementHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	h.mux.ServeHTTP(writer, request)
}

func (h *managementHandler) Routes() []string {
	return append([]string(nil), managementRoutes...)
}

type Registry struct {
	ready  atomic.Bool
	mu     sync.RWMutex
	checks map[string]Check
	build  buildinfo.Info
}

func New(build buildinfo.Info) *Registry {
	return &Registry{checks: make(map[string]Check), build: build}
}

func (r *Registry) Register(name string, check Check) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.checks[name] = check
}

func (r *Registry) RegisterAll(checks map[string]func(context.Context) error) {
	for name, check := range checks {
		r.Register(name, check)
	}
}

func (r *Registry) SetReady(ready bool) { r.ready.Store(ready) }

func (r *Registry) Handler() http.Handler {
	handler := &managementHandler{mux: http.NewServeMux()}
	handler.mux.HandleFunc("GET /livez", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "alive"})
	})
	handler.mux.HandleFunc("GET /readyz", r.readyHandler)
	handler.mux.HandleFunc("GET /buildinfo", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, r.build)
	})
	return handler
}

func (r *Registry) readyHandler(w http.ResponseWriter, request *http.Request) {
	if !r.ready.Load() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
		return
	}
	ctx, cancel := context.WithTimeout(request.Context(), 750*time.Millisecond)
	defer cancel()
	checks, names := r.snapshot()
	type result struct {
		name string
		err  error
	}
	results := make(chan result, len(names))
	for _, name := range names {
		name := name
		go func() {
			results <- result{name: name, err: checks[name](ctx)}
		}()
	}
	failures := make(map[string]struct{})
	completed := make(map[string]struct{})
	for range names {
		select {
		case result := <-results:
			completed[result.name] = struct{}{}
			if result.err != nil {
				failures[result.name] = struct{}{}
			}
		case <-ctx.Done():
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready", "dependency": firstPending(names, completed)})
			return
		}
	}
	for _, name := range names {
		if _, failed := failures[name]; failed {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready", "dependency": name})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (r *Registry) snapshot() (map[string]Check, []string) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	checks := make(map[string]Check, len(r.checks))
	names := make([]string, 0, len(r.checks))
	for name, check := range r.checks {
		checks[name] = check
		names = append(names, name)
	}
	sort.Strings(names)
	return checks, names
}

func firstPending(names []string, completed map[string]struct{}) string {
	for _, name := range names {
		if _, done := completed[name]; !done {
			return name
		}
	}
	return "unknown"
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
