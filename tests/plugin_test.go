package tests

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"noble-babbage/internal/config"
	"noble-babbage/internal/database"
	"noble-babbage/internal/models"
	"noble-babbage/internal/plugins"
	"noble-babbage/internal/server"
)

// TestPluginLifecycleAndRouting testa a compilação de um plugin Go .so, carregamento e despacho de rotas
func TestPluginLifecycleAndRouting(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "noble-babbage-plugin-test-*")
	if err != nil {
		t.Fatalf("falha ao criar temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	cfg := &config.Config{
		Port:          "8080",
		AdminUser:     "admin",
		AdminPassword: "admin",
		DatabasePath:  dbPath,
		SessionSecret: "secret-session-key",
	}

	db, err := database.NewDB(dbPath)
	if err != nil {
		t.Fatalf("falha ao inicializar banco de teste: %v", err)
	}
	defer db.Close()

	pm, err := plugins.NewManager(db, tempDir)
	if err != nil {
		t.Fatalf("falha ao inicializar plugin manager: %v", err)
	}

	router := server.SetupRouter(cfg, db, pm)

	// 1. Cria diretório de código-fonte de um plugin Go válido
	sampleRepoDir := filepath.Join(tempDir, "sample_plugin_src")
	if err := os.MkdirAll(sampleRepoDir, 0755); err != nil {
		t.Fatalf("falha ao criar sampleRepoDir: %v", err)
	}

	// Obtém o caminho absoluto do workspace atual para referenciar o módulo no replace
	workspaceAbs, _ := filepath.Abs("..")

	goModContent := `module sample-plugin

go 1.24

require (
	github.com/go-chi/chi/v5 v5.2.1
	noble-babbage v0.0.0
)

replace noble-babbage => ` + workspaceAbs + `
`
	if err := os.WriteFile(filepath.Join(sampleRepoDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		t.Fatalf("falha ao escrever go.mod do plugin: %v", err)
	}

	if sumBytes, err := os.ReadFile(filepath.Join(workspaceAbs, "go.sum")); err == nil {
		_ = os.WriteFile(filepath.Join(sampleRepoDir, "go.sum"), sumBytes, 0644)
	}

	mainGoContent := `package main

import (
	"net/http"
	"github.com/go-chi/chi/v5"
	pluginSDK "noble-babbage/pkg/plugin"
)

type SamplePlugin struct{}

func (s *SamplePlugin) Name() string {
	return "Sample Plugin Test"
}

func (s *SamplePlugin) BasePath() string {
	return "/sample-plugin"
}

func (s *SamplePlugin) RegisterRoutes(r chi.Router) {
	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte("<h1>Olá do Sample Plugin!</h1>"))
	})
	r.Get("/api/status", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(` + "`" + `{"plugin":"sample","status":"online"}` + "`" + `))
	})
}

var Plugin pluginSDK.RoutePlugin = &SamplePlugin{}
`
	if err := os.WriteFile(filepath.Join(sampleRepoDir, "main.go"), []byte(mainGoContent), 0644); err != nil {
		t.Fatalf("falha ao escrever main.go do plugin: %v", err)
	}

	// 2. Cadastra o repositório no banco
	repo := &models.Repository{
		Link:   sampleRepoDir,
		Name:   "sample-plugin",
		Status: models.StatusPending,
	}
	if err := db.Create(repo); err != nil {
		t.Fatalf("falha ao cadastrar repositório no banco: %v", err)
	}

	// 3. Executa a sincronização e build do plugin
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := pm.SyncAndBuild(ctx, repo); err != nil {
		t.Fatalf("falha no SyncAndBuild do plugin: %v", err)
	}

	// 4. Verifica status ativo no banco
	fetched, err := db.GetByID(repo.ID)
	if err != nil {
		t.Fatalf("falha ao buscar repo: %v", err)
	}
	if fetched.Status != models.StatusActive {
		t.Errorf("esperado status 'active', obtido %q (erro: %s)", fetched.Status, fetched.LastBuildError)
	}
	if fetched.BasePath != "/sample-plugin" {
		t.Errorf("esperado BasePath '/sample-plugin', obtido %q", fetched.BasePath)
	}

	// 5. Testa chamada HTTP à página web do plugin (/sample-plugin)
	req := httptest.NewRequest(http.MethodGet, "/sample-plugin", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200 OK na rota do plugin, obtido %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Olá do Sample Plugin!") {
		t.Errorf("resposta da rota do plugin inesperada: %s", rec.Body.String())
	}

	// 6. Testa chamada HTTP à sub-rota API do plugin (/sample-plugin/api/status)
	req = httptest.NewRequest(http.MethodGet, "/sample-plugin/api/status", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200 OK na API do plugin, obtido %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"status":"online"`) {
		t.Errorf("resposta da API do plugin inesperada: %s", rec.Body.String())
	}
}

// TestSSHKeyAndTokenHandling testa se chaves SSH privadas e tokens HTTPS são processados sem pânico
func TestSSHKeyAndTokenHandling(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "noble-babbage-auth-test-*")
	if err != nil {
		t.Fatalf("falha ao criar temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	db, err := database.NewDB(dbPath)
	if err != nil {
		t.Fatalf("falha ao inicializar db: %v", err)
	}
	defer db.Close()

	pm, err := plugins.NewManager(db, tempDir)
	if err != nil {
		t.Fatalf("falha ao inicializar pm: %v", err)
	}

	// 1. Testa repositório com chave SSH privada em AccessKey
	fakeSSHKey := "-----BEGIN OPENSSH PRIVATE KEY-----\nb3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW\n-----END OPENSSH PRIVATE KEY-----"
	repoSSH := &models.Repository{
		Link:      "git@github.com:example/private-repo.git",
		Name:      "private-repo",
		AccessKey: fakeSSHKey,
		Status:    models.StatusPending,
	}
	_ = db.Create(repoSSH)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// SyncAndBuild tentará clonar com a chave SSH injetada
	_ = pm.SyncAndBuild(ctx, repoSSH)

	// Verifica se a chave privada foi gravada com segurança no diretório keys
	keyPath := filepath.Join(tempDir, "keys", fmt.Sprintf("id_%s", repoSSH.ID))
	if info, err := os.Stat(keyPath); err == nil {
		if info.Mode().Perm() != 0600 {
			t.Errorf("esperado permissão 0600 na chave SSH privada, obtido %v", info.Mode().Perm())
		}
	} else {
		t.Errorf("arquivo de chave SSH não foi criado: %v", err)
	}
}

