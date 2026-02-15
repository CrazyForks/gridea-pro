package repository

import (
	"context"
	"fmt"
	"gridea-pro/backend/internal/domain"
	"path/filepath"
	"sync"
)

type categoryRepository struct {
	mu     sync.RWMutex
	appDir string
}

func NewCategoryRepository(appDir string) domain.CategoryRepository {
	return &categoryRepository{appDir: appDir}
}

// loadCategories loads all categories from file
func (r *categoryRepository) loadCategories() ([]domain.Category, error) {
	dbPath := filepath.Join(r.appDir, "config", "categories.json")
	var db struct {
		Categories []domain.Category `json:"categories"`
	}

	if err := LoadJSONFile(dbPath, &db); err != nil {
		if filepath.Base(dbPath) == "categories.json" {
			return []domain.Category{}, nil
		}
		return nil, fmt.Errorf("failed to load categories: %w", err)
	}
	return db.Categories, nil
}

// saveCategories saves all categories to file
func (r *categoryRepository) saveCategories(categories []domain.Category) error {
	dbPath := filepath.Join(r.appDir, "config", "categories.json")
	db := map[string]interface{}{"categories": categories}
	return SaveJSONFile(dbPath, db)
}

func (r *categoryRepository) SaveAll(ctx context.Context, categories []domain.Category) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.saveCategories(categories)
}

func (r *categoryRepository) Create(ctx context.Context, category *domain.Category) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	categories, err := r.loadCategories()
	if err != nil {
		return err
	}

	for _, c := range categories {
		if c.Slug == category.Slug {
			return fmt.Errorf("category slug already exists")
		}
	}

	categories = append(categories, *category)
	return r.saveCategories(categories)
}

func (r *categoryRepository) Update(ctx context.Context, slug string, category *domain.Category) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	categories, err := r.loadCategories()
	if err != nil {
		return err
	}

	found := false
	for i, c := range categories {
		if c.Slug == slug {
			// Update fields
			categories[i] = *category
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("category not found")
	}

	return r.saveCategories(categories)
}

func (r *categoryRepository) Delete(ctx context.Context, slug string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	categories, err := r.loadCategories()
	if err != nil {
		return err
	}

	newCategories := make([]domain.Category, 0, len(categories))
	for _, c := range categories {
		if c.Slug != slug {
			newCategories = append(newCategories, c)
		}
	}

	if len(newCategories) == len(categories) {
		return fmt.Errorf("category not found")
	}

	return r.saveCategories(newCategories)
}

func (r *categoryRepository) GetBySlug(ctx context.Context, slug string) (*domain.Category, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	categories, err := r.loadCategories()
	if err != nil {
		return nil, err
	}

	for _, c := range categories {
		if c.Slug == slug {
			return &c, nil
		}
	}

	return nil, fmt.Errorf("category not found")
}

func (r *categoryRepository) List(ctx context.Context) ([]domain.Category, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.loadCategories()
}
