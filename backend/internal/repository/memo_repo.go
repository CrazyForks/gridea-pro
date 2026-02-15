package repository

import (
	"context"
	"fmt"
	"gridea-pro/backend/internal/domain"
	"gridea-pro/backend/internal/utils"
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

// Internal DTO for JSON serialization compatibility
type MemoDTO struct {
	ID        string      `json:"id"`
	Content   string      `json:"content"`
	Tags      []string    `json:"tags"`
	Images    []string    `json:"images"`
	CreatedAt interface{} `json:"createdAt"` // Can be string or int64
	UpdatedAt interface{} `json:"updatedAt"`
}

func (r *memoRepository) loadMemos() ([]domain.Memo, error) {
	dbPath := filepath.Join(r.appDir, "config", "memos.json")
	var db struct {
		Memos []MemoDTO `json:"memos"`
	}

	if err := LoadJSONFile(dbPath, &db); err != nil {
		return []domain.Memo{}, nil
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

func (r *memoRepository) saveMemos(memos []domain.Memo) error {
	dbPath := filepath.Join(r.appDir, "config", "memos.json")

	dtos := make([]MemoDTO, len(memos))
	for i, m := range memos {
		dtos[i] = MemoDTO{
			ID:        m.ID,
			Content:   m.Content,
			Tags:      m.Tags,
			Images:    m.Images,
			CreatedAt: m.CreatedAt.Format(time.RFC3339),
			UpdatedAt: m.UpdatedAt.Format(time.RFC3339),
		}
	}

	db := map[string]interface{}{
		"memos": dtos,
	}
	return SaveJSONFile(dbPath, db)
}

func parseTime(v interface{}) time.Time {
	switch t := v.(type) {
	case string:
		if t == "" {
			return time.Now()
		}
		if parsed, err := utils.ParseTime(t); err == nil {
			return parsed
		}
		// If strict parsing fails (e.g. some really weird format), fallback to Now
		// But utils.ParseTime handles RFC3339 and TimeLayout.
		// One edge case: TimeLayout (2006-01-02 15:04:05) is parsed as UTC by utils.ParseTime.
		// If we want to support legacy local time data, we might need a specific check here?
		// For now, let's trust utils.ParseTime.
		// If legacy data was saved as "2024-02-14 10:00:00" (Local), utils.ParseTime parses as 10:00:00 UTC.
		// If Local is UTC+8, actual time was 02:00:00 UTC.
		// So it shifts 8 hours into the future (10:00:00 UTC vs 02:00:00 UTC).
		// This explains why "Just now" shows for old data? No, "future" data shows as "Just now" in the frontend logic?
		// "diff < 60*1000" logic in frontend handles future?
		// diff = now - date. If date > now, diff is negative. diff < 60000 is true.
		// So future dates show as "Just now".
		// To fix legacy data, we explicitly try ParseInLocation for TimeLayout.
		if t2, err := time.ParseInLocation(domain.TimeLayout, t, time.Local); err == nil {
			return t2
		}
		return time.Now()
	case float64:
		return time.UnixMilli(int64(t))
	case int64:
		return time.UnixMilli(t)
	default:
		return time.Now()
	}
}

func (r *memoRepository) SaveAll(ctx context.Context, memos []domain.Memo) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.saveMemos(memos)
}

func (r *memoRepository) Create(ctx context.Context, memo *domain.Memo) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	memos, err := r.loadMemos()
	if err != nil {
		return err
	}

	memos = append(memos, *memo)
	return r.saveMemos(memos)
}

func (r *memoRepository) Update(ctx context.Context, id string, memo *domain.Memo) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	memos, err := r.loadMemos()
	if err != nil {
		return err
	}

	found := false
	for i, m := range memos {
		if m.ID == id {
			memos[i] = *memo
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("memo not found")
	}

	return r.saveMemos(memos)
}

func (r *memoRepository) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	memos, err := r.loadMemos()
	if err != nil {
		return err
	}

	newMemos := make([]domain.Memo, 0, len(memos))
	for _, m := range memos {
		if m.ID != id {
			newMemos = append(newMemos, m)
		}
	}

	if len(newMemos) == len(memos) {
		return fmt.Errorf("memo not found")
	}

	return r.saveMemos(newMemos)
}

func (r *memoRepository) GetByID(ctx context.Context, id string) (*domain.Memo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	memos, err := r.loadMemos()
	if err != nil {
		return nil, err
	}

	for _, m := range memos {
		if m.ID == id {
			return &m, nil
		}
	}
	return nil, fmt.Errorf("memo not found")
}

func (r *memoRepository) List(ctx context.Context) ([]domain.Memo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.loadMemos()
}
