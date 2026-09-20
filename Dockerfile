FROM golang:alpine

# Instala ferramentas essenciais de desenvolvimento
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    curl \
    git \
    bash \
    make \
    build-base

# Define diretório de trabalho
WORKDIR /app

# Variáveis de ambiente
ENV CGO_ENABLED=0 \
    GOCACHE=/tmp/gocache

# Ponto de entrada que mantém o container rodando aguardando comandos/execução
CMD ["tail", "-f", "/dev/null"]
