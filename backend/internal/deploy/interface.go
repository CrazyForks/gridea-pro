package deploy

import (
	"context"
	"gridea-pro/backend/internal/domain"
)

// LogFunc 定义了日志输出回调的签名，彻底阻断其直接依赖 wails 的 runtime.EventsEmit
type LogFunc func(msg string)

// Provider 定义了统一的部署标准接口
type Provider interface {
	Deploy(ctx context.Context, outputDir string, setting *domain.Setting, logger LogFunc) error
}
