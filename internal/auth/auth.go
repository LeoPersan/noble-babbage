package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"noble-babbage/internal/config"
)

const (
	SessionCookieName = "admin_session"
	CookieMaxAge      = 86400 // 24 horas
)

// GenerateSessionToken cria um token assinado contendo timestamp e HMAC SHA256
func GenerateSessionToken(user, secret string) string {
	timestamp := time.Now().Unix()
	payload := fmt.Sprintf("%s:%d", user, timestamp)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	signature := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("%s:%s", payload, signature)
}

// ValidateSessionToken verifica a assinatura e validade temporal do token
func ValidateSessionToken(token, expectedUser, secret string) bool {
	parts := strings.Split(token, ":")
	if len(parts) != 3 {
		return false
	}
	user := parts[0]
	timestampStr := parts[1]
	signature := parts[2]

	if user != expectedUser {
		return false
	}

	payload := fmt.Sprintf("%s:%s", user, timestampStr)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
		return false
	}

	var timestamp int64
	_, err := fmt.Sscanf(timestampStr, "%d", &timestamp)
	if err != nil {
		return false
	}

	// Validade de 24 horas
	if time.Now().Unix()-timestamp > CookieMaxAge {
		return false
	}

	return true
}

func SetSessionCookie(w http.ResponseWriter, user, secret string) {
	token := GenerateSessionToken(user, secret)
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   CookieMaxAge,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
}

// IsAuthenticated valida se a requisição possui credenciais válidas via Cookie ou HTTP Basic Auth
func IsAuthenticated(r *http.Request, cfg *config.Config) bool {
	// 1. Checa HTTP Basic Auth
	user, pass, ok := r.BasicAuth()
	if ok && user == cfg.AdminUser && pass == cfg.AdminPassword {
		return true
	}

	// 2. Checa Cookie de Sessão
	cookie, err := r.Cookie(SessionCookieName)
	if err == nil && cookie.Value != "" {
		if ValidateSessionToken(cookie.Value, cfg.AdminUser, cfg.SessionSecret) {
			return true
		}
	}

	return false
}

// AuthMiddleware protege as rotas sob /admin
func AuthMiddleware(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Permite rotas públicas de login
			if r.URL.Path == "/admin/login" || r.URL.Path == "/admin/logout" {
				next.ServeHTTP(w, r)
				return
			}

			if !IsAuthenticated(r, cfg) {
				// Se for requisição para a API JSON
				if strings.HasPrefix(r.URL.Path, "/admin/api") || strings.Contains(r.Header.Get("Accept"), "application/json") {
					w.Header().Set("WWW-Authenticate", `Basic realm="Admin Access"`)
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusUnauthorized)
					w.Write([]byte(`{"error":"Acesso não autorizado. Forneça login e senha válidos."}`))
					return
				}

				// Se for navegador, redireciona para a página de login
				http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
