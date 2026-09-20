package models

import (
	"net/url"
	"strings"
	"time"
)

const (
	StatusPending  = "pending"
	StatusBuilding = "building"
	StatusActive   = "active"
	StatusError    = "error"
)

type Repository struct {
	ID             string     `json:"id"`
	Link           string     `json:"link"`
	Name           string     `json:"name"`
	AccessKey      string     `json:"access_key,omitempty"`
	Status         string     `json:"status"`
	BasePath       string     `json:"base_path,omitempty"`
	LastBuildAt    *time.Time `json:"last_build_at,omitempty"`
	LastBuildError string     `json:"last_build_error,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// MaskedAccessKey retorna a chave de acesso mascarada para visualização segura na UI
func (r Repository) MaskedAccessKey() string {
	if r.AccessKey == "" {
		return "-"
	}
	if len(r.AccessKey) <= 6 {
		return "******"
	}
	return r.AccessKey[:3] + "..." + r.AccessKey[len(r.AccessKey)-3:]
}

// EffectiveBasePath calcula a rota final onde o plugin será exposto
func (r Repository) EffectiveBasePath() string {
	path := strings.TrimSpace(r.BasePath)
	if path == "" {
		path = "/" + r.Name
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return strings.TrimRight(path, "/")
}

// ExtractRepoName extrai automaticamente o nome do repositório a partir de URLs HTTP(S), SSH ou caminhos Git.
func ExtractRepoName(link string) string {
	trimmed := strings.TrimSpace(link)
	if trimmed == "" {
		return "unnamed-repo"
	}

	// Remove barras finais
	trimmed = strings.TrimRight(trimmed, "/")

	// Remove sufixo .git se houver
	trimmed = strings.TrimSuffix(trimmed, ".git")

	// Caso seja formato SSH (ex: git@github.com:usuario/repo)
	if strings.Contains(trimmed, ":") && !strings.Contains(trimmed, "://") {
		parts := strings.Split(trimmed, ":")
		if len(parts) > 1 {
			trimmed = parts[len(parts)-1]
		}
	} else if u, err := url.Parse(trimmed); err == nil && u.Path != "" {
		trimmed = u.Path
	}

	// Pega o último segmento do caminho
	segments := strings.Split(strings.ReplaceAll(trimmed, "\\", "/"), "/")
	var lastSegment string
	for i := len(segments) - 1; i >= 0; i-- {
		seg := strings.TrimSpace(segments[i])
		if seg != "" {
			lastSegment = seg
			break
		}
	}

	if lastSegment == "" {
		return "unnamed-repo"
	}

	return lastSegment
}
