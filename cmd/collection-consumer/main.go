package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/yuhang1130/go-service-main/internal/bootstrap"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := bootstrap.RunCollectionConsumer(ctx); err != nil {
		log.Fatal(err)
	}
}
