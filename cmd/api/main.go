package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/jadenj13/wallet-backend/internal/api/application"
)

func main() {
	app, err := application.New(application.LoadConfig())
	if err != nil {
		fmt.Println("Failed to initialize api:", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if err := app.Start(ctx); err != nil {
		fmt.Println("Failed to start api:", err)
	}
}
