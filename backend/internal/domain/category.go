package domain

import (
	"context"
	"errors"
)

// Category 分类实体 (Pure Entity)
// Added json tags for frontend compatibility.
type Category struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
}

// Validate 校验分类数据
func (c *Category) Validate() error {
	if c.Name == "" {
		return errors.New("category name cannot be empty")
	}
	if c.Slug == "" {
		return errors.New("category slug cannot be empty")
	}
	return nil
}

type CategoryRepository interface {
	// Create 创建分类
	Create(ctx context.Context, category *Category) error

	// Update 更新分类
	Update(ctx context.Context, slug string, category *Category) error

	// Delete 删除分类
	Delete(ctx context.Context, slug string) error

	// GetBySlug 根据 Slug 获取分类
	GetBySlug(ctx context.Context, slug string) (*Category, error)

	// List 获取分类列表
	List(ctx context.Context) ([]Category, error)
	SaveAll(ctx context.Context, categories []Category) error
}
