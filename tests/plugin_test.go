package tests

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
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

// TestPluginUpdateAndHotReload valida que um plugin pode ser atualizado e imediatamente recarregado no roteador ativo
func TestPluginUpdateAndHotReload(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "noble-babbage-reload-test-*")
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
		t.Fatalf("falha ao inicializar banco: %v", err)
	}
	defer db.Close()

	pm, err := plugins.NewManager(db, tempDir)
	if err != nil {
		t.Fatalf("falha ao inicializar manager: %v", err)
	}

	router := server.SetupRouter(cfg, db, pm)
	workspaceAbs, _ := filepath.Abs("..")

	pluginSrcDir := filepath.Join(tempDir, "reload_plugin_src")
	if err := os.MkdirAll(pluginSrcDir, 0755); err != nil {
		t.Fatalf("falha ao criar pluginSrcDir: %v", err)
	}

	goModContent := `module reload-plugin

go 1.24

require (
	github.com/go-chi/chi/v5 v5.2.1
	noble-babbage v0.0.0
)

replace noble-babbage => ` + workspaceAbs + `
`
	_ = os.WriteFile(filepath.Join(pluginSrcDir, "go.mod"), []byte(goModContent), 0644)
	if sumBytes, err := os.ReadFile(filepath.Join(workspaceAbs, "go.sum")); err == nil {
		_ = os.WriteFile(filepath.Join(pluginSrcDir, "go.sum"), sumBytes, 0644)
	}

	// Código da Versão 1
	v1Code := `package main

import (
	"net/http"
	"github.com/go-chi/chi/v5"
	pluginSDK "noble-babbage/pkg/plugin"
)

type ReloadPlugin struct{}

func (p *ReloadPlugin) Name() string { return "Reload Plugin" }
func (p *ReloadPlugin) BasePath() string { return "/reload-test" }
func (p *ReloadPlugin) RegisterRoutes(r chi.Router) {
	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte("Versão 1 do Plugin"))
	})
}

var Plugin pluginSDK.RoutePlugin = &ReloadPlugin{}
`
	if err := os.WriteFile(filepath.Join(pluginSrcDir, "main.go"), []byte(v1Code), 0644); err != nil {
		t.Fatalf("falha ao criar v1 main.go: %v", err)
	}

	repo := &models.Repository{
		Link:   pluginSrcDir,
		Name:   "reload-plugin",
		Status: models.StatusPending,
	}
	if err := db.Create(repo); err != nil {
		t.Fatalf("falha ao criar repo no banco: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// 1. Primeira sincronização e build (v1)
	if err := pm.SyncAndBuild(ctx, repo); err != nil {
		t.Fatalf("SyncAndBuild v1 falhou: %v", err)
	}

	req1 := httptest.NewRequest(http.MethodGet, "/reload-test", nil)
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Fatalf("esperado 200 OK na rota v1, obtido %d", rec1.Code)
	}
	if !strings.Contains(rec1.Body.String(), "Versão 1 do Plugin") {
		t.Fatalf("resposta v1 inesperada: %s", rec1.Body.String())
	}

	// 2. Atualiza o código-fonte para a Versão 2
	v2Code := `package main

import (
	"net/http"
	"github.com/go-chi/chi/v5"
	pluginSDK "noble-babbage/pkg/plugin"
)

type ReloadPlugin struct{}

func (p *ReloadPlugin) Name() string { return "Reload Plugin V2" }
func (p *ReloadPlugin) BasePath() string { return "/reload-test" }
func (p *ReloadPlugin) RegisterRoutes(r chi.Router) {
	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Write([]byte("Versão 2 do Plugin Atualizada!"))
	})
}

var Plugin pluginSDK.RoutePlugin = &ReloadPlugin{}
`
	if err := os.WriteFile(filepath.Join(pluginSrcDir, "main.go"), []byte(v2Code), 0644); err != nil {
		t.Fatalf("falha ao atualizar para v2 main.go: %v", err)
	}

	// 3. Segunda sincronização e build (v2) - deve recompilar e atualizar sem erro de 'already loaded'
	if err := pm.SyncAndBuild(ctx, repo); err != nil {
		t.Fatalf("SyncAndBuild v2 falhou: %v", err)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/reload-test", nil)
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Fatalf("esperado 200 OK na rota v2, obtido %d", rec2.Code)
	}
	if !strings.Contains(rec2.Body.String(), "Versão 2 do Plugin Atualizada!") {
		t.Fatalf("esperado retorno da versão 2 atualizada, obtido: %s", rec2.Body.String())
	}

	// Valida que o status no banco está ativo sem erros
	fetched, err := db.GetByID(repo.ID)
	if err != nil {
		t.Fatalf("falha ao buscar repo: %v", err)
	}
	if fetched.Status != models.StatusActive {
		t.Errorf("status inesperado no banco: %q (erro: %s)", fetched.Status, fetched.LastBuildError)
	}
}

