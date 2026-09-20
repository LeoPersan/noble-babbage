package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"noble-babbage/internal/config"
	"noble-babbage/internal/database"
	"noble-babbage/internal/models"
	"noble-babbage/internal/plugins"
	"noble-babbage/internal/server"
)

func setupTestEnvironment(t *testing.T) (*config.Config, *database.DB, http.Handler, func()) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "noble-babbage-test-*")
	if err != nil {
		t.Fatalf("falha ao criar temp dir: %v", err)
	}

	dbPath := filepath.Join(tempDir, "test.db")
	cfg := &config.Config{
		Port:          "8080",
		AdminUser:     "testadmin",
		AdminPassword: "testpassword123",
		DatabasePath:  dbPath,
		SessionSecret: "test-secret-key-12345",
	}

	db, err := database.NewDB(dbPath)
	if err != nil {
		t.Fatalf("falha ao inicializar banco de teste: %v", err)
	}

	pm, err := plugins.NewManager(db, tempDir)
	if err != nil {
		t.Fatalf("falha ao inicializar plugin manager: %v", err)
	}

	router := server.SetupRouter(cfg, db, pm)

	cleanup := func() {
		db.Close()
		os.RemoveAll(tempDir)
	}

	return cfg, db, router, cleanup
}

// -------------------------------------------------------------
// 1. Testes de Extração de Nome de Repositório
// -------------------------------------------------------------

func TestExtractRepoName(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"https://github.com/torvalds/linux.git", "linux"},
		{"https://github.com/torvalds/linux", "linux"},
		{"http://gitlab.com/group/subgroup/awesome-app/", "awesome-app"},
		{"git@github.com:golang/go.git", "go"},
		{"ssh://git@git.empresa.com:22/org/backend-service.git", "backend-service"},
		{"meu-repositorio", "meu-repositorio"},
		{"", "unnamed-repo"},
		{"   ", "unnamed-repo"},
	}

	for _, c := range cases {
		t.Run(c.input, func(t *testing.T) {
			actual := models.ExtractRepoName(c.input)
			if actual != c.expected {
				t.Errorf("ExtractRepoName(%q) = %q, esperado %q", c.input, actual, c.expected)
			}
		})
	}
}

// -------------------------------------------------------------
// 2. Testes de Banco de Dados SQLite (CRUD)
// -------------------------------------------------------------

