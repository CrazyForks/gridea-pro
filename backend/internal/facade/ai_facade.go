package facade

import (
	"context"
	"gridea-pro/backend/internal/domain"
	"gridea-pro/backend/internal/service"
)

// AIFacade 暴露给前端的 AI 功能接口
type AIFacade struct {
	repo    domain.AISettingRepository
	service *service.AIService
}

func NewAIFacade(repo domain.AISettingRepository, svc *service.AIService) *AIFacade {
	return &AIFacade{repo: repo, service: svc}
}

// GetAISetting 获取 AI 配置
func (f *AIFacade) GetAISetting() (domain.AISetting, error) {
	ctx := WailsContext
	if ctx == nil {
		ctx = context.TODO()
	}
	return f.repo.GetAISetting(ctx)
}

// SaveAISettingFromFrontend 保存 AI 配置
func (f *AIFacade) SaveAISettingFromFrontend(setting domain.AISetting) error {
	ctx := WailsContext
	if ctx == nil {
		ctx = context.TODO()
	}
	return f.repo.SaveAISetting(ctx, setting)
}

// GenerateSlug 根据文章标题 AI 生成 SEO 友好的英文 Slug
func (f *AIFacade) GenerateSlug(title string) (string, error) {
	ctx := WailsContext
	if ctx == nil {
		ctx = context.TODO()
	}
	return f.service.GenerateSlug(ctx, title)
}
