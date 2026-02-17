package comment

import (
	"context"
	"fmt"
	"gridea-pro/backend/internal/domain"
	"log/slog"
	"time"
)

type DisqusProvider struct {
	*BaseProvider
	config *DisqusConfig
}

func NewDisqusProvider(config *DisqusConfig, logger *slog.Logger) *DisqusProvider {
	return &DisqusProvider{
		BaseProvider: NewBaseProvider(10*time.Second, logger),
		config:       config,
	}
}

func (p *DisqusProvider) GetComments(ctx context.Context, articleID string) ([]domain.Comment, error) {
	// API: https://disqus.com/api/docs/posts/list/
	// 需要 public key, secret key, access_token 等，Disqus API 比较复杂，且通常前端直接加载 widget。
	// 后端获取评论通常用于 SEO 渲染或数据备份。
	// 这里暂不实现完整逻辑，只做结构适配。
	return nil, fmt.Errorf("%w: Disqus GetComments not implemented", ErrNotImplemented)
}

func (p *DisqusProvider) GetRecentComments(ctx context.Context, limit int) ([]domain.Comment, error) {
	return nil, fmt.Errorf("%w: Disqus GetRecentComments not implemented", ErrNotImplemented)
}

// GetAdminComments implementation
func (p *DisqusProvider) GetAdminComments(ctx context.Context, page, pageSize int) ([]domain.Comment, int64, error) {
	return nil, 0, fmt.Errorf("%w: Disqus GetAdminComments not implemented", ErrNotImplemented)
}

func (p *DisqusProvider) PostComment(ctx context.Context, comment *domain.Comment) error {
	return fmt.Errorf("%w: Disqus PostComment not implemented", ErrNotImplemented)
}

func (p *DisqusProvider) DeleteComment(ctx context.Context, commentID string) error {
	return fmt.Errorf("%w: Disqus DeleteComment not implemented", ErrNotImplemented)
}