// TestGitSyncDirtyReset valida que git pull com reset --hard HEAD descarta modificações locais e atualiza o repositório
func TestGitSyncDirtyReset(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "noble-babbage-git-reset-*")
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

	// 1. Cria um repositório git remoto bare e um clone dev
	remoteDir := filepath.Join(tempDir, "remote.git")
	devDir := filepath.Join(tempDir, "dev_repo")

	runCmd := func(dir string, name string, args ...string) {
		t.Helper()
		c := exec.Command(name, args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("comando '%s %v' falhou: %v: %s", name, args, err, string(out))
		}
	}

	runCmd(tempDir, "git", "init", "--bare", "-b", "main", remoteDir)
	runCmd(tempDir, "git", "clone", remoteDir, devDir)
	runCmd(devDir, "git", "config", "user.name", "Tester")
	runCmd(devDir, "git", "config", "user.email", "tester@example.com")

	workspaceAbs, _ := filepath.Abs("..")
	goModContent := `module git-reset-plugin

go 1.24

require (
	github.com/go-chi/chi/v5 v5.2.1
	noble-babbage v0.0.0
)

replace noble-babbage => ` + workspaceAbs + `
`
	_ = os.WriteFile(filepath.Join(devDir, "go.mod"), []byte(goModContent), 0644)
	if sumBytes, err := os.ReadFile(filepath.Join(workspaceAbs, "go.sum")); err == nil {
		_ = os.WriteFile(filepath.Join(devDir, "go.sum"), sumBytes, 0644)
	}

	mainContentV1 := `package main

import (
	"net/http"
	"github.com/go-chi/chi/v5"
	pluginSDK "noble-babbage/pkg/plugin"
)

type GitResetPlugin struct{}

func (p *GitResetPlugin) Name() string { return "Git Reset Plugin" }
func (p *GitResetPlugin) BasePath() string { return "/git-reset" }
func (p *GitResetPlugin) RegisterRoutes(r chi.Router) {
	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("Commit V1"))
	})
}

var Plugin pluginSDK.RoutePlugin = &GitResetPlugin{}
`
	_ = os.WriteFile(filepath.Join(devDir, "main.go"), []byte(mainContentV1), 0644)
	runCmd(devDir, "git", "add", ".")
	runCmd(devDir, "git", "commit", "-m", "Initial commit v1")
	runCmd(devDir, "git", "push", "origin", "main")

	// 2. Cadastra repositório no noble-babbage apontando para o remote
	repo := &models.Repository{
		Link:   "file://" + remoteDir,
		Name:   "git-reset-plugin",
		Status: models.StatusPending,
	}
	_ = db.Create(repo)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// 3. Primeira sincronização e build do repositório
	if err := pm.SyncAndBuild(ctx, repo); err != nil {
		t.Fatalf("primeira sincronização falhou: %v", err)
	}

	// 4. Suja propositalmente a working tree do repositório clonado pelo noble-babbage
	clonedRepoDir := filepath.Join(tempDir, "repos", repo.ID)
	_ = os.WriteFile(filepath.Join(clonedRepoDir, "go.mod"), []byte("// dirty modified file\n"), 0644)
	_ = os.WriteFile(filepath.Join(clonedRepoDir, "untracked_file.tmp"), []byte("dirty untracked"), 0644)

	// 5. Gera um novo commit v2 no repositório remoto
	mainContentV2 := `package main

import (
	"net/http"
	"github.com/go-chi/chi/v5"
	pluginSDK "noble-babbage/pkg/plugin"
)

type GitResetPlugin struct{}

func (p *GitResetPlugin) Name() string { return "Git Reset Plugin V2" }
func (p *GitResetPlugin) BasePath() string { return "/git-reset" }
func (p *GitResetPlugin) RegisterRoutes(r chi.Router) {
	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("Commit V2 Updated"))
	})
}

var Plugin pluginSDK.RoutePlugin = &GitResetPlugin{}
`
	_ = os.WriteFile(filepath.Join(devDir, "main.go"), []byte(mainContentV2), 0644)
	runCmd(devDir, "git", "commit", "-am", "Commit v2")
	runCmd(devDir, "git", "push", "origin", "main")

	// 6. Executa a segunda sincronização: deve resetar as alterações locais e atualizar para v2
	if err := pm.SyncAndBuild(ctx, repo); err != nil {
		t.Fatalf("segunda sincronização com dirty working tree falhou: %v", err)
	}

	// 7. Valida que o código atualizado em clonedRepoDir reflete o commit v2
	updatedMain, err := os.ReadFile(filepath.Join(clonedRepoDir, "main.go"))
	if err != nil {
		t.Fatalf("falha ao ler main.go atualizado: %v", err)
	}
	if !strings.Contains(string(updatedMain), "Commit V2 Updated") {
		t.Fatalf("repositório não foi atualizado para v2: %s", string(updatedMain))
	}
}


