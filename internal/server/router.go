package server

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"noble-babbage/internal/auth"
	"noble-babbage/internal/config"
	"noble-babbage/internal/database"
	"noble-babbage/internal/handlers"
	"noble-babbage/internal/plugins"
)

func SetupRouter(cfg *config.Config, db *database.DB, pm *plugins.Manager) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// Middleware dinâmico para despachar requisições para os plugins Go carregados
	if pm != nil {
		r.Use(pm.DynamicRouterMiddleware)
	}

	adminHandler := handlers.NewAdminHandler(cfg, db, pm)

	// Healthcheck
	healthHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if r.Method != http.MethodHead {
			w.Write([]byte(`{"status":"ok"}`))
		}
	}
	r.Get("/health", healthHandler)
	r.Head("/health", healthHandler)

	// Redireciona raiz para /admin
	rootRedirect := func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
	}
	r.Get("/", rootRedirect)
	r.Head("/", rootRedirect)

	// Rotas sob /admin
	r.Route("/admin", func(admin chi.Router) {
		// Rotas públicas de login
		admin.Get("/login", adminHandler.LoginPage)
		admin.Head("/login", adminHandler.LoginPage)
		admin.Post("/login", adminHandler.LoginSubmit)
		admin.Get("/logout", adminHandler.Logout)

		// Rotas protegidas por autenticação
		admin.Group(func(protected chi.Router) {
			protected.Use(auth.AuthMiddleware(cfg))

			// Web UI
			protected.Get("/", adminHandler.AdminDashboard)
			protected.Head("/", adminHandler.AdminDashboard)
			protected.Post("/repos", adminHandler.CreateRepoWeb)
			protected.Post("/repos/{id}/sync", adminHandler.SyncRepoWeb)
			protected.Post("/repos/{id}/edit", adminHandler.EditRepoWeb)
			protected.Post("/repos/{id}/delete", adminHandler.DeleteRepoWeb)

			// REST API
			protected.Route("/api/repos", func(api chi.Router) {
				api.Get("/", adminHandler.APIGetAll)
				api.Head("/", adminHandler.APIGetAll)
				api.Post("/", adminHandler.APICreate)
				api.Get("/{id}", adminHandler.APIGetByID)
				api.Head("/{id}", adminHandler.APIGetByID)
				api.Post("/{id}/sync", adminHandler.APISync)
				api.Put("/{id}", adminHandler.APIUpdate)
				api.Delete("/{id}", adminHandler.APIDelete)
			})
		})
	})

	return r
}
