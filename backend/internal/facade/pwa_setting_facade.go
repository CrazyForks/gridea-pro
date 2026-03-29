package facade

import (
	"context"
	"fmt"
	"gridea-pro/backend/internal/domain"
	"os"
	"path/filepath"
)

type PwaSettingFacade struct {
	repo   domain.PwaSettingRepository
	appDir string
}

func NewPwaSettingFacade(repo domain.PwaSettingRepository, appDir string) *PwaSettingFacade {
	return &PwaSettingFacade{repo: repo, appDir: appDir}
}

func (f *PwaSettingFacade) GetPwaSetting() (domain.PwaSetting, error) {
	ctx := WailsContext
	if ctx == nil {
		ctx = context.TODO()
	}
	return f.repo.GetPwaSetting(ctx)
}

func (f *PwaSettingFacade) SavePwaSettingFromFrontend(setting domain.PwaSetting) error {
	ctx := WailsContext
	if ctx == nil {
		ctx = context.TODO()
	}
	return f.repo.SavePwaSetting(ctx, setting)
}

// HasCustomPwaIcon 检查是否存在自定义 PWA 图标
func (f *PwaSettingFacade) HasCustomPwaIcon() bool {
	iconPath := filepath.Join(f.appDir, "images", "pwa-icon.png")
	_, err := os.Stat(iconPath)
	return err == nil
}

// SavePwaIcon 保存自定义 PWA 图标（512x512）
func (f *PwaSettingFacade) SavePwaIcon(sourcePath string) error {
	if sourcePath == "" {
		return fmt.Errorf("图片路径不能为空")
	}

	destDir := filepath.Join(f.appDir, "images")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	destPath := filepath.Join(destDir, "pwa-icon.png")

	sourceData, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("读取源文件失败: %w", err)
	}

	if err := os.WriteFile(destPath, sourceData, 0644); err != nil {
		return fmt.Errorf("保存 PWA 图标失败: %w", err)
	}

	return nil
}

// RemovePwaIcon 删除自定义 PWA 图标，恢复使用头像
func (f *PwaSettingFacade) RemovePwaIcon() error {
	iconPath := filepath.Join(f.appDir, "images", "pwa-icon.png")
	if err := os.Remove(iconPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除 PWA 图标失败: %w", err)
	}
	return nil
}
