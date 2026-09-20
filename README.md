# Servidor HTTP & Gerenciador de Repositórios (`noble-babbage`)

Aplicação web e API REST desenvolvida em Go para gerenciamento centralizado de repositórios Git, com painel administrativo `/admin`, autenticação via variáveis de ambiente e persistência em SQLite puro (`modernc.org/sqlite`).

---

## ✨ Funcionalidades

- **Painel Administrativo (`/admin`):**
  - Interface Web HTML responsiva (estilizada com Tailwind CSS) para visualização e cadastro de repositórios.
  - Tela de login amigável em `/admin/login` e encerramento de sessão em `/admin/logout`.
  - Suporte a autenticação por **Cookie de Sessão** (navegador) e **HTTP Basic Auth** (APIs/CLI).
- **CRUD de Repositórios:**
  - **Link:** URL do repositório Git (HTTPS, SSH ou caminho Git).
  - **Nome:** Opcional. Se omitido, é extraído automaticamente do link (ex: `https://github.com/torvalds/linux.git` -> `linux`).
  - **Chave de Acesso:** Opcional. Suporta tokens de deploy / personal access tokens, com exibição mascarada na UI (`sec...123`).
  - **Operações:** Listagem, Criação, Edição e Exclusão com feedback visual.
- **API REST Integrada (`/admin/api/repos`):**
  - Endpoints JSON para automação e integração externa protegidos por autenticação.
- **Persistência SQLite:**
  - Driver Go puro sem CGO (`modernc.org/sqlite`), compatível com qualquer arquitetura e `CGO_ENABLED=0`.
- **Ambiente Docker Completo:**
  - `Dockerfile` e `docker-compose.yml` prontos para execução e desenvolvimento.

---

## ⚙️ Variáveis de Ambiente

| Variável | Padrão | Descrição |
| :--- | :--- | :--- |
| `PORT` | `8080` | Porta HTTP do servidor |
| `ADMIN_USER` / `ADMIN_USERNAME` | `admin` | Nome de usuário para login administrativo |
| `ADMIN_PASSWORD` | `admin` | Senha de acesso para o painel administrativo |
| `DATABASE_PATH` | `data/repos.db` | Caminho do arquivo de banco de dados SQLite |
| `SESSION_SECRET` | `session-secret-key-noble-babbage` | Chave secreta para assinatura dos cookies de sessão |

---

## 🚀 Como Executar

### Localmente (Go)

```bash
# Executa o servidor HTTP
go run ./cmd/server
```

Acesse no navegador: [http://localhost:8080/admin](http://localhost:8080/admin)

### Via Docker Compose

```bash
# Sobe o container
docker compose up -d

# Executa comandos no container
docker exec -it go-container go run ./cmd/server
```

---

## 📡 Endpoints da API REST

Todas as chamadas da API REST exigem autenticação via **HTTP Basic Auth** (`ADMIN_USER:ADMIN_PASSWORD`):

### 1. Listar Repositórios
```bash
curl -u admin:admin http://localhost:8080/admin/api/repos
```

### 2. Cadastrar Repositório
```bash
# Com nome automático extraído do link
curl -u admin:admin -X POST http://localhost:8080/admin/api/repos \
  -H "Content-Type: application/json" \
  -d '{"link":"https://github.com/torvalds/linux.git","access_key":"token-xyz"}'

# Com nome personalizado
curl -u admin:admin -X POST http://localhost:8080/admin/api/repos \
  -H "Content-Type: application/json" \
  -d '{"link":"https://github.com/usuario/meu-repo.git","name":"Meu Projeto","access_key":"token-123"}'
```

### 3. Obter Repositório por ID
```bash
curl -u admin:admin http://localhost:8080/admin/api/repos/{id}
```

### 4. Atualizar Repositório
```bash
curl -u admin:admin -X PUT http://localhost:8080/admin/api/repos/{id} \
  -H "Content-Type: application/json" \
  -d '{"name":"Novo Nome","access_key":"nova-chave"}'
```

### 5. Excluir Repositório
```bash
curl -u admin:admin -X DELETE http://localhost:8080/admin/api/repos/{id}
```

---

## 🧪 Testes Automatizados

Para executar os testes unitários e de integração locais:
```bash
go test -v ./...
```

Para executar os testes automatizados de ponta a ponta no container Docker:
```bash
./tests/test_docker_runner.sh
```
