package facade

import (
	"context"
	"gridea-pro/backend/internal/domain"
)

type SeoSettingFacade struct {
	repo domain.SeoSettingRepository
}

func NewSeoSettingFacade(repo domain.SeoSettingRepository) *SeoSettingFacade {
	return &SeoSettingFacade{repo: repo}
}

func (f *SeoSettingFacade) GetSeoSetting() (domain.SeoSetting, error) {
	ctx := WailsContext
	if ctx == nil {
		ctx = context.TODO()
	}
	return f.repo.GetSeoSetting(ctx)
}

func (f *SeoSettingFacade) SaveSeoSettingFromFrontend(setting domain.SeoSetting) error {
	ctx := WailsContext
	if ctx == nil {
		ctx = context.TODO()
	}
	return f.repo.SaveSeoSetting(ctx, setting)
}
