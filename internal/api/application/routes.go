package application

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/jadenj13/wallet-backend/internal/api/client"
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
	parties := make([]*client.PartyClient, len(a.config.PartyURLs))
	for i, url := range a.config.PartyURLs {
		parties[i] = client.NewPartyClient(url)
	}

	walletHandler := handler.NewWallet(parties)

	router.Post("/", walletHandler.Init)
	router.Post("/sign", walletHandler.Sign)
	router.Get("/pubkey", walletHandler.GetPubKey)
	router.Get("/address", walletHandler.GetAddress)
}
