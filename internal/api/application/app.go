package application

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"gorm.io/gorm"

	"github.com/jadenj13/wallet-backend/internal/api/data"
)

type App struct {
	router http.Handler
	config Config
	db     *gorm.DB
}

func New(config Config) (*App, error) {
	db, err := data.Open(config.DBPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	app := &App{
		config: config,
		db:     db,
	}

	app.loadRoutes()

	return app, nil
}

func (a *App) Start(ctx context.Context) error {
	sqlDB, err := a.db.DB()
	if err == nil {
		defer sqlDB.Close()
	}

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", a.config.ServerPort),
		Handler: a.router,
	}

	ch := make(chan error, 1)

	go func() {
		if err := server.ListenAndServe(); err != nil {
			ch <- fmt.Errorf("Failed to start server : %w", err)
		}
		close(ch)
	}()

	select {
	case err = <-ch:
		return err
	case <-ctx.Done():
		timeout, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()

		return server.Shutdown(timeout)
	}
}
