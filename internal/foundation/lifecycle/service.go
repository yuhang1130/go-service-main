package lifecycle

import "context"

// Service is one explicitly constructed process dependency. Implementations
// must keep Start and Stop idempotent enough for startup rollback and graceful
// shutdown. Dependencies name other services in the same Manager.
type Service interface {
	Name() string
	Dependencies() []string
	Start(context.Context) error
	Stop(context.Context) error
	Ready(context.Context) error
}

type FuncService struct {
	name         string
	dependencies []string
	start        func(context.Context) error
	stop         func(context.Context) error
	ready        func(context.Context) error
}

func NewService(
	name string,
	dependencies []string,
	start func(context.Context) error,
	stop func(context.Context) error,
	ready func(context.Context) error,
) *FuncService {
	return &FuncService{
		name:         name,
		dependencies: append([]string(nil), dependencies...),
		start:        start,
		stop:         stop,
		ready:        ready,
	}
}

func (s *FuncService) Name() string { return s.name }

func (s *FuncService) Dependencies() []string {
	return append([]string(nil), s.dependencies...)
}

func (s *FuncService) Start(ctx context.Context) error {
	if s.start == nil {
		return nil
	}
	return s.start(ctx)
}

func (s *FuncService) Stop(ctx context.Context) error {
	if s.stop == nil {
		return nil
	}
	return s.stop(ctx)
}

func (s *FuncService) Ready(ctx context.Context) error {
	if s.ready == nil {
		return nil
	}
	return s.ready(ctx)
}
