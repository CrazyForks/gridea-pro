package domain

import (
	"context"
	"encoding/json"
	"errors"
)

// Setting 系统设置
// platform 标识当前启用的平台，platformConfigs 按平台独立存储所有配置
type Setting struct {
	Platform        string                       `json:"platform"`
	PlatformConfigs map[string]json.RawMessage   `json:"platformConfigs,omitempty"`
}

// getConfig 解析当前平台的配置为 map
func (s *Setting) getConfig() map[string]any {
	if s.PlatformConfigs == nil {
		return nil
	}
	raw := s.PlatformConfigs[s.Platform]
	if raw == nil {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil
	}
	return m
}

// Get 获取当前平台的指定配置项
func (s *Setting) Get(key string) string {
	m := s.getConfig()
	if m == nil {
		return ""
	}
	v, _ := m[key].(string)
	return v
}

// GetFrom 获取指定平台的指定配置项
func (s *Setting) GetFrom(platform, key string) string {
	if s.PlatformConfigs == nil {
		return ""
	}
	raw := s.PlatformConfigs[platform]
	if raw == nil {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}
	v, _ := m[key].(string)
	return v
}

// Domain 当前平台的域名
func (s *Setting) Domain() string { return s.Get("domain") }

// Repository 当前平台的仓库名/项目名
func (s *Setting) Repository() string { return s.Get("repository") }

// Branch 当前平台的分支
func (s *Setting) Branch() string { return s.Get("branch") }

// Username 当前平台的用户名
func (s *Setting) Username() string { return s.Get("username") }

// Email 当前平台的邮箱
func (s *Setting) Email() string { return s.Get("email") }

// TokenUsername 当前平台的 Token 用户名
func (s *Setting) TokenUsername() string { return s.Get("tokenUsername") }

// Token 当前平台的 Token
func (s *Setting) Token() string { return s.Get("token") }

// CNAME 当前平台的 CNAME
func (s *Setting) CNAME() string { return s.Get("cname") }

// Password 当前平台的密码
func (s *Setting) Password() string { return s.Get("password") }

// PrivateKey 当前平台的私钥路径
func (s *Setting) PrivateKey() string { return s.Get("privateKey") }

// NetlifyAccessToken 当前平台的 Netlify Access Token
func (s *Setting) NetlifyAccessToken() string { return s.Get("netlifyAccessToken") }

// Validate 校验配置数据
func (s *Setting) Validate() error {
	if s.Platform == "" {
		return errors.New("platform is required")
	}
	return nil
}

// SetPlatformConfig 设置指定平台的某个配置项
func (s *Setting) SetPlatformConfig(platform, key string, value any) {
	if s.PlatformConfigs == nil {
		s.PlatformConfigs = make(map[string]json.RawMessage)
	}
	var m map[string]any
	if raw := s.PlatformConfigs[platform]; raw != nil {
		_ = json.Unmarshal(raw, &m)
	}
	if m == nil {
		m = make(map[string]any)
	}
	m[key] = value
	data, _ := json.Marshal(m)
	s.PlatformConfigs[platform] = data
}

// SettingRepository 定义配置存储接口
type SettingRepository interface {
	GetSetting(ctx context.Context) (Setting, error)
	SaveSetting(ctx context.Context, setting Setting) error
}

type DeployResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}
