package handlers

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"noble-babbage/internal/auth"
	"noble-babbage/internal/config"
	"noble-babbage/internal/database"
	"noble-babbage/internal/models"
	"noble-babbage/internal/views"
)

type AdminHandler struct {
	cfg *config.Config
	db  *database.DB
}

func NewAdminHandler(cfg *config.Config, db *database.DB) *AdminHandler {
	return &AdminHandler{
		cfg: cfg,
		db:  db,
	}
}

// -------------------------------------------------------------
// Web UI Handlers
// -------------------------------------------------------------

func (h *AdminHandler) LoginPage(w http.ResponseWriter, r *http.Request) {
	if auth.IsAuthenticated(r, h.cfg) {
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}

	data := views.LoginPageData{
		Error:   r.URL.Query().Get("error"),
		Success: r.URL.Query().Get("success"),
	}
	views.TmplLogin.Execute(w, data)
}

func (h *AdminHandler) LoginSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		views.TmplLogin.Execute(w, views.LoginPageData{Error: "Dados de formulário inválidos"})
		return
	}

	username := strings.TrimSpace(r.FormValue("username"))
	password := strings.TrimSpace(r.FormValue("password"))

	if username != h.cfg.AdminUser || password != h.cfg.AdminPassword {
		w.WriteHeader(http.StatusUnauthorized)
		views.TmplLogin.Execute(w, views.LoginPageData{Error: "Usuário ou senha incorretos."})
		return
	}

	auth.SetSessionCookie(w, h.cfg.AdminUser, h.cfg.SessionSecret)
	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func (h *AdminHandler) Logout(w http.ResponseWriter, r *http.Request) {
	auth.ClearSessionCookie(w)
	http.Redirect(w, r, "/admin/login?success="+url.QueryEscape("Desconectado com sucesso."), http.StatusSeeOther)
}

func (h *AdminHandler) AdminDashboard(w http.ResponseWriter, r *http.Request) {
	repos, err := h.db.GetAll()
	if err != nil {
		http.Error(w, "Erro ao carregar repositórios: "+err.Error(), http.StatusInternalServerError)
		return
	}

	data := views.AdminPageData{
		User:         h.cfg.AdminUser,
		Repositories: repos,
		Success:      r.URL.Query().Get("success"),
		Error:        r.URL.Query().Get("error"),
	}
	views.TmplAdmin.Execute(w, data)
}

func (h *AdminHandler) CreateRepoWeb(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin?error="+url.QueryEscape("Dados inválidos"), http.StatusSeeOther)
		return
	}

	link := strings.TrimSpace(r.FormValue("link"))
	name := strings.TrimSpace(r.FormValue("name"))
	accessKey := strings.TrimSpace(r.FormValue("access_key"))

	if link == "" {
		http.Redirect(w, r, "/admin?error="+url.QueryEscape("O link do repositório é obrigatório"), http.StatusSeeOther)
		return
	}

	repo := &models.Repository{
		Link:      link,
		Name:      name,
		AccessKey: accessKey,
	}

	if err := h.db.Create(repo); err != nil {
		http.Redirect(w, r, "/admin?error="+url.QueryEscape("Erro ao salvar: "+err.Error()), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin?success="+url.QueryEscape("Repositório '"+repo.Name+"' cadastrado com sucesso!"), http.StatusSeeOther)
}

func (h *AdminHandler) EditRepoWeb(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Redirect(w, r, "/admin?error="+url.QueryEscape("ID do repositório inválido"), http.StatusSeeOther)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin?error="+url.QueryEscape("Formulário inválido"), http.StatusSeeOther)
		return
	}

	existing, err := h.db.GetByID(id)
	if err != nil {
		http.Redirect(w, r, "/admin?error="+url.QueryEscape("Repositório não encontrado"), http.StatusSeeOther)
		return
	}

	link := strings.TrimSpace(r.FormValue("link"))
	if link != "" {
		existing.Link = link
	}
	existing.Name = strings.TrimSpace(r.FormValue("name"))
	newKey := strings.TrimSpace(r.FormValue("access_key"))
	if newKey != "" {
		existing.AccessKey = newKey
	}

	if err := h.db.Update(existing); err != nil {
		http.Redirect(w, r, "/admin?error="+url.QueryEscape("Falha na atualização: "+err.Error()), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin?success="+url.QueryEscape("Repositório '"+existing.Name+"' atualizado com sucesso!"), http.StatusSeeOther)
}

func (h *AdminHandler) DeleteRepoWeb(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		http.Redirect(w, r, "/admin?error="+url.QueryEscape("ID inválido"), http.StatusSeeOther)
		return
	}

	if err := h.db.Delete(id); err != nil {
		http.Redirect(w, r, "/admin?error="+url.QueryEscape("Erro ao excluir: "+err.Error()), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin?success="+url.QueryEscape("Repositório excluído com sucesso!"), http.StatusSeeOther)
}

// -------------------------------------------------------------
// REST API Handlers (/admin/api/repos)
// -------------------------------------------------------------

func (h *AdminHandler) APIGetAll(w http.ResponseWriter, r *http.Request) {
	repos, err := h.db.GetAll()
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	respondJSON(w, http.StatusOK, repos)
}

func (h *AdminHandler) APIGetByID(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	repo, err := h.db.GetByID(id)
	if err != nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "Repositório não encontrado"})
		return
	}
	respondJSON(w, http.StatusOK, repo)
}

type CreateRepoRequest struct {
	Link      string `json:"link"`
	Name      string `json:"name,omitempty"`
	AccessKey string `json:"access_key,omitempty"`
}

func (h *AdminHandler) APICreate(w http.ResponseWriter, r *http.Request) {
	var req CreateRepoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "JSON inválido na requisição"})
		return
	}

	if strings.TrimSpace(req.Link) == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "O campo 'link' é obrigatório"})
		return
	}

	repo := &models.Repository{
		Link:      strings.TrimSpace(req.Link),
		Name:      strings.TrimSpace(req.Name),
		AccessKey: strings.TrimSpace(req.AccessKey),
	}

	if err := h.db.Create(repo); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusCreated, repo)
}

type UpdateRepoRequest struct {
	Link      string `json:"link,omitempty"`
	Name      string `json:"name,omitempty"`
	AccessKey string `json:"access_key,omitempty"`
}

func (h *AdminHandler) APIUpdate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	existing, err := h.db.GetByID(id)
	if err != nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "Repositório não encontrado"})
		return
	}

	var req UpdateRepoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "JSON inválido na requisição"})
		return
	}

	if strings.TrimSpace(req.Link) != "" {
		existing.Link = strings.TrimSpace(req.Link)
	}
	if req.Name != "" {
		existing.Name = strings.TrimSpace(req.Name)
	}
	if req.AccessKey != "" {
		existing.AccessKey = strings.TrimSpace(req.AccessKey)
	}

	if err := h.db.Update(existing); err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	respondJSON(w, http.StatusOK, existing)
}

func (h *AdminHandler) APIDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.db.Delete(id); err != nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "Repositório não encontrado"})
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"message": "Repositório removido com sucesso"})
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
