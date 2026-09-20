package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"noble-babbage/internal/config"
	"noble-babbage/internal/database"
	"noble-babbage/internal/plugins"
	"noble-babbage/internal/server"
)

func main() {
	cfg := config.LoadConfig()

	log.Printf("Iniciando servidor HTTP na porta %s...", cfg.Port)
	log.Printf("Banco de dados SQLite: %s", cfg.DatabasePath)
	log.Printf("Usuário admin configurado: %s", cfg.AdminUser)

	db, err := database.NewDB(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("Erro fatal ao inicializar banco de dados: %v", err)
	}
	defer db.Close()

	// Inicializa o gerenciador de plugins dinâmicos
	dataDir := filepath.Dir(cfg.DatabasePath)
	if dataDir == "" || dataDir == "." {
		dataDir = "data"
	}
	pm, err := plugins.NewManager(db, dataDir)
	if err != nil {
		log.Fatalf("Erro fatal ao inicializar gerenciador de plugins: %v", err)
	}

	// Sincroniza e carrega os repositórios previamente cadastrados em background
	repos, err := db.GetAll()
	if err == nil && len(repos) > 0 {
		log.Printf("Sincronizando %d repositório(s) cadastrados em background...", len(repos))
		pm.SyncAllRepos(repos)
	}

	router := server.SetupRouter(cfg, db, pm)

	httpServer := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- fmt.Errorf("erro no servidor HTTP: %w", err)
		}
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		log.Fatalf("Erro fatal: %v", err)
	case sig := <-shutdown:
		log.Printf("Sinal de término recebido (%v). Encerrando servidor com graceful shutdown...", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := httpServer.Shutdown(ctx); err != nil {
			log.Fatalf("Erro durante o encerramento gracioso: %v", err)
		}
		log.Println("Servidor encerrado com sucesso.")
	}
}
