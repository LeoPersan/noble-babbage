package plugins

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	goplugin "plugin"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"noble-babbage/internal/database"
	"noble-babbage/internal/models"
	pluginSDK "noble-babbage/pkg/plugin"
)

type LoadedPlugin struct {
	RepoID   string
	Name     string
	BasePath string
	Instance pluginSDK.RoutePlugin
	Router   *chi.Mux
	LoadedAt time.Time
}

type Manager struct {
	mu            sync.RWMutex
	plugins       map[string]*LoadedPlugin // keyed by RepoID
	buildingMu    sync.Mutex
	buildingRepos map[string]bool // deduplica builds concorrentes para o mesmo repo
	db            *database.DB
	reposDir      string
	pluginsDir    string
}

func NewManager(db *database.DB, dataDir string) (*Manager, error) {
	reposDir := filepath.Join(dataDir, "repos")
	pluginsDir := filepath.Join(dataDir, "plugins")

	if err := os.MkdirAll(reposDir, 0755); err != nil {
		return nil, fmt.Errorf("falha ao criar reposDir: %w", err)
	}
	if err := os.MkdirAll(pluginsDir, 0755); err != nil {
		return nil, fmt.Errorf("falha ao criar pluginsDir: %w", err)
	}

	return &Manager{
		plugins:       make(map[string]*LoadedPlugin),
		buildingRepos: make(map[string]bool),
		db:            db,
		reposDir:      reposDir,
		pluginsDir:    pluginsDir,
	}, nil
}

// SyncAndBuild clona/atualiza o repositório, compila como .so e carrega dinamicamente
func (m *Manager) SyncAndBuild(ctx context.Context, repo *models.Repository) error {
	m.buildingMu.Lock()
	if m.buildingRepos[repo.ID] {
		m.buildingMu.Unlock()
		log.Printf("[PluginManager] Sincronização para '%s' (%s) já em andamento, ignorando chamada concorrente.", repo.Name, repo.ID)
		return nil
	}
	m.buildingRepos[repo.ID] = true
	m.buildingMu.Unlock()

	defer func() {
		m.buildingMu.Lock()
		delete(m.buildingRepos, repo.ID)
		m.buildingMu.Unlock()
	}()

	log.Printf("[PluginManager] Iniciando sincronização e build para o repositório '%s' (%s)...", repo.Name, repo.ID)

	// Atualiza status para 'building' no banco
	now := time.Now().UTC()
	_ = m.db.UpdateBuildStatus(repo.ID, models.StatusBuilding, repo.BasePath, "", &now)

	repoPath, err := m.syncGitRepo(ctx, repo)
	if err != nil {
		errStr := fmt.Sprintf("Erro ao sincronizar git: %v", err)
		log.Printf("[PluginManager] %s", errStr)
		_ = m.db.UpdateBuildStatus(repo.ID, models.StatusError, repo.BasePath, errStr, &now)
		return errors.New(errStr)
	}

	soPath, err := m.buildPlugin(ctx, repoPath, repo.ID)
	if err != nil {
		errStr := fmt.Sprintf("Erro ao compilar plugin Go: %v", err)
		log.Printf("[PluginManager] %s", errStr)
		_ = m.db.UpdateBuildStatus(repo.ID, models.StatusError, repo.BasePath, errStr, &now)
		return errors.New(errStr)
	}

	loaded, err := m.loadPluginFile(soPath, repo)
	if err != nil {
		errStr := fmt.Sprintf("Erro ao carregar .so: %v", err)
		log.Printf("[PluginManager] %s", errStr)
		_ = m.db.UpdateBuildStatus(repo.ID, models.StatusError, repo.BasePath, errStr, &now)
		return errors.New(errStr)
	}

	// Atualiza status ativo no banco
	_ = m.db.UpdateBuildStatus(repo.ID, models.StatusActive, loaded.BasePath, "", &now)
	log.Printf("[PluginManager] Plugin '%s' montado com sucesso em '%s'!", loaded.Name, loaded.BasePath)

	return nil
}

