package bootstrap

import (
	"fmt"

	"github.com/yuhang1130/go-service-main/internal/foundation/eventing"
	"gorm.io/gorm"
)

type eventHandlerRegistration struct {
	feature  string
	register func(*eventing.Registry, *gorm.DB, string) error
}

var eventHandlerRegistrations []eventHandlerRegistration

// registerEventHandlers is the explicit Consumer composition root. Register
// handlers by stable event type and version before deploying the Consumer Role.
func registerEventHandlers(registry *eventing.Registry, database *gorm.DB, consumerGroup string) error {
	for _, registration := range eventHandlerRegistrations {
		if err := registration.register(registry, database, consumerGroup); err != nil {
			return fmt.Errorf("register %s event handlers: %w", registration.feature, err)
		}
	}
	return nil
}
