package domain

import "context"

// AISetting AI 功能配置
type AISetting struct {
	ZhipuAPIKey string `json:"zhipuApiKey"` // 用户自己的 Key，空则用内置
	Model       string `json:"model"`       // 默认 glm-4-flash
}

// AISettingRepository AI 配置存储接口
type AISettingRepository interface {
	GetAISetting(ctx context.Context) (AISetting, error)
	SaveAISetting(ctx context.Context, setting AISetting) error
}
