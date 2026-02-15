package repository

import (
	"context"
	"fmt"
	"gridea-pro/backend/internal/domain"
	"path/filepath"
	"sync"
)

type menuRepository struct {
	mu     sync.RWMutex
	appDir string
}

func NewMenuRepository(appDir string) domain.MenuRepository {
	return &menuRepository{appDir: appDir}
}

func (r *menuRepository) loadMenus() ([]domain.Menu, error) {
	dbPath := filepath.Join(r.appDir, "config", "menus.json")
	var db struct {
		Menus []domain.Menu `json:"menus"`
	}

	if err := LoadJSONFile(dbPath, &db); err != nil {
		if filepath.Base(dbPath) == "menus.json" {
			return []domain.Menu{}, nil
		}
		return nil, fmt.Errorf("failed to load menus: %w", err)
	}
	return db.Menus, nil
}

func (r *menuRepository) saveMenus(menus []domain.Menu) error {
	dbPath := filepath.Join(r.appDir, "config", "menus.json")
	db := map[string]interface{}{"menus": menus}
	return SaveJSONFile(dbPath, db)
}

func (r *menuRepository) SaveAll(ctx context.Context, menus []domain.Menu) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.saveMenus(menus)
}

func (r *menuRepository) Create(ctx context.Context, menu *domain.Menu) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	menus, err := r.loadMenus()
	if err != nil {
		return err
	}

	// Assuming ID is checked or we scan for duplicates if necessary.
	// For now, adhere to simple append.
	menus = append(menus, *menu)
	return r.saveMenus(menus)
}

func (r *menuRepository) Update(ctx context.Context, id string, menu *domain.Menu) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	menus, err := r.loadMenus()
	if err != nil {
		return err
	}

	found := false
	for i, m := range menus {
		// If ID is used for lookup
		if m.ID == id { // We added ID to Menu in domain refactor
			menus[i] = *menu
			found = true
			break
		}
	}

	// Fallback/Legacy Logic: if ID is empty/not used before, maybe strict update isn't possible
	// without ID. But we added ID field. Code using this must generate IDs.
	// If existing data has no IDs, this might be tricky.

	if !found {
		// Try name matching if ID lookup fails? No, clean architecture says we should rely on ID.
		return fmt.Errorf("menu not found")
	}

	return r.saveMenus(menus)
}

func (r *menuRepository) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	menus, err := r.loadMenus()
	if err != nil {
		return err
	}

	newMenus := make([]domain.Menu, 0, len(menus))
	for _, m := range menus {
		if m.ID != id {
			newMenus = append(newMenus, m)
		}
	}

	if len(newMenus) == len(menus) {
		return fmt.Errorf("menu not found")
	}

	return r.saveMenus(newMenus)
}

func (r *menuRepository) GetByID(ctx context.Context, id string) (*domain.Menu, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	menus, err := r.loadMenus()
	if err != nil {
		return nil, err
	}

	for _, m := range menus {
		if m.ID == id {
			return &m, nil
		}
	}
	return nil, fmt.Errorf("menu not found")
}

func (r *menuRepository) List(ctx context.Context) ([]domain.Menu, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.loadMenus()
}