func (m *Manager) syncGitRepo(ctx context.Context, repo *models.Repository) (string, error) {
	// Suporte para diretório local no host (ótimo para testes e desenvolvimento)
	if strings.HasPrefix(repo.Link, "/") || strings.HasPrefix(repo.Link, "./") || strings.HasPrefix(repo.Link, "../") {
		absPath, err := filepath.Abs(repo.Link)
		if err == nil && fileExists(absPath) && fileExists(filepath.Join(absPath, "go.mod")) {
			return absPath, nil
		}
	}

	targetDir := filepath.Join(m.reposDir, repo.ID)
	cloneURL := repo.Link

	var extraEnv []string

	// Suporte para chave privada SSH inserida diretamente no campo AccessKey
	if repo.AccessKey != "" && strings.Contains(repo.AccessKey, "PRIVATE KEY") {
		keysDir := filepath.Join(filepath.Dir(m.reposDir), "keys")
		_ = os.MkdirAll(keysDir, 0700)
		keyPath := filepath.Join(keysDir, fmt.Sprintf("id_%s", repo.ID))

		keyContent := normalizePrivateKey(repo.AccessKey)
		if err := os.WriteFile(keyPath, []byte(keyContent), 0600); err == nil {
			extraEnv = append(extraEnv, fmt.Sprintf("GIT_SSH_COMMAND=ssh -i %s -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null", keyPath))
		}
	} else if strings.HasPrefix(repo.Link, "git@") || strings.HasPrefix(repo.Link, "ssh://") {
		// Evita travamento interativo na confirmação de host keys em background
		extraEnv = append(extraEnv, "GIT_SSH_COMMAND=ssh -o StrictHostKeyChecking=accept-new")
	} else if repo.AccessKey != "" && (strings.HasPrefix(repo.Link, "https://") || strings.HasPrefix(repo.Link, "http://")) {
		// Injeta AccessKey (Personal Access Token) na URL HTTPS
		if u, err := url.Parse(repo.Link); err == nil {
			u.User = url.UserPassword("x-access-token", repo.AccessKey)
			cloneURL = u.String()
		}
	}

	if fileExists(filepath.Join(targetDir, ".git")) {
		// Repositório já existe: limpa alterações locais geradas por builds anteriores (go mod edit/tidy, etc.)
		resetHardCmd := exec.CommandContext(ctx, "git", "reset", "--hard", "HEAD")
		resetHardCmd.Dir = targetDir
		resetHardCmd.Env = append(os.Environ(), extraEnv...)
		_ = resetHardCmd.Run()

		cleanCmd := exec.CommandContext(ctx, "git", "clean", "-fd")
		cleanCmd.Dir = targetDir
		cleanCmd.Env = append(os.Environ(), extraEnv...)
		_ = cleanCmd.Run()

		// Busca atualizações de todos os remotos
		fetchCmd := exec.CommandContext(ctx, "git", "fetch", "--all", "--prune")
		fetchCmd.Dir = targetDir
		fetchCmd.Env = append(os.Environ(), extraEnv...)
		var fetchOut bytes.Buffer
		fetchCmd.Stdout = &fetchOut
		fetchCmd.Stderr = &fetchOut
		if err := fetchCmd.Run(); err != nil {
			log.Printf("[PluginManager] git fetch avisou para %s (%s): %v: %s", repo.Name, repo.ID, err, fetchOut.String())
		}

		// Tenta realizar o pull das novas alterações
		pullCmd := exec.CommandContext(ctx, "git", "pull", "--ff-only")
		pullCmd.Dir = targetDir
		pullCmd.Env = append(os.Environ(), extraEnv...)
		var pullOut bytes.Buffer
		pullCmd.Stdout = &pullOut
		pullCmd.Stderr = &pullOut

		if err := pullCmd.Run(); err != nil {
			// Se o pull falhar (ex: histórico divergente), força reset para o upstream ou origin/HEAD
			upstreamReset := exec.CommandContext(ctx, "git", "reset", "--hard", "@{u}")
			upstreamReset.Dir = targetDir
			upstreamReset.Env = append(os.Environ(), extraEnv...)
			if uErr := upstreamReset.Run(); uErr != nil {
				originReset := exec.CommandContext(ctx, "git", "reset", "--hard", "origin/HEAD")
				originReset.Dir = targetDir
				originReset.Env = append(os.Environ(), extraEnv...)
				_ = originReset.Run()
			}
		}
	} else {
		// Repositório novo, realiza clone
		_ = os.RemoveAll(targetDir)
		cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", cloneURL, targetDir)
		cmd.Env = append(os.Environ(), extraEnv...)
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &out
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("%v: %s", err, out.String())
		}
	}

	return targetDir, nil
}

