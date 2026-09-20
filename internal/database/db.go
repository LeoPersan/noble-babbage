package database

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
	"noble-babbage/internal/models"
)

var (
	ErrNotFound = errors.New("repositório não encontrado")
)

type DB struct {
	db *sql.DB
}

func NewDB(dbPath string) (*DB, error) {
	dir := filepath.Dir(dbPath)
	if dir != "." && dir != "/" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("falha ao criar diretório do banco de dados: %w", err)
		}
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("falha ao abrir banco sqlite: %w", err)
	}

	// Limita conexões para SQLite para evitar locks simultâneos de escrita
	db.SetMaxOpenConns(1)

	instance := &DB{db: db}
	if err := instance.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("falha ao executar migrações: %w", err)
	}

	return instance, nil
}

func (d *DB) migrate() error {
	query := `
	CREATE TABLE IF NOT EXISTS repositories (
		id TEXT PRIMARY KEY,
		link TEXT NOT NULL,
		name TEXT NOT NULL,
		access_key TEXT,
		created_at DATETIME NOT NULL,
		updated_at DATETIME NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_repositories_name ON repositories(name);
	`
	_, err := d.db.Exec(query)
	return err
}

func (d *DB) Create(repo *models.Repository) error {
	if repo.ID == "" {
		repo.ID = uuid.New().String()
	}
	if repo.Name == "" {
		repo.Name = models.ExtractRepoName(repo.Link)
	}
	now := time.Now().UTC()
	repo.CreatedAt = now
	repo.UpdatedAt = now

	query := `
	INSERT INTO repositories (id, link, name, access_key, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?)
	`
	_, err := d.db.Exec(query, repo.ID, repo.Link, repo.Name, repo.AccessKey, repo.CreatedAt, repo.UpdatedAt)
	if err != nil {
		return fmt.Errorf("falha ao inserir repositório: %w", err)
	}
	return nil
}

func (d *DB) GetAll() ([]models.Repository, error) {
	query := `
	SELECT id, link, name, COALESCE(access_key, ''), created_at, updated_at
	FROM repositories
	ORDER BY created_at DESC
	`
	rows, err := d.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("falha ao listar repositórios: %w", err)
	}
	defer rows.Close()

	var repos []models.Repository
	for rows.Next() {
		var r models.Repository
		if err := rows.Scan(&r.ID, &r.Link, &r.Name, &r.AccessKey, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("falha ao ler linha de repositório: %w", err)
		}
		repos = append(repos, r)
	}

	if repos == nil {
		repos = []models.Repository{}
	}

	return repos, nil
}

func (d *DB) GetByID(id string) (*models.Repository, error) {
	query := `
	SELECT id, link, name, COALESCE(access_key, ''), created_at, updated_at
	FROM repositories
	WHERE id = ?
	`
	var r models.Repository
	err := d.db.QueryRow(query, id).Scan(&r.ID, &r.Link, &r.Name, &r.AccessKey, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("falha ao buscar repositório por id: %w", err)
	}
	return &r, nil
}

func (d *DB) Update(repo *models.Repository) error {
	if repo.Name == "" {
		repo.Name = models.ExtractRepoName(repo.Link)
	}
	repo.UpdatedAt = time.Now().UTC()

	query := `
	UPDATE repositories
	SET link = ?, name = ?, access_key = ?, updated_at = ?
	WHERE id = ?
	`
	res, err := d.db.Exec(query, repo.Link, repo.Name, repo.AccessKey, repo.UpdatedAt, repo.ID)
	if err != nil {
		return fmt.Errorf("falha ao atualizar repositório: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (d *DB) Delete(id string) error {
	query := `DELETE FROM repositories WHERE id = ?`
	res, err := d.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("falha ao excluir repositório: %w", err)
	}
	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (d *DB) Close() error {
	return d.db.Close()
}
