package application

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/jadenj13/wallet-backend/internal/api/client"
	"github.com/jadenj13/wallet-backend/internal/api/data"
	"github.com/jadenj13/wallet-backend/internal/api/handler"
)

func (a *App) loadRoutes() {
	router := chi.NewRouter()

	router.Use(middleware.Logger)

	router.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	authService := handler.NewAuthService(
		[]byte(a.config.JWTSecret),
		[]byte(a.config.DeviceSecret),
		data.NewSessionStore(a.db),
		data.NewDeviceStore(a.db),
		data.NewUserStore(a.db),
		data.NewTOTPStore(a.db),
	)

	router.Route("/auth", func(r chi.Router) { a.loadAuthRoutes(r, authService) })
	router.Route("/wallet", func(r chi.Router) { a.loadWalletRoutes(r, authService) })

	a.router = router
}

func (a *App) loadAuthRoutes(router chi.Router, authService *handler.AuthService) {
	router.Post("/register", authService.HandleRegister)
	router.Post("/login", authService.HandleLogin)
}

func (a *App) loadWalletRoutes(router chi.Router, authService *handler.AuthService) {
	router.Use(authService.AuthMiddleware(handler.AuthLevelPassword))

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
