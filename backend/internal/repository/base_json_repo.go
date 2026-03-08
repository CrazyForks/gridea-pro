package repository

import (
	"context"
	"fmt"
	"gridea-pro/backend/internal/domain"
	"path/filepath"
	"sync"
)

// BaseJSONRepository 泛型 JSON 仓库基类
// T 必须实现 domain.Identifiable 接口
type BaseJSONRepository[T domain.Identifiable] struct {
	mu       sync.RWMutex
	appDir   string
	fileName string
	rootKey  string
	data     []T
	loaded   bool
}

// NewBaseJSONRepository 创建新的泛型仓库
func NewBaseJSONRepository[T domain.Identifiable](appDir, fileName, rootKey string) *BaseJSONRepository[T] {
	return &BaseJSONRepository[T]{
		appDir:   appDir,
		fileName: fileName,
		rootKey:  rootKey,
		data:     make([]T, 0),
		loaded:   false,
	}
}

// initIfNeeded 确保数据已加载 (Double-checked locking impl via forceLoad)
func (r *BaseJSONRepository[T]) initIfNeeded() error {
	r.mu.RLock()
	if r.loaded {
		r.mu.RUnlock()
		return nil
	}
	r.mu.RUnlock()

	return r.forceLoad()
}

// forceLoad 强制加载数据
func (r *BaseJSONRepository[T]) forceLoad() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.loaded {
		return nil
	}

	dbPath := filepath.Join(r.appDir, "config", r.fileName)

	// 使用 map[string][]T 来适配 {"rootKey": [...]} 结构
	dataMap := make(map[string][]T)

	if err := LoadJSONFile(dbPath, &dataMap); err != nil {
		// 如果加载失败，假设文件不存在或为空，初始化为空列表
		// 也可以根据 err 类型做更细致的处理
		r.data = []T{}
		r.loaded = true
		return nil
	}

	if val, ok := dataMap[r.rootKey]; ok {
		r.data = val
	} else {
		r.data = []T{}
	}

	r.loaded = true
	return nil
}

// save 保存数据到磁盘 (Callers must hold Lock)
func (r *BaseJSONRepository[T]) save() error {
	dbPath := filepath.Join(r.appDir, "config", r.fileName)
	payload := map[string][]T{r.rootKey: r.data}
	return SaveJSONFile(dbPath, payload)
}

// List 获取所有条目
func (r *BaseJSONRepository[T]) List(ctx context.Context) ([]T, error) {
	if err := r.initIfNeeded(); err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// 返回副本以防止外部修改影响内部状态
	result := make([]T, len(r.data))
	copy(result, r.data)
	return result, nil
}

// Get 获取单个条目
func (r *BaseJSONRepository[T]) Get(ctx context.Context, id string) (T, error) {
	var zero T
	if err := r.initIfNeeded(); err != nil {
		return zero, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, item := range r.data {
		if item.GetID() == id {
			return item, nil
		}
	}

	return zero, fmt.Errorf("item not found with id: %s", id)
}

// Add 添加条目
func (r *BaseJSONRepository[T]) Add(ctx context.Context, item T) error {
	if err := r.initIfNeeded(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	id := item.GetID()
	for _, exists := range r.data {
		if exists.GetID() == id {
			return fmt.Errorf("item already exists with id: %s", id)
		}
	}

	// Create a new slice with the added item
	newData := make([]T, len(r.data)+1)
	copy(newData, r.data)
	newData[len(r.data)] = item

	// Save to file first
	originalData := r.data // Store original data in case save fails
	r.data = newData       // Temporarily set r.data to newData for saving
	if err := r.save(); err != nil {
		r.data = originalData // Revert r.data if save fails
		return err
	}

	// Update cache (r.data) only if file save succeeded
	// r.data is already newData from the temporary assignment, so no further action needed here.
	return nil
}

// Update 更新条目
func (r *BaseJSONRepository[T]) Update(ctx context.Context, id string, item T) error {
	if err := r.initIfNeeded(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	idx := -1
	for i, exists := range r.data {
		if exists.GetID() == id {
			idx = i
			break
		}
	}

	if idx == -1 {
		return fmt.Errorf("item not found with id: %s", id)
	}

	// 1. Prepare new data (Copy on Write)
	newData := make([]T, len(r.data))
	copy(newData, r.data)
	newData[idx] = item

	// 2. Save to disk using new data
	originalData := r.data
	r.data = newData
	if err := r.save(); err != nil {
		r.data = originalData // Revert
		return err
	}

	// 3. Cache already updated (r.data = newData)
	return nil
}

// Delete 删除条目
func (r *BaseJSONRepository[T]) Delete(ctx context.Context, id string) error {
	if err := r.initIfNeeded(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	idx := -1
	for i, item := range r.data {
		if item.GetID() == id {
			idx = i
			break
		}
	}

	if idx == -1 {
		return fmt.Errorf("item not found with id: %s", id)
	}

	// 1. Prepare new data
	newData := make([]T, 0, len(r.data)-1)
	newData = append(newData, r.data[:idx]...)
	newData = append(newData, r.data[idx+1:]...)

	// 2. Save to disk
	originalData := r.data
	r.data = newData
	if err := r.save(); err != nil {
		r.data = originalData // Revert
		return err
	}

	// 3. Cache updated
	return nil
}

// Invalidate 清除缓存标志，下次访问时将从磁盘重新加载
func (r *BaseJSONRepository[T]) Invalidate() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.loaded = false
}

// SaveAll 批量保存 (全量覆盖)
func (r *BaseJSONRepository[T]) SaveAll(ctx context.Context, items []T) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	originalData := r.data
	r.data = items
	r.loaded = true

	if err := r.save(); err != nil {
		r.data = originalData // Revert
		return err
	}
	return nil
}
