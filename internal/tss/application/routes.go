package application

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/jadenj13/wallet-backend/internal/tss/data"
	"github.com/jadenj13/wallet-backend/internal/tss/handler"
)

func (a *App) loadRoutes() {
	router := chi.NewRouter()

	router.Use(middleware.Logger)

	store := data.NewRedisPartyStore(a.rdb)
	partyHandler := handler.NewParty(
		int(a.config.PartyID),
		a.config.Peers,
		int(a.config.Threshold),
		int(a.config.Parties),
		store,
	)

	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	router.Post("/keygen", partyHandler.InitKeygen)
	router.Post("/sign", partyHandler.HandleSign)
	router.Post("/message", partyHandler.HandleMessage)
	router.Get("/pubkey", partyHandler.HandleGetPubKey)

	router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	a.router = router
}
