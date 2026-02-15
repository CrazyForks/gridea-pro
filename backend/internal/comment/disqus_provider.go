package comment

import (
	"context"
	"fmt"
	"gridea-pro/backend/internal/domain"
)

type DisqusProvider struct {
	config map[string]any
}

func NewDisqusProvider(config map[string]any) *DisqusProvider {
	return &DisqusProvider{config: config}
}

func (p *DisqusProvider) GetComments(ctx context.Context, articleID string) ([]domain.Comment, error) {
	return []domain.Comment{}, nil
}

func (p *DisqusProvider) GetRecentComments(ctx context.Context, limit int) ([]domain.Comment, error) {
	return []domain.Comment{}, nil
}

// GetAdminComments implementation
func (p *DisqusProvider) GetAdminComments(ctx context.Context, page, pageSize int) ([]domain.Comment, int64, error) {
	return nil, 0, fmt.Errorf("DisqusProvider GetAdminComments not implemented")
}

func (p *DisqusProvider) PostComment(ctx context.Context, comment *domain.Comment) error {
	return fmt.Errorf("Disqus PostComment not implemented")
}

func (p *DisqusProvider) DeleteComment(ctx context.Context, commentID string) error {
	return nil
}
