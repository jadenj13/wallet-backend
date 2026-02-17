package application

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/jadenj13/wallet-backend/internal/api/handler"
)

func (a *App) loadRoutes() {
	router := chi.NewRouter()

	router.Use(middleware.Logger)

	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	router.Route("/wallet", a.loadWalletRoutes)

	a.router = router
}

func (a *App) loadWalletRoutes(router chi.Router) {
	walletHandler := &handler.Wallet{}

	router.Post("/", walletHandler.Init)
	router.Post("/sign", walletHandler.Sign)
}