func (m *Manager) buildPlugin(ctx context.Context, repoPath, repoID string) (string, error) {
	ts := time.Now().UnixNano()
	outputPath := filepath.Join(m.pluginsDir, fmt.Sprintf("plugin_%s_%d.so", repoID, ts))
	appRoot := findAppRoot()

	// Prepara e isola o módulo do plugin gerando escopo único para subpacotes e ajustando replaces do SDK
	if err := preparePluginModule(ctx, repoPath, repoID, ts, appRoot); err != nil {
		return "", fmt.Errorf("falha ao preparar modulo do plugin: %w", err)
	}

	pluginPkg := fmt.Sprintf("plugin_%s_%d", sanitizePkgName(repoID), ts)

	// Compila o plugin com -buildmode=plugin e identificador único de pacote para permitir hot-reload
	buildCmd := exec.CommandContext(ctx, "go", "build",
		"-buildmode=plugin",
		"-gcflags=-p="+pluginPkg,
		"-ldflags=-pluginpath="+pluginPkg,
		"-o", outputPath,
		".",
	)
	buildCmd.Dir = repoPath
	buildCmd.Env = append(os.Environ(), "CGO_ENABLED=1")

	var out bytes.Buffer
	buildCmd.Stdout = &out
	buildCmd.Stderr = &out

	if err := buildCmd.Run(); err != nil {
		return "", fmt.Errorf("%v: %s", err, out.String())
	}

	return outputPath, nil
}

func (m *Manager) loadPluginFile(soPath string, repo *models.Repository) (*LoadedPlugin, error) {
	p, err := goplugin.Open(soPath)
	if err != nil {
		return nil, fmt.Errorf("falha ao abrir plugin: %w", err)
	}

	// Procura pelo símbolo exportado 'Plugin' ou função construtora 'NewPlugin'
	var routePlugin pluginSDK.RoutePlugin

	if sym, err := p.Lookup("Plugin"); err == nil {
		if inst, ok := sym.(pluginSDK.RoutePlugin); ok {
			routePlugin = inst
		} else if ptr, ok := sym.(*pluginSDK.RoutePlugin); ok && ptr != nil {
			routePlugin = *ptr
		}
	}

	if routePlugin == nil {
		if sym, err := p.Lookup("NewPlugin"); err == nil {
			if fn, ok := sym.(func() pluginSDK.RoutePlugin); ok {
				routePlugin = fn()
			}
		}
	}

	if routePlugin == nil {
		return nil, errors.New("o plugin compilado não exporta uma variável 'Plugin' ou função 'NewPlugin()' que implemente a interface pkg/plugin.RoutePlugin")
	}

	basePath := strings.TrimSpace(routePlugin.BasePath())
	if basePath == "" {
		basePath = repo.EffectiveBasePath()
	}
	if !strings.HasPrefix(basePath, "/") {
		basePath = "/" + basePath
	}
	basePath = strings.TrimRight(basePath, "/")

	// Cria sub-roteador dedicado para as rotas do plugin
	subRouter := chi.NewRouter()
	routePlugin.RegisterRoutes(subRouter)

	loaded := &LoadedPlugin{
		RepoID:   repo.ID,
		Name:     routePlugin.Name(),
		BasePath: basePath,
		Instance: routePlugin,
		Router:   subRouter,
		LoadedAt: time.Now().UTC(),
	}

	m.mu.Lock()
	m.plugins[repo.ID] = loaded
	m.mu.Unlock()

	return loaded, nil
}

// UnloadPlugin remove um plugin do roteador ativo
func (m *Manager) UnloadPlugin(repoID string) {
	m.mu.Lock()
	delete(m.plugins, repoID)
	m.mu.Unlock()
}

// GetLoadedPlugins retorna a lista de todos os plugins ativos
func (m *Manager) GetLoadedPlugins() []*LoadedPlugin {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]*LoadedPlugin, 0, len(m.plugins))
	for _, p := range m.plugins {
		list = append(list, p)
	}
	return list
}

