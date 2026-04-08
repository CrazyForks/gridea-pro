package repository

import (
	"context"
	"gridea-pro/backend/internal/domain"
	"path/filepath"
	"sync"
)

type aiSettingRepository struct {
	mu     sync.RWMutex
	appDir string
	cache  *domain.AISetting
	loaded bool
}

func NewAISettingRepository(appDir string) domain.AISettingRepository {
	return &aiSettingRepository{
		appDir: appDir,
	}
}

func (r *aiSettingRepository) loadIfNeeded() error {
	r.mu.RLock()
	if r.loaded {
		r.mu.RUnlock()
		return nil
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.loaded {
		return nil
	}

	settingPath := filepath.Join(r.appDir, "config", "ai_setting.json")
	var setting domain.AISetting
	if err := LoadJSONFile(settingPath, &setting); err != nil {
		r.cache = &domain.AISetting{}
		r.loaded = true
		return nil
	}

	r.cache = &setting
	r.loaded = true
	return nil
}

func (r *aiSettingRepository) GetAISetting(ctx context.Context) (domain.AISetting, error) {
	if err := r.loadIfNeeded(); err != nil {
		return domain.AISetting{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.cache == nil {
		return domain.AISetting{}, nil
	}
	return *r.cache, nil
}

func (r *aiSettingRepository) SaveAISetting(ctx context.Context, setting domain.AISetting) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	settingPath := filepath.Join(r.appDir, "config", "ai_setting.json")
	if err := SaveJSONFile(settingPath, setting); err != nil {
		return err
	}

	r.cache = &setting
	r.loaded = true
	return nil
}
