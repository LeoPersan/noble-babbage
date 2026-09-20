package plugin

import "github.com/go-chi/chi/v5"

// RoutePlugin é a interface canônica que os repositórios Go devem implementar
// para serem carregados dinamicamente como plugins no servidor principal.
type RoutePlugin interface {
	// Name retorna o nome legível do plugin/aplicação
	Name() string

	// BasePath retorna o prefixo de rota HTTP onde as rotas do plugin serão montadas.
	// Exemplo: "/torznab", "/dashboard", "/financeiro"
	BasePath() string

	// RegisterRoutes registra as rotas, endpoints e páginas web no sub-roteador chi dedicado.
	// Todas as rotas registradas em 'r' serão relativas ao BasePath().
	RegisterRoutes(r chi.Router)
}
