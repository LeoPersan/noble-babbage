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

# Variáveis de ambiente padrão
ENV PORT=8080 \
    CGO_ENABLED=0 \
    GOCACHE=/tmp/gocache \
    ADMIN_USER=admin \
    ADMIN_PASSWORD=admin \
    DATABASE_PATH=/data/repos.db

# Expõe a porta do servidor HTTP
EXPOSE 8080

# Healthcheck nativo
HEALTHCHECK --interval=15s --timeout=5s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# Ponto de entrada padrão: executa o servidor HTTP Go
CMD ["go", "run", "./cmd/server"]
