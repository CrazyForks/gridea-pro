package facade

import (
	"context"
	"gridea-pro/backend/internal/domain"
)

type CdnSettingFacade struct {
	repo domain.CdnSettingRepository
}

func NewCdnSettingFacade(repo domain.CdnSettingRepository) *CdnSettingFacade {
	return &CdnSettingFacade{repo: repo}
}

func (f *CdnSettingFacade) GetCdnSetting() (domain.CdnSetting, error) {
	ctx := WailsContext
	if ctx == nil {
		ctx = context.TODO()
	}
	return f.repo.GetCdnSetting(ctx)
}

func (f *CdnSettingFacade) SaveCdnSettingFromFrontend(setting domain.CdnSetting) error {
	ctx := WailsContext
	if ctx == nil {
		ctx = context.TODO()
	}
	return f.repo.SaveCdnSetting(ctx, setting)
}