func TestDatabaseCRUD(t *testing.T) {
	_, db, _, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// 1. Create com nome customizado
	repo1 := &models.Repository{
		Link:      "https://github.com/user/custom-name.git",
		Name:      "Meu Repositório Custom",
		AccessKey: "secret-token-xyz-123456",
	}
	if err := db.Create(repo1); err != nil {
		t.Fatalf("falha ao criar repo1: %v", err)
	}
	if repo1.ID == "" {
		t.Errorf("esperado ID preenchido após Create")
	}

	// 2. Create sem nome (fallback automático)
	repo2 := &models.Repository{
		Link: "https://github.com/user/auto-detected-repo.git",
	}
	if err := db.Create(repo2); err != nil {
		t.Fatalf("falha ao criar repo2: %v", err)
	}
	if repo2.Name != "auto-detected-repo" {
		t.Errorf("esperado nome extraído 'auto-detected-repo', obtido %q", repo2.Name)
	}

	// 3. GetAll
	all, err := db.GetAll()
	if err != nil {
		t.Fatalf("falha ao buscar todos os repos: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("esperado 2 repositórios, obtidos %d", len(all))
	}

	// 4. GetByID
	fetched, err := db.GetByID(repo1.ID)
	if err != nil {
		t.Fatalf("falha ao buscar repo1 por ID: %v", err)
	}
	if fetched.Name != "Meu Repositório Custom" {
		t.Errorf("esperado nome 'Meu Repositório Custom', obtido %q", fetched.Name)
	}
	if fetched.MaskedAccessKey() != "sec...456" {
		t.Errorf("esperado chave mascarada 'sec...456', obtido %q", fetched.MaskedAccessKey())
	}

	// 5. Update
	fetched.Name = "Nome Atualizado"
	fetched.AccessKey = "nova-chave"
	if err := db.Update(fetched); err != nil {
		t.Fatalf("falha ao atualizar repo: %v", err)
	}
	updated, _ := db.GetByID(repo1.ID)
	if updated.Name != "Nome Atualizado" || updated.AccessKey != "nova-chave" {
		t.Errorf("dados não atualizados corretamente: %+v", updated)
	}

	// 6. Delete
	if err := db.Delete(repo1.ID); err != nil {
		t.Fatalf("falha ao deletar repo: %v", err)
	}
	_, err = db.GetByID(repo1.ID)
	if err != database.ErrNotFound {
		t.Errorf("esperado ErrNotFound após delete, obtido: %v", err)
	}
}

// -------------------------------------------------------------
// 3. Testes de Autenticação e Rotas /admin
// -------------------------------------------------------------

func TestAuthAndAdminRoutes(t *testing.T) {
	cfg, _, router, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// 1. Acesso a /admin sem login deve redirecionar para /admin/login
	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Errorf("esperado redirect (303), obtido %d", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Location"), "/admin/login") {
		t.Errorf("esperado redirecionamento para /admin/login, obtido %q", rec.Header().Get("Location"))
	}

	// 2. Acesso à API REST sem autenticação deve retornar 401 Unauthorized
	req = httptest.NewRequest(http.MethodGet, "/admin/api/repos", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("esperado 401 Unauthorized na API, obtido %d", rec.Code)
	}

	// 3. Login com credenciais inválidas
	form := url.Values{}
	form.Set("username", "wrong")
	form.Set("password", "wrong")
	req = httptest.NewRequest(http.MethodPost, "/admin/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("esperado 401 para credenciais incorretas, obtido %d", rec.Code)
	}

	// 4. Login com credenciais válidas
	form.Set("username", cfg.AdminUser)
	form.Set("password", cfg.AdminPassword)
	req = httptest.NewRequest(http.MethodPost, "/admin/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("esperado redirect (303) no login bem-sucedido, obtido %d", rec.Code)
	}

	cookie := rec.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookie {
		if c.Name == "admin_session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatalf("cookie admin_session não encontrado na resposta de login")
	}

	// 5. Acesso autenticado a /admin com o cookie de sessão
	req = httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.AddCookie(sessionCookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("esperado 200 OK em /admin autenticado, obtido %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Painel de Controle /admin") {
		t.Errorf("corpo da resposta de /admin não contém texto esperado")
	}

	// 6. Acesso à API com HTTP Basic Auth
	req = httptest.NewRequest(http.MethodGet, "/admin/api/repos", nil)
	req.SetBasicAuth(cfg.AdminUser, cfg.AdminPassword)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("esperado 200 OK na API com Basic Auth, obtido %d", rec.Code)
	}

	// 7. Logout limpa cookie
	req = httptest.NewRequest(http.MethodGet, "/admin/logout", nil)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Errorf("esperado redirect após logout, obtido %d", rec.Code)
	}
}

// -------------------------------------------------------------
// 4. Testes Completos de Endpoints REST API (/admin/api/repos)
// -------------------------------------------------------------

