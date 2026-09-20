FROM golang:1.24-alpine

# Instala ferramentas essenciais de desenvolvimento e compilador C (necessário para CGO e Go Plugins)
RUN apk add --no-cache \
    ca-certificates \
    tzdata \
    curl \
    git \
    openssh-client \
    bash \
    make \
    gcc \
    g++ \
    musl-dev \
    build-base

# Define diretório de trabalho
WORKDIR /app

# Variáveis de ambiente padrão com CGO habilitado para Go Plugins
ENV PORT=8080 \
    CGO_ENABLED=1 \
    GOCACHE=/tmp/gocache \
    ADMIN_USER=admin \
    ADMIN_PASSWORD=admin \
    DATABASE_PATH=/data/repos.db

# Copia módulos e baixa dependências
COPY go.mod go.sum ./
RUN go mod download

# Copia todo o código fonte
COPY . .

# Prepara diretório de dados e compila o binário do servidor
RUN mkdir -p /data /tmp/gocache && \
    go build -o /app/server ./cmd/server

# Expõe a porta do servidor HTTP
EXPOSE 8080

# Healthcheck nativo
HEALTHCHECK --interval=15s --timeout=5s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/health || exit 1

# Ponto de entrada padrão: executa o binário do servidor
CMD ["/app/server"]
