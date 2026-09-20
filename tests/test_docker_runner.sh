#!/bin/bash
set -euo pipefail

IMAGE_NAME="noble-babbage-go:test"
CONTAINER_NAME="noble-babbage-live-test"
WORKSPACE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

echo "=========================================="
echo " [TEST SUITE] Docker & Go Plugin Verification"
echo "=========================================="

cleanup() {
    echo "[Cleanup] Parando container de teste..."
    docker rm -f "${CONTAINER_NAME}" 2>/dev/null || true
}
trap cleanup EXIT

# 1. Build da imagem Docker
echo "==> 1. Build da imagem Docker com CGO e suporte a plugins..."
docker build -t "${IMAGE_NAME}" "${WORKSPACE_DIR}"

# 2. Executa testes unitários e de integração de plugins dentro do container Docker
echo "==> 2. Executando testes unitários e de Go Plugin dentro do container Docker..."
docker run --rm -v "${WORKSPACE_DIR}:/app" "${IMAGE_NAME}" go test -v ./tests/...

# 3. Inicia o servidor HTTP no container em background
echo "==> 3. Iniciando servidor HTTP dentro do container Docker..."
docker run -d --name "${CONTAINER_NAME}" -p 18080:8080 -v "${WORKSPACE_DIR}:/app" "${IMAGE_NAME}" go run ./cmd/server

# 4. Polling do endpoint de Healthcheck
echo "==> 4. Testando endpoint de healthcheck (/health)..."
HEALTH_RESP=""
for i in {1..20}; do
    HEALTH_RESP=$(curl -s http://localhost:18080/health || true)
    if [[ "${HEALTH_RESP}" == *'"status":"ok"'* ]]; then
        break
    fi
    sleep 1
done

echo "    Resposta /health: ${HEALTH_RESP}"
if [[ "${HEALTH_RESP}" != *'"status":"ok"'* ]]; then
    echo "ERRO: Falha ao verificar endpoint de healthcheck. Logs do container:"
    docker logs "${CONTAINER_NAME}"
    exit 1
fi

# 5. Testa redirecionamento da rota raiz / para /admin
echo "==> 5. Testando redirecionamento de / para /admin..."
REDIRECT_STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:18080/)
echo "    Status HTTP: ${REDIRECT_STATUS}"
if [ "${REDIRECT_STATUS}" != "303" ]; then
    echo "ERRO: Esperado redirecionamento 303 da rota raiz."
    exit 1
fi

# 6. Testa tela de login pública (/admin/login)
echo "==> 6. Testando acesso à tela de login (/admin/login)..."
LOGIN_PAGE=$(curl -s http://localhost:18080/admin/login)
if [[ "${LOGIN_PAGE}" != *"Painel Administrativo"* ]]; then
    echo "ERRO: Página de login não renderizou texto esperado."
    exit 1
fi
echo "    Página de login renderizada com sucesso!"

# 7. Testa API de CRUD autenticada com HTTP Basic Auth
echo "==> 7. Testando criação de repositório via API (/admin/api/repos)..."
REPO_RESP=$(curl -s -u admin:admin -X POST http://localhost:18080/admin/api/repos \
    -H "Content-Type: application/json" \
    -d '{"link":"https://github.com/torvalds/linux.git","access_key":"token-xyz-123456"}')
echo "    Resposta API Criação: ${REPO_RESP}"
if [[ "${REPO_RESP}" != *'"name":"linux"'* ]]; then
    echo "ERRO: Falha ao cadastrar repositório ou auto-extrair nome."
    exit 1
fi

echo "==> 8. Testando listagem de repositórios via API (/admin/api/repos)..."
LIST_RESP=$(curl -s -u admin:admin http://localhost:18080/admin/api/repos)
echo "    Resposta API Listagem: ${LIST_RESP}"
if [[ "${LIST_RESP}" != *'"name":"linux"'* ]]; then
    echo "ERRO: Repositório não encontrado na listagem da API."
    exit 1
fi

echo "=========================================="
echo " TODOS OS TESTES PASSARAM COM SUCESSO!"
echo "=========================================="
