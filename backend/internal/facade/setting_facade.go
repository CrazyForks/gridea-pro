package facade

import (
	"context"
	"log/slog"

	"gridea-pro/backend/internal/deploy"
	"gridea-pro/backend/internal/domain"
	"gridea-pro/backend/internal/service"
)

// SettingFacade wraps SettingService
type SettingFacade struct {
	internal *service.SettingService
	logger   *slog.Logger
}

func NewSettingFacade(s *service.SettingService) *SettingFacade {
	return &SettingFacade{
		internal: s,
		logger:   slog.Default(),
	}
}

func (f *SettingFacade) GetSetting() (domain.Setting, error) {
	ctx := WailsContext
	if ctx == nil {
		ctx = context.TODO()
	}
	return f.internal.GetSetting(ctx)
}

func (f *SettingFacade) SaveAvatar(sourcePath string) error {
	ctx := WailsContext
	if ctx == nil {
		ctx = context.TODO()
	}
	return f.internal.SaveAvatar(ctx, sourcePath)
}

func (f *SettingFacade) SaveFavicon(sourcePath string) error {
	ctx := WailsContext
	if ctx == nil {
		ctx = context.TODO()
	}
	return f.internal.SaveFavicon(ctx, sourcePath)
}

func (f *SettingFacade) SaveSettingFromFrontend(setting domain.Setting) error {
	ctx := WailsContext
	if ctx == nil {
		ctx = context.TODO()
	}

	// 保存前获取旧配置，用于检测域名变更
	oldSetting, _ := f.internal.GetSetting(ctx)

	// 保存新配置
	if err := f.internal.SaveSetting(ctx, setting); err != nil {
		return err
	}

	// Vercel 自定义域名自动绑定
	if setting.Platform == "vercel" {
		newCname := setting.CNAME()
		oldCname := oldSetting.GetFrom("vercel", "cname")
		projectName := setting.Repository()
		token := setting.Token()

		if projectName != "" && token != "" {
			proxyURL := ""
			if setting.ProxyEnabled {
				proxyURL = setting.ProxyURL
			}
			vercel := deploy.NewVercelProvider(proxyURL)

			// 域名变更时，先删除旧域名
			if oldCname != "" && oldCname != newCname {
				if err := vercel.RemoveCustomDomain(ctx, projectName, oldCname, token); err != nil {
					f.logger.Error("Vercel 旧域名解绑失败", "domain", oldCname, "error", err)
				} else {
					f.logger.Info("Vercel 旧域名已解绑", "domain", oldCname)
				}
			}

			// 绑定新域名
			if newCname != "" {
				if err := vercel.AddCustomDomain(ctx, projectName, newCname, token); err != nil {
					f.logger.Error("Vercel 域名绑定失败", "domain", newCname, "error", err)
				} else {
					f.logger.Info("Vercel 域名绑定成功", "domain", newCname)
				}
			}
		}
	}

	return nil
}

func (f *SettingFacade) RemoteDetectFromFrontend(setting domain.Setting) (map[string]interface{}, error) {
	ctx := WailsContext
	if ctx == nil {
		ctx = context.TODO()
	}
	return f.internal.RemoteDetect(ctx, setting)
}
