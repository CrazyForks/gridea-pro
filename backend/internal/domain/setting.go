package domain

// Added Validate() method.

import (
	"context"
	"errors"
)

// Setting 系统设置
type Setting struct {
	Platform      string `json:"platform"`
	Domain        string `json:"domain"`
	Repository    string `json:"repository"`
	Branch        string `json:"branch"`
	Username      string `json:"username"`
	Email         string `json:"email"`
	TokenUsername string `json:"tokenUsername"`
	Token         string `json:"token"`
	CNAME         string `json:"cname"`
	// Poxy? (Typo in original?) Proxy? Keeping as is for now if matched with FE
	Port               string `json:"port"`
	Server             string `json:"server"`
	Password           string `json:"password"`
	PrivateKey         string `json:"privateKey"`
	RemotePath         string `json:"remotePath"`
	ProxyPath          string `json:"proxyPath"`
	ProxyPort          string `json:"proxyPort"`
	EnabledProxy       string `json:"enabledProxy"`
	NetlifySiteId      string `json:"netlifySiteId"`
	NetlifyAccessToken string `json:"netlifyAccessToken"`
}

// Validate 校验配置数据
func (s *Setting) Validate() error {
	// 基础校验，例如 Platform 不能为空
	if s.Platform == "" {
		return errors.New("platform is required")
	}
	// 可以根据 Platform 添加特定校验
	return nil
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
