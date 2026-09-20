# Go Docker Runner (`noble-babbage`)

Container Docker otimizado e flexível para desenvolvimento, testes e execução de projetos em Go (Golang), compatível com qualquer estrutura de projeto Go (incluindo o `adventurous-pascal` e microsserviços modernos).

## 🚀 Funcionalidades

- **Ambiente Go Completo:** Baseado em `golang:alpine`, incluindo compilador Go, `git`, `bash`, `curl`, `tzdata`, `ca-certificates` e ferramentas de build (`make`, `gcc`).
- **Detecção Automática de Projetos:**
  - Identifica `cmd/server/main.go`, `cmd/server`, `main.go` ou binários pré-compilados.
  - Baixa automaticamente dependências (`go mod download`).
- **Mapeamento de Usuário Não-Root (PUID/PGID):** Evita problemas de permissão nos volumes montados no host.
- **Cache Persistente:** Módulos Go (`/go/pkg/mod`) e cache de compilação (`/tmp/gocache`) armazenados em volumes Docker dedicados para builds ultrarrápidos.
- **Execução Flexível de Comandos:** Permite rodar `go test`, `go run`, `go build`, `bash` ou qualquer comando Go diretamente.

---

## 🛠️ Como Usar

### 1. Construir a Imagem

```bash
docker build -t go-runner .
```

### 2. Executar via Docker Compose

Inicia o serviço montando o diretório atual em `/app`:
```bash
docker compose up --build
```

Para apontar para outro projeto Go no host (como `adventurous-pascal`):
```bash
GO_PROJECT_PATH=../adventurous-pascal docker compose up
```

### 3. Executar Testes Go no Container

```bash
# Executa testes no projeto montado
docker run --rm -v $(pwd):/app go-runner go test -v ./...
```

### 4. Executar Comandos Go ou Acessar o Shell

```bash
# Entrar no terminal interativo
docker run --rm -it -v $(pwd):/app go-runner bash

# Verificar a versão do Go
docker run --rm go-runner go version
```

---

## 🧪 Testes Automatizados

O projeto inclui uma suíte completa de testes automatizados para validar a construção do container, execução de projetos Go, execução de testes unitários e compatibilidade:

```bash
./tests/test_docker_runner.sh
```
