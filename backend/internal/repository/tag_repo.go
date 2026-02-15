package repository

import (
	"context"
	"fmt"
	"gridea-pro/backend/internal/domain"
	"path/filepath"
	"sync"
)

type tagRepository struct {
	mu     sync.RWMutex
	appDir string
}

func NewTagRepository(appDir string) domain.TagRepository {
	return &tagRepository{appDir: appDir}
}

func (r *tagRepository) List(ctx context.Context) ([]domain.Tag, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.loadTags()
}

func (r *tagRepository) Create(ctx context.Context, tag *domain.Tag) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	tags, err := r.loadTags()
	if err != nil {
		return err
	}

	// Check duplicates? ID is optional in Create request maybe?
	// Assuming ID is pre-generated or we check Name.
	tags = append(tags, *tag)
	return r.saveTags(tags)
}

func (r *tagRepository) Update(ctx context.Context, tag *domain.Tag) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	tags, err := r.loadTags()
	if err != nil {
		return err
	}

	found := false
	for i, t := range tags {
		if t.ID == tag.ID {
			tags[i] = *tag
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("tag not found: %s", tag.ID)
	}

	return r.saveTags(tags)
}

func (r *tagRepository) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	tags, err := r.loadTags()
	if err != nil {
		return err
	}

	var newTags []domain.Tag
	for _, t := range tags {
		if t.ID != id {
			newTags = append(newTags, t)
		}
	}

	return r.saveTags(newTags)
}

// Helpers

func (r *tagRepository) loadTags() ([]domain.Tag, error) {
	dbPath := filepath.Join(r.appDir, "config", "tags.json")
	var db struct {
		Tags []domain.Tag `json:"tags"`
	}
	if err := LoadJSONFile(dbPath, &db); err != nil {
		return []domain.Tag{}, nil
	}
	return db.Tags, nil
}

func (r *tagRepository) saveTags(tags []domain.Tag) error {
	dbPath := filepath.Join(r.appDir, "config", "tags.json")
	db := map[string]interface{}{"tags": tags}
	return SaveJSONFile(dbPath, db)
}