// DynamicRouterMiddleware intercepta requisições HTTP e roteia para o plugin correspondente se o prefixo bater
func (m *Manager) DynamicRouterMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Não intercepta rotas reservadas do sistema
		if strings.HasPrefix(path, "/admin") || path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		m.mu.RLock()
		var matchedPlugin *LoadedPlugin
		for _, p := range m.plugins {
			if path == p.BasePath || strings.HasPrefix(path, p.BasePath+"/") {
				matchedPlugin = p
				break
			}
		}
		m.mu.RUnlock()

		if matchedPlugin != nil {
			// Ajusta o roteamento relativo ao BasePath do plugin
			http.StripPrefix(matchedPlugin.BasePath, matchedPlugin.Router).ServeHTTP(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// SyncAllRepos executa a sincronização de todos os repositórios em background
func (m *Manager) SyncAllRepos(repos []models.Repository) {
	go func() {
		for i := range repos {
			repo := repos[i]
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			_ = m.SyncAndBuild(ctx, &repo)
			cancel()
		}
	}()
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func normalizePrivateKey(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if !strings.Contains(trimmed, "PRIVATE KEY") {
		return trimmed
	}
	// Se já contiver quebras de linha normais, apenas garante terminação limpa
	if strings.Count(trimmed, "\n") >= 2 {
		return strings.ReplaceAll(trimmed, "\r\n", "\n") + "\n"
	}
	// Se foi colada em um campo de texto de linha única e as quebras viraram espaços
	beginIdx := strings.Index(trimmed, "-----BEGIN ")
	endIdx := strings.Index(trimmed, "-----END ")
	if beginIdx != -1 && endIdx != -1 && endIdx > beginIdx {
		headerEndRelative := strings.Index(trimmed[beginIdx+11:], "-----")
		if headerEndRelative != -1 {
			headerEnd := beginIdx + 11 + headerEndRelative + 5
			header := trimmed[beginIdx:headerEnd]
			footer := trimmed[endIdx:]
			body := strings.TrimSpace(trimmed[headerEnd:endIdx])
			bodyWords := strings.Fields(body)
			return header + "\n" + strings.Join(bodyWords, "\n") + "\n" + footer + "\n"
		}
	}
	return trimmed + "\n"
}

func findAppRoot() string {
	if fileExists("/app/go.mod") {
		return "/app"
	}
	if wd, err := os.Getwd(); err == nil {
		dir := wd
		for {
			if fileExists(filepath.Join(dir, "go.mod")) {
				return dir
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	return "/app"
}

func sanitizePkgName(s string) string {
	var sb strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			sb.WriteRune(r)
		} else {
			sb.WriteRune('_')
		}
	}
	res := sb.String()
	if res == "" {
		res = "plugin"
	}
	return res
}

func extractModuleName(data []byte) string {
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			mod := strings.TrimSpace(strings.TrimPrefix(line, "module"))
			mod = strings.Trim(mod, "`\"'")
			if idx := strings.Index(mod, "//"); idx != -1 {
				mod = strings.TrimSpace(mod[:idx])
			}
			return mod
		}
	}
	return ""
}

func findLocalPackages(repoPath string) []string {
	var pkgs []string
	_ = filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(repoPath, path)
		if err != nil || rel == "." || strings.HasPrefix(rel, ".") || strings.HasPrefix(rel, "vendor") {
			return nil
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil
		}
		hasGo := false
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") {
				hasGo = true
				break
			}
		}
		if hasGo {
			pkgs = append(pkgs, filepath.ToSlash(rel))
		}
		return nil
	})
	return pkgs
}

