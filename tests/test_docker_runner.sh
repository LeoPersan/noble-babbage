#!/bin/bash
set -euo pipefail

IMAGE_NAME="noble-babbage-go:test"
CONTAINER_NAME="test-go-container-live"
WORKSPACE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

echo "=========================================="
echo " [TEST SUITE] Go Container with Tail Entrypoint"
echo "=========================================="

cleanup() {
    echo "[Cleanup] Parando container de teste..."
    docker rm -f "${CONTAINER_NAME}" 2>/dev/null || true
}
trap cleanup EXIT

# 1. Build da imagem Docker
echo "==> 1. Build da imagem Docker..."
docker build -t "${IMAGE_NAME}" "${WORKSPACE_DIR}"

# 2. Inicia o container com o comando padrão (tail -f /dev/null)
echo "==> 2. Iniciando container em background..."
docker run -d --name "${CONTAINER_NAME}" "${IMAGE_NAME}"
sleep 2

# 3. Verifica se o container está em execução ativa
STATUS=$(docker inspect --format '{{.State.Running}}' "${CONTAINER_NAME}")
echo "    Status do container (Running): ${STATUS}"
if [ "${STATUS}" != "true" ]; then
    echo "ERRO: O container não está rodando."
    exit 1
fi

# 4. Testa a execução do compilador Go dentro do container
echo "==> 3. Verificando 'go version' dentro do container..."
GO_VER=$(docker exec "${CONTAINER_NAME}" go version)
echo "    Versão Go: ${GO_VER}"

# 5. Testa execução de código Go dentro do container
echo "==> 4. Executando código Go com 'go run' dentro do container..."
OUTPUT=$(docker exec "${CONTAINER_NAME}" go run -e 'package main; import "fmt"; func main() { fmt.Println("Go Container Ready") }' 2>/dev/null || \
         docker exec "${CONTAINER_NAME}" sh -c 'echo "package main; import \"fmt\"; func main() { fmt.Println(\"Go Container Ready\") }" > /tmp/main.go && go run /tmp/main.go')
echo "    Saída: ${OUTPUT}"

if [[ "${OUTPUT}" != *"Go Container Ready"* ]]; then
    echo "ERRO: Falha ao executar código Go dentro do container."
    exit 1
fi

echo "=========================================="
echo " TODOS OS TESTES PASSARAM COM SUCESSO!"
echo "=========================================="
