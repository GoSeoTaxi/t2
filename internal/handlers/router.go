package handlers

import (
	"context"
	"github.com/GoSeoTaxi/t1/internal/storage"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/jwtauth/v5"
	"go.uber.org/zap"
)

// BonusRouter arranges the whole API endpoints and their correponding handlers
func BonusRouter(ctx context.Context, db storage.DBinterface, secret string, logger *zap.Logger) chi.Router {

	r := chi.NewRouter()
	mh := NewHandler(ctx, db, logger)
	tokenAuth := jwtauth.New("HS256", []byte(secret), nil)

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(jwtauth.Verifier(tokenAuth))

	r.Route("/api/user/", func(r chi.Router) {
		r.Post("/register", Conveyor(mh.HandlerPostRegister(tokenAuth), unpackGZIP, checkForJSON))
		r.Post("/login", Conveyor(mh.HandlerPostLogin(tokenAuth), unpackGZIP, checkForJSON))
		r.With(jwtauth.Authenticator).Post("/orders", Conveyor(mh.HandlerPostOrders(), unpackGZIP, checkForText))
		r.With(jwtauth.Authenticator).Get("/orders", Conveyor(mh.HandlerGetOrders(), unpackGZIP, packGZIP))
		r.With(jwtauth.Authenticator).Route("/balance", func(r chi.Router) {
			r.Get("/", Conveyor(mh.HandlerGetBalance(), unpackGZIP))
			r.Post("/withdraw", Conveyor(mh.HandlerPostWithdraw(), unpackGZIP))
			r.Get("/withdrawals", Conveyor(mh.HandlerGetWithdrawals(), unpackGZIP))
		})

		r.With(jwtauth.Authenticator).Post("/withdraw", Conveyor(mh.HandlerPostWithdraw(), unpackGZIP))
		r.With(jwtauth.Authenticator).Get("/withdrawals", Conveyor(mh.HandlerPostWithdraw(), unpackGZIP))

	})

	return r
}