func TestAPIRepositoryEndpoints(t *testing.T) {
	cfg, _, router, cleanup := setupTestEnvironment(t)
	defer cleanup()

	authHeader := func(req *http.Request) {
		req.SetBasicAuth(cfg.AdminUser, cfg.AdminPassword)
	}

	// 1. Criar repositório via POST API com nome fornecido
	payload := `{"link":"https://github.com/myorg/project-a.git","name":"Projeto Alpha","access_key":"secret123"}`
	req := httptest.NewRequest(http.MethodPost, "/admin/api/repos", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	authHeader(req)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("esperado 201 Created na API, obtido %d: %s", rec.Code, rec.Body.String())
	}

	var created models.Repository
	if err := json.NewDecoder(rec.Body).Decode(&created); err != nil {
		t.Fatalf("falha ao decodificar resposta JSON: %v", err)
	}
	if created.ID == "" || created.Name != "Projeto Alpha" || created.Link != "https://github.com/myorg/project-a.git" {
		t.Errorf("dados criados incorretos: %+v", created)
	}

	// 2. Criar repositório sem nome (fallback automático para nome do repo)
	payload2 := `{"link":"https://github.com/myorg/project-beta.git"}`
	req = httptest.NewRequest(http.MethodPost, "/admin/api/repos", bytes.NewBufferString(payload2))
	req.Header.Set("Content-Type", "application/json")
	authHeader(req)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("esperado 201 Created na API para fallback de nome, obtido %d: %s", rec.Code, rec.Body.String())
	}
	var created2 models.Repository
	json.NewDecoder(rec.Body).Decode(&created2)
	if created2.Name != "project-beta" {
		t.Errorf("esperado nome extraído 'project-beta', obtido %q", created2.Name)
	}

	// 3. Listar todos os repositórios via GET API
	req = httptest.NewRequest(http.MethodGet, "/admin/api/repos", nil)
	authHeader(req)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var list []models.Repository
	json.NewDecoder(rec.Body).Decode(&list)
	if len(list) != 2 {
		t.Errorf("esperado lista com 2 itens, obtido %d", len(list))
	}

	// 4. Buscar por ID via GET API
	req = httptest.NewRequest(http.MethodGet, "/admin/api/repos/"+created.ID, nil)
	authHeader(req)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("esperado 200 OK ao buscar por ID, obtido %d", rec.Code)
	}

	// 5. Atualizar via PUT API
	updatePayload := `{"name":"Projeto Alpha Renovado","access_key":"nova-chave-456"}`
	req = httptest.NewRequest(http.MethodPut, "/admin/api/repos/"+created.ID, bytes.NewBufferString(updatePayload))
	req.Header.Set("Content-Type", "application/json")
	authHeader(req)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200 OK ao atualizar, obtido %d: %s", rec.Code, rec.Body.String())
	}

	// 6. Deletar via DELETE API
	req = httptest.NewRequest(http.MethodDelete, "/admin/api/repos/"+created.ID, nil)
	authHeader(req)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("esperado 200 OK ao excluir via API, obtido %d", rec.Code)
	}
}

// -------------------------------------------------------------
// 5. Testes de Ações CRUD na Web UI (/admin/repos...)
// -------------------------------------------------------------

func TestWebRepositoryActions(t *testing.T) {
	cfg, _, router, cleanup := setupTestEnvironment(t)
	defer cleanup()

	// Autentica via login
	form := url.Values{}
	form.Set("username", cfg.AdminUser)
	form.Set("password", cfg.AdminPassword)
	req := httptest.NewRequest(http.MethodPost, "/admin/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	var sessionCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == "admin_session" {
			sessionCookie = c
			break
		}
	}

	// 1. Criar repositório via formulário Web
	repoForm := url.Values{}
	repoForm.Set("link", "https://github.com/user/web-created-repo.git")
	repoForm.Set("name", "") // Deixa vazio para testar extração
	repoForm.Set("access_key", "key-xyz")

	req = httptest.NewRequest(http.MethodPost, "/admin/repos", strings.NewReader(repoForm.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(sessionCookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("esperado redirect (303) ao cadastrar via Web, obtido %d", rec.Code)
	}

	// 2. Verifica listagem na página HTML
	req = httptest.NewRequest(http.MethodGet, "/admin", nil)
	req.AddCookie(sessionCookie)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	body := rec.Body.String()
	if !strings.Contains(body, "web-created-repo") {
		t.Errorf("esperado nome 'web-created-repo' na página HTML /admin, corpo: %s", body)
	}
}
