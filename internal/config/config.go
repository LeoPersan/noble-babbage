package config

import (
	"os"
)

type Config struct {
	Port          string
	AdminUser     string
	AdminPassword string
	DatabasePath  string
	SessionSecret string
}

func LoadConfig() *Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	adminUser := os.Getenv("ADMIN_USER")
	if adminUser == "" {
		adminUser = os.Getenv("ADMIN_USERNAME")
	}
	if adminUser == "" {
		adminUser = "admin"
	}

	adminPassword := os.Getenv("ADMIN_PASSWORD")
	if adminPassword == "" {
		adminPassword = "admin"
	}

	dbPath := os.Getenv("DATABASE_PATH")
	if dbPath == "" {
		dbPath = "data/repos.db"
	}

	sessionSecret := os.Getenv("SESSION_SECRET")
	if sessionSecret == "" {
		sessionSecret = "session-secret-key-noble-babbage"
	}

	return &Config{
		Port:          port,
		AdminUser:     adminUser,
		AdminPassword: adminPassword,
		DatabasePath:  dbPath,
		SessionSecret: sessionSecret,
	}
}
