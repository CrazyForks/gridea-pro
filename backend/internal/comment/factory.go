package comment

import (
	"encoding/json"
	"fmt"
	"gridea-pro/backend/internal/domain"
	"log/slog"
)

// Config structs for each provider
type ValineConfig struct {
	AppID      string `json:"appId"`
	AppKey     string `json:"appKey"`
	MasterKey  string `json:"masterKey"`
	ServerURLs string `json:"serverURLs"`
}

type WalineConfig struct {
	ServerURLs string `json:"serverURLs"`
	AppID      string `json:"appId"`     // Optional but sometimes used in templates
	AppKey     string `json:"appKey"`    // Optional
	MasterKey  string `json:"masterKey"` // Admin Token
}

type TwikooConfig struct {
	EnvID string `json:"envId"`
}

type GitHubConfig struct {
	Owner        string `json:"owner"`
	Repo         string `json:"repo"`
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
}

type DisqusConfig struct {
	Shortname string `json:"shortname"`
	APIKey    string `json:"apiKey"`
}

// NewProvider 创建评论提供者
func NewProvider(settings domain.CommentSettings) (domain.CommentProvider, error) {
	if !settings.Enable {
		return nil, ErrInvalidConfig // Or specific "disabled" error if needed, but usually we just don't init
	}

	// Helper helper to parse config
	parseConfig := func(src map[string]any, dst any) error {
		bytes, err := json.Marshal(src)
		if err != nil {
			return fmt.Errorf("failed to marshal config: %w", err)
		}
		return json.Unmarshal(bytes, dst)
	}

	rawConfig := settings.PlatformConfigs[settings.Platform]
	if rawConfig == nil {
		rawConfig = make(map[string]any)
	}

	// Create a base logger with context
	logger := slog.Default().With("platform", settings.Platform)

	switch settings.Platform {
	case domain.CommentPlatformValine:
		var cfg ValineConfig
		if err := parseConfig(rawConfig, &cfg); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidConfig, err)
		}
		if cfg.AppID == "" || cfg.AppKey == "" {
			return nil, fmt.Errorf("%w: Valine AppID and AppKey are required", ErrInvalidConfig)
		}
		return NewValineProvider(&cfg, logger), nil

	case domain.CommentPlatformWaline:
		var cfg WalineConfig
		if err := parseConfig(rawConfig, &cfg); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidConfig, err)
		}
		if cfg.ServerURLs == "" {
			return nil, fmt.Errorf("%w: Waline ServerURLs is required", ErrInvalidConfig)
		}
		return NewWalineProvider(&cfg, logger), nil

	case domain.CommentPlatformTwikoo:
		var cfg TwikooConfig
		if err := parseConfig(rawConfig, &cfg); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidConfig, err)
		}
		if cfg.EnvID == "" {
			return nil, fmt.Errorf("%w: Twikoo EnvID is required", ErrInvalidConfig)
		}
		return NewTwikooProvider(&cfg, logger), nil

	case domain.CommentPlatformGitalk:
		var cfg GitHubConfig
		if err := parseConfig(rawConfig, &cfg); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidConfig, err)
		}
		if cfg.Owner == "" || cfg.Repo == "" {
			return nil, fmt.Errorf("%w: GitHub Owner and Repo are required", ErrInvalidConfig)
		}
		// Gitalk uses GitHub Provider backed logic
		return NewGitHubProvider(&cfg, logger), nil

	case domain.CommentPlatformGiscus:
		return nil, fmt.Errorf("%w: Giscus provider not implemented yet (use GitHub Provider internally?)", ErrNotImplemented)

	case domain.CommentPlatformDisqus:
		// Disqus config might be unstructured in original map, let's try strict now
		// If original code used raw map, we need to see what fields it used.
		// Checking disqus_provider.go... it was empty `config map[string]any`.
		// Let's assume generic map for now or define a struct if we know.
		// Based on typical usage, it needs Shortname and maybe API Key for backend fetching.
		var cfg DisqusConfig
		if err := parseConfig(rawConfig, &cfg); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidConfig, err)
		}
		// Minimal requirement check
		if cfg.APIKey == "" {
			// If we dictate API Key is required for backend fetching (which it is for Disqus API)
			// context: Disqus public widget uses shortname, but backend API needs public key.
		}
		return NewDisqusProvider(&cfg, logger), nil

	default:
		return nil, fmt.Errorf("%w: %s", ErrNotImplemented, settings.Platform)
	}
}
