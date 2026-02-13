package api

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/jadenj13/wallet-backend/application"
)

func main() {
	app := application.New(application.LoadConfig())

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	err := app.Start(ctx)
	if err != nil {
		fmt.Println("Failed to start api:", err)
	}
}