func rewriteImport(importPath string, localPkgs []string, origModule, uniqueModule string) string {
	// Se for o SDK noble-babbage, nunca reescreve
	if importPath == "noble-babbage" || strings.HasPrefix(importPath, "noble-babbage/") ||
		importPath == "github.com/LeoPersan/noble-babbage" || strings.HasPrefix(importPath, "github.com/LeoPersan/noble-babbage/") {
		return importPath
	}

	// Verifica se bate com algum subpacote local do repositório
	for _, lp := range localPkgs {
		if importPath == lp {
			return uniqueModule + "/" + lp
		}
		if origModule != "" && importPath == origModule+"/"+lp {
			return uniqueModule + "/" + lp
		}
		if strings.HasSuffix(importPath, "/"+lp) {
			firstSegment := strings.Split(importPath, "/")[0]
			if strings.HasPrefix(firstSegment, "p_") || !strings.Contains(firstSegment, ".") {
				return uniqueModule + "/" + lp
			}
		}
	}

	// Se for o próprio módulo original sem subpacote
	if origModule != "" && origModule != uniqueModule && importPath == origModule {
		return uniqueModule
	}

	// Se começar com origModule/
	if origModule != "" && origModule != uniqueModule && strings.HasPrefix(importPath, origModule+"/") {
		return uniqueModule + strings.TrimPrefix(importPath, origModule)
	}

	return importPath
}

func preparePluginModule(ctx context.Context, repoPath, repoID string, ts int64, appRoot string) error {
	goModPath := filepath.Join(repoPath, "go.mod")
	if !fileExists(goModPath) {
		return nil
	}

	data, err := os.ReadFile(goModPath)
	if err != nil {
		return fmt.Errorf("falha ao ler go.mod: %w", err)
	}

	origModule := extractModuleName(data)
	uniqueModule := fmt.Sprintf("p_%s_%d", sanitizePkgName(repoID), ts)

	// Atualiza o nome do módulo no go.mod
	editModuleCmd := exec.CommandContext(ctx, "go", "mod", "edit", "-module="+uniqueModule)
	editModuleCmd.Dir = repoPath
	if out, err := editModuleCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("falha ao renomear modulo para %s: %v: %s", uniqueModule, err, string(out))
	}

	// Adiciona os replaces para o SDK do noble-babbage
	editCmd1 := exec.CommandContext(ctx, "go", "mod", "edit", "-replace=noble-babbage="+appRoot)
	editCmd1.Dir = repoPath
	_ = editCmd1.Run()

	editCmd2 := exec.CommandContext(ctx, "go", "mod", "edit", "-replace=github.com/LeoPersan/noble-babbage="+appRoot)
	editCmd2.Dir = repoPath
	_ = editCmd2.Run()

	localPkgs := findLocalPackages(repoPath)

	singleImportRe := regexp.MustCompile(`(?m)^(\s*import\s+(?:[a-zA-Z0-9_.]+\s+)?"([^"]+)")`)
	multiImportRe := regexp.MustCompile(`(?s)import\s*\((.*?)\)`)
	quotedStringRe := regexp.MustCompile(`"([^"]+)"`)

	_ = filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		rel, _ := filepath.Rel(repoPath, path)
		if strings.HasPrefix(rel, ".") || strings.HasPrefix(rel, "vendor") {
			return nil
		}

		contentBytes, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		content := string(contentBytes)

		// Substitui imports em blocos multi-linha: import ( ... )
		newContent := multiImportRe.ReplaceAllStringFunc(content, func(match string) string {
			return quotedStringRe.ReplaceAllStringFunc(match, func(quoted string) string {
				imp := strings.Trim(quoted, `"`)
				rewritten := rewriteImport(imp, localPkgs, origModule, uniqueModule)
				return `"` + rewritten + `"`
			})
		})

		// Substitui imports de linha única: import "..." ou import alias "..."
		newContent = singleImportRe.ReplaceAllStringFunc(newContent, func(match string) string {
			return quotedStringRe.ReplaceAllStringFunc(match, func(quoted string) string {
				imp := strings.Trim(quoted, `"`)
				rewritten := rewriteImport(imp, localPkgs, origModule, uniqueModule)
				return `"` + rewritten + `"`
			})
		})

		if newContent != content {
			_ = os.WriteFile(path, []byte(newContent), info.Mode().Perm())
		}
		return nil
	})

	// Executa go mod tidy para reconciliar dependências e checksums
	tidyCmd := exec.CommandContext(ctx, "go", "mod", "tidy")
	tidyCmd.Dir = repoPath
	tidyCmd.Env = append(os.Environ(), "CGO_ENABLED=1")
	if out, err := tidyCmd.CombinedOutput(); err != nil {
		log.Printf("[PluginManager] go mod tidy emitiu aviso para %s: %v: %s", repoID, err, string(out))
	}

	return nil
}


