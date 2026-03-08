package facade

import (
	"context"
	"gridea-pro/backend/internal/domain"
	"gridea-pro/backend/internal/service"
	"log/slog"
)

// ThemeFacade wraps ThemeService
type ThemeFacade struct {
	internal *service.ThemeService
	renderer *RendererFacade
	logger   *slog.Logger
}

func NewThemeFacade(s *service.ThemeService) *ThemeFacade {
	return &ThemeFacade{internal: s, logger: slog.Default()}
}

func (f *ThemeFacade) SetRenderer(renderer *RendererFacade) {
	f.renderer = renderer
}

func (f *ThemeFacade) LoadThemes() ([]domain.Theme, error) {
	ctx := WailsContext
	if ctx == nil {
		ctx = context.TODO()
	}
	return f.internal.LoadThemes(ctx)
}

func (f *ThemeFacade) LoadThemeConfig() (domain.ThemeConfig, error) {
	ctx := WailsContext
	if ctx == nil {
		ctx = context.TODO()
	}
	return f.internal.LoadThemeConfig(ctx)
}

func (f *ThemeFacade) SaveThemeConfig(config domain.ThemeConfig) error {
	ctx := WailsContext
	if ctx == nil {
		ctx = context.TODO()
	}
	return f.internal.SaveThemeConfig(ctx, config)
}

func (f *ThemeFacade) UploadThemeCustomConfigImage(sourcePath string) (string, error) {
	ctx := WailsContext
	if ctx == nil {
		ctx = context.TODO()
	}
	return f.internal.SaveThemeImage(ctx, sourcePath)
}

// SaveThemeConfigFromFrontend saves theme config and triggers render
func (f *ThemeFacade) SaveThemeConfigFromFrontend(config domain.ThemeConfig) error {
	ctx := WailsContext
	if ctx == nil {
		ctx = context.TODO()
	}
	if err := f.internal.SaveThemeConfig(ctx, config); err != nil {
		return err
	}

	// Trigger render
	if f.renderer != nil {
		go func() {
			if err := f.renderer.RenderAll(); err != nil {
				f.logger.Error("Error rendering after theme save", "error", err)
			}
		}()
	}

	return nil
}

// SaveThemeCustomConfigFromFrontend saves custom config and triggers render
func (f *ThemeFacade) SaveThemeCustomConfigFromFrontend(customConfig map[string]interface{}) error {
	// 1. Load current config
	currentConfig, err := f.LoadThemeConfig()
	if err != nil {
		return err
	}

	// 2. Update CustomConfig
	currentConfig.CustomConfig = customConfig

	// 3. Save config
	ctx := WailsContext
	if ctx == nil {
		ctx = context.TODO()
	}
	if err := f.internal.SaveThemeConfig(ctx, currentConfig); err != nil {
		return err
	}

	// 4. Trigger render
	if f.renderer != nil {
		go func() {
			if err := f.renderer.RenderAll(); err != nil {
				f.logger.Error("Error rendering after theme custom config save", "error", err)
			}
		}()
	}

	return nil
}
