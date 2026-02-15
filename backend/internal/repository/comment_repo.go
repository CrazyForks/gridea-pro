package repository

import (
	"context"
	"gridea-pro/backend/internal/domain"
	"path/filepath"
	"sync"
)

type commentRepository struct {
	mu     sync.RWMutex
	appDir string
}

func NewCommentRepository(appDir string) domain.CommentRepository {
	return &commentRepository{appDir: appDir}
}

func (r *commentRepository) GetSettings(ctx context.Context) (*domain.CommentSettings, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	dbPath := filepath.Join(r.appDir, "config", "comment.json")
	var settings domain.CommentSettings

	if err := LoadJSONFile(dbPath, &settings); err != nil {
		if filepath.Base(dbPath) == "comment.json" {
			return &domain.CommentSettings{}, nil
		}
		return nil, err
	}

	return &settings, nil
}

func (r *commentRepository) SaveSettings(ctx context.Context, settings *domain.CommentSettings) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	dbPath := filepath.Join(r.appDir, "config", "comment.json")
	return SaveJSONFile(dbPath, settings)
}
