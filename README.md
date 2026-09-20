# Servidor HTTP & Gerenciador de Go Plugins (`noble-babbage`)

[![Build and Publish Container](https://github.com/LeoPersan/noble-babbage/actions/workflows/docker-build.yml/badge.svg)](https://github.com/LeoPersan/noble-babbage/actions/workflows/docker-build.yml)

Aplicação web e plataforma centralizada para execução dinâmica de repositórios Go como plugins HTTP via `plugin.Open`, com painel administrativo `/admin`, autenticação e persistência em SQLite puro.

---

## 🖥️ Como Importar e Rodar no ZimaOS (ou CasaOS)

O **ZimaOS** permite a importação direta de aplicações através do formato Docker Compose (com suporte às extensões de metadados da App Store).

### Método 1: Importação Direta via Docker Compose (Recomendado)

1. Acesse o painel web do seu **ZimaOS**.
2. Abra a **App Store**.
3. Clique em **Custom Install** (botão `+` ou "Instalar app personalizado" no canto superior direito).
4. No canto superior direito da janela de configuração, clique no ícone **Import** (importar).
5. Cole o conteúdo YAML abaixo e clique em **Submit**:

```yaml
name: noble-babbage
services:
  app:
    image: ghcr.io/leopersan/noble-babbage:latest
    container_name: noble-babbage
    restart: unless-stopped
    ports:
      - "8081:8080"
    environment:
      - PORT=8080
      - ADMIN_USER=admin
      - ADMIN_PASSWORD=admin
      - DATABASE_PATH=/data/repos.db
      - CGO_ENABLED=1
      - GOCACHE=/tmp/gocache
    volumes:
      - /DATA/AppData/noble-babbage/data:/data
    x-casaos:
      ports:
        - container: "8080"
          description:
            en_us: Web UI & Admin Panel Port
      volumes:
        - container: /data
          description:
            en_us: Persistent SQLite database and dynamic plugins storage
x-casaos:
  architectures:
    - amd64
    - arm64
  main: app
  description:
    en_us: Dynamic Go HTTP Plugin Server and Manager with Admin UI.
  tagline:
    en_us: Dynamic Go Plugins HTTP Platform
  developer: LeoPersan
  author: LeoPersan
  icon: https://raw.githubusercontent.com/LeoPersan/noble-babbage/main/cmd/server/favicon.ico
  thumbnail: ""
  title:
    en_us: Noble Babbage
  category: Utilities
  port_map: "8081"
  index: /admin
```

6. O ZimaOS preencherá automaticamente os campos, ícone e portas. Clique em **Install** para iniciar o container!
7. Acesse o painel administrativo clicando no ícone do app no ZimaOS ou diretamente via `http://<IP-DO-SEU-ZIMAOS>:8081/admin`.

---

### Método 2: Configuração Manual dos Campos no ZimaOS

Se preferir preencher manualmente a tela do **Custom Install** do ZimaOS:

| Campo na Interface | Valor |
| :--- | :--- |
| **App Name** | `Noble Babbage` |
| **Image URL** | `ghcr.io/leopersan/noble-babbage:latest` |
| **Web UI Port (Host)** | `8081` |
| **Container Port** | `8080` |
| **Web UI Path** | `/admin` |
| **Volume (Host)** | `/DATA/AppData/noble-babbage/data` |
| **Volume (Container)** | `/data` |
| **Environment Variables** | `PORT=8080`<br>`ADMIN_USER=admin`<br>`ADMIN_PASSWORD=admin`<br>`DATABASE_PATH=/data/repos.db` |

---

## 🌟 Como Funciona o Padrão de Plugins

Cada repositório Go cadastrado no painel `/admin` é baixado via `git clone/pull`, compilado dinamicamente com `-buildmode=plugin` em um arquivo `.so`, e suas rotas e telas web são montadas dinamicamente no servidor principal.

### Contrato do Plugin (`pkg/plugin`)

Os repositórios devem implementar a interface `pkg/plugin.RoutePlugin`:

```go
package main

import (
	"net/http"
	"github.com/go-chi/chi/v5"
	pluginSDK "noble-babbage/pkg/plugin"
)

type MeuAppPlugin struct{}

func (p *MeuAppPlugin) Name() string {
	return "Meu Aplicativo Web"
}

// BasePath define onde as rotas serão montadas (ex: http://localhost:8081/meu-app)
func (p *MeuAppPlugin) BasePath() string {
	return "/meu-app"
}

func (p *MeuAppPlugin) RegisterRoutes(r chi.Router) {
	// Rota relativa raiz: /meu-app
	r.Get("/", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte("<h1>Página Web do Plugin Carregado!</h1>"))
	})

	// Sub-rota: /meu-app/api/dados
	r.Get("/api/dados", func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"online","dados":[1,2,3]}`))
	})
}

// Exporta o símbolo 'Plugin' obrigatório
var Plugin pluginSDK.RoutePlugin = &MeuAppPlugin{}
```

---

## 🚀 Painel Administrativo (`/admin`)

- **Interface Web:** Cadastro de repositórios (HTTPS, SSH ou caminho local).
- **Status do Plugin:**
  - `Ativo`: Plugin compilado e rotas respondendo normalmente (com link direto clicável).
  - `Compilando...`: Download do código e compilação em andamento.
  - `Erro de Build`: Exibe o log detalhado do erro caso a compilação falhe.
  - `Pendente`: Aguardando primeira sincronização.
- **Botão "Sync & Build":** Dispara a atualização do repositório (`git pull`), recompilação e recarregamento a quente do plugin.

---

## ⚙️ Variáveis de Ambiente

| Variável | Padrão | Descrição |
| :--- | :--- | :--- |
| `PORT` | `8080` | Porta HTTP interna do container |
| `HOST_PORT` | `8081` | Porta exposta no host via Docker Compose |
| `ADMIN_USER` / `ADMIN_USERNAME` | `admin` | Usuário de acesso ao `/admin` |
| `ADMIN_PASSWORD` | `admin` | Senha de acesso ao `/admin` |
| `DATABASE_PATH` | `/data/repos.db` | Arquivo do banco de dados SQLite |

---

## 🧪 Testes Automatizados

Executar testes locais de unit/plugin:
```bash
go test -v ./tests/...
```

Executar testes completos no container Docker:
```bash
./tests/test_docker_runner.sh
```
