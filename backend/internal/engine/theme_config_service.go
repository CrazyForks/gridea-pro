package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gridea-pro/backend/internal/domain"
)

// ThemeConfigService 主题配置服务
type ThemeConfigService struct {
	appDir string
	cache  map[string]*domain.ThemeConfigSchema
	mu     sync.RWMutex
}

// NewThemeConfigService 创建主题配置服务
func NewThemeConfigService(appDir string) *ThemeConfigService {
	return &ThemeConfigService{
		appDir: appDir,
		cache:  make(map[string]*domain.ThemeConfigSchema),
	}
}

// LoadThemeConfig 加载主题配置定义
func (s *ThemeConfigService) LoadThemeConfig(themeName string) (*domain.ThemeConfigSchema, error) {
	// 1. Check Cache (Read Lock)
	s.mu.RLock()
	if config, ok := s.cache[themeName]; ok {
		s.mu.RUnlock()
		return config, nil
	}
	s.mu.RUnlock()

	// 2. Load from Disk (Write Lock)
	s.mu.Lock()
	defer s.mu.Unlock()

	// Double check cache after acquiring write lock
	if config, ok := s.cache[themeName]; ok {
		return config, nil
	}

	configPath := filepath.Join(s.appDir, "themes", themeName, "config.json")

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("读取主题配置失败: %w", err)
	}

	var config domain.ThemeConfigSchema
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析主题配置失败: %w", err)
	}

	// Update Cache
	s.cache[themeName] = &config

	return &config, nil
}

// GetDefaultConfig 获取主题默认配置（name -> value map）
func (s *ThemeConfigService) GetDefaultConfig(themeName string) (map[string]interface{}, error) {
	// Reuses LoadThemeConfig which is cached
	themeConfig, err := s.LoadThemeConfig(themeName)
	if err != nil {
		return nil, err
	}

	result := make(map[string]interface{})
	for _, item := range themeConfig.CustomConfig {
		result[item.Name] = item.Value
	}

	return result, nil
}

// MergeConfig 合并默认配置和用户配置
func (s *ThemeConfigService) MergeConfig(
	defaultConfig map[string]interface{},
	userConfig map[string]interface{},
) map[string]interface{} {
	result := make(map[string]interface{})

	// 先复制默认配置
	for k, v := range defaultConfig {
		result[k] = v
	}

	// 用户配置覆盖默认值
	for k, v := range userConfig {
		result[k] = v
	}

	return result
}

// GetFinalConfig 获取最终配置：主题默认配置 + 用户自定义配置合并
// 用户配置存储于 {appDir}/config/config.json 的 customConfig 字段
func (s *ThemeConfigService) GetFinalConfig(themeName string) (map[string]interface{}, error) {
	// Phase 1：读取主题默认配置
	defaultConfig, err := s.GetDefaultConfig(themeName)
	if err != nil {
		return nil, err
	}

	// Phase 2：读取用户已保存的配置并合并（用户配置优先）
	userConfig := s.loadUserConfig()
	if len(userConfig) > 0 {
		return s.MergeConfig(defaultConfig, userConfig), nil
	}

	return defaultConfig, nil
}

// loadUserConfig 读取用户已保存的主题自定义配置
// 路径：{appDir}/config/config.json 中的 customConfig 字段
func (s *ThemeConfigService) loadUserConfig() map[string]interface{} {
	configPath := filepath.Join(s.appDir, "config", "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}

	if cc, ok := raw["customConfig"].(map[string]interface{}); ok {
		return cc
	}
	return nil
}
