package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"edith/studio/internal/studio"
)

func main() {
	projectRoot, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := studio.Start(ctx, projectRoot, "127.0.0.1:8765"); err != nil {
		log.Fatal(err)
	}
}
