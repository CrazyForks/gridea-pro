package repository

import (
	"context"
	"gridea-pro/backend/internal/domain"
	"path/filepath"
	"sync"
	"time"
)

type memoRepository struct {
	mu     sync.RWMutex
	appDir string
}

func NewMemoRepository(appDir string) domain.MemoRepository {
	return &memoRepository{appDir: appDir}
}

func (r *memoRepository) GetAll(ctx context.Context) ([]domain.Memo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	dbPath := filepath.Join(r.appDir, "config", "memos.json")
	// Temporary structs for compatible loading
	type MemoDTO struct {
		ID        string      `json:"id"`
		Content   string      `json:"content"`
		Tags      []string    `json:"tags"`
		Images    []string    `json:"images"`
		CreatedAt interface{} `json:"createdAt"` // string or float64 (json number)
		UpdatedAt interface{} `json:"updatedAt"`
	}

	var db struct {
		Memos []MemoDTO `json:"memos"`
	}

	if err := LoadJSONFile(dbPath, &db); err != nil {
		return []domain.Memo{}, nil
	}

	parseTime := func(v interface{}) string {
		switch t := v.(type) {
		case string:
			return t
		case float64:
			return time.UnixMilli(int64(t)).Format(domain.TimeLayout)
		case int64: // Should not happen with standard json decoder but good to have
			return time.UnixMilli(t).Format(domain.TimeLayout)
		default:
			return time.Now().Format(domain.TimeLayout)
		}
	}

	memos := make([]domain.Memo, len(db.Memos))
	for i, m := range db.Memos {
		memos[i] = domain.Memo{
			ID:        m.ID,
			Content:   m.Content,
			Tags:      m.Tags,
			Images:    m.Images,
			CreatedAt: parseTime(m.CreatedAt),
			UpdatedAt: parseTime(m.UpdatedAt),
		}
	}

	return memos, nil
}

func (r *memoRepository) SaveAll(ctx context.Context, memos []domain.Memo) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	dbPath := filepath.Join(r.appDir, "config", "memos.json")
	db := map[string]interface{}{
		"memos": memos,
	}
	return SaveJSONFile(dbPath, db)
}
