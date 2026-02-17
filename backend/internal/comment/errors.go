package comment

import "errors"

var (
	// ErrAuthFailed 认证失败
	ErrAuthFailed = errors.New("authentication failed")
	// ErrNotFound 资源不存在
	ErrNotFound = errors.New("resource not found")
	// ErrProviderError 第三方服务错误 (通用)
	ErrProviderError = errors.New("provider error")
	// ErrNotImplemented 功能未实现
	ErrNotImplemented = errors.New("feature not implemented")
	// ErrInvalidConfig 配置无效
	ErrInvalidConfig = errors.New("invalid configuration")
)
