package eventing

import (
	"context"
	"fmt"
	"strings"

	domainevent "github.com/yuhang1130/go-service-main/internal/foundation/eventing"
)

// InboxHandler decorates a business event handler with database-backed
// idempotency for at-least-once broker delivery.
type InboxHandler struct {
	store         *InboxStore
	consumerGroup string
	next          domainevent.Handler
}

func NewInboxHandler(store *InboxStore, consumerGroup string, next domainevent.Handler) (*InboxHandler, error) {
	if store == nil {
		return nil, fmt.Errorf("inbox store is required")
	}
	if strings.TrimSpace(consumerGroup) == "" {
		return nil, fmt.Errorf("consumer group is required")
	}
	if next == nil {
		return nil, fmt.Errorf("event handler is required")
	}
	return &InboxHandler{store: store, consumerGroup: consumerGroup, next: next}, nil
}

func (h *InboxHandler) Handle(ctx context.Context, event domainevent.Envelope) (domainevent.Result, error) {
	return h.store.Process(ctx, h.consumerGroup, event, h.next)
}
